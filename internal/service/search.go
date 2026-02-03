package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"dict_new/config"
	"dict_new/internal/database"
	"dict_new/internal/util"
)

type SearchResult struct {
	Name        string `json:"name"`
	Nation      string `json:"nation"`
	Translation string `json:"translation"`
}

type TabResult struct {
	Tab     string         `json:"tab"`
	Results []SearchResult `json:"results"`
}

type SearchResponse struct {
	Names               []SearchResult            `json:"names,omitempty"`
	Places              []SearchResult            `json:"places,omitempty"`
	Tabs                []string                  `json:"tabs,omitempty"`
	TabResults          map[string][]SearchResult `json:"tab_results,omitempty"`
	FullQueryNoRecord   string                    `json:"full_query_no_record,omitempty"`
	Cache               bool                      `json:"cache"`
}

type SearchService struct {
	db     *database.Connection
	cache  *CacheService
	config *config.Config
}

func NewSearchService(db *database.Connection, cache *CacheService, cfg *config.Config) (*SearchService, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if cache == nil {
		return nil, fmt.Errorf("cache service is required")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	return &SearchService{
		db:     db,
		cache:  cache,
		config: cfg,
	}, nil
}

// getTableName returns the actual table name from config based on search type
func (s *SearchService) getTableName(searchType string) string {
	switch searchType {
	case "names":
		return s.config.Tables.Names
	case "places":
		return s.config.Tables.Places
	default:
		return searchType
	}
}

// 分割查询字符串
func (s *SearchService) splitQuery(query string) []string {
	if query == "" {
		return []string{}
	}

	words := strings.Fields(query) // 按空格分割
	if len(words) == 1 {
		return []string{query}
	}

	// 完整查询词 + 单个单词
	result := []string{query}
	result = append(result, words...)
	return result
}

// 主搜索方法
func (s *SearchService) Search(query, searchType string) (*SearchResponse, error) {
	return s.SearchWithOptions(query, searchType, false)
}

// 带选项的搜索方法
func (s *SearchService) SearchWithOptions(query, searchType string, noCache bool) (*SearchResponse, error) {
	// 处理查询字符串 (use util functions directly instead of wrapper methods)
	language := util.DetectLanguage(query)
	processedQuery := util.ProcessQueryByLanguage(query, language)

	// 如果不禁用缓存，尝试从缓存获取
	if !noCache {
		cacheKey := s.cache.GenerateKey(processedQuery, searchType, language)
		if cached, err := s.cache.Get(cacheKey); err == nil {
			// both 模式：旧缓存可能无 tab 数据，不使用以免 Ford 等词被忽略
			if searchType == "both" && !hasBothTabData(cached) {
				// 当作未命中，走数据库
			} else {
				return &SearchResponse{
					Names:             s.convertToSearchResults(cached.Data["names"]),
					Places:            s.convertToSearchResults(cached.Data["places"]),
					Tabs:              cached.Tabs,
					TabResults:        cached.TabResults,
					FullQueryNoRecord: cached.FullQueryNoRecord,
					Cache:             true,
				}, nil
			}
		}
	}

	// 从数据库查询
	response, err := s.searchFromDatabase(processedQuery, searchType, language)
	if err != nil {
		return nil, err
	}

	// 如果不禁用缓存，保存到缓存
	if !noCache {
		// 生成缓存键
		cacheKey := s.cache.GenerateKey(processedQuery, searchType, language)

		// 保存到缓存
		cacheData := &CacheData{
			Query:              processedQuery,
			Type:               searchType,
			Language:           language,
			Data:               map[string]interface{}{
				"names":  response.Names,
				"places": response.Places,
			},
			Tabs:               response.Tabs,
			TabResults:         response.TabResults,
			FullQueryNoRecord:  response.FullQueryNoRecord,
		}

		if err := s.cache.Set(cacheKey, cacheData); err != nil {
			// 缓存失败不影响搜索结果，静默忽略
		}
	}

	response.Cache = false
	return response, nil
}

// hasBothTabData 判断缓存是否包含 both 模式所需的 tab 数据（旧缓存可能无）
func hasBothTabData(c *CacheData) bool {
	return c != nil && c.TabResults != nil
}

// 从数据库搜索
func (s *SearchService) searchFromDatabase(query, searchType, language string) (*SearchResponse, error) {
	response := &SearchResponse{}

	// 分割查询
	queryParts := s.splitQuery(query)
	response.Tabs = queryParts

	switch searchType {
	case "names":
		results, tabResults, err := s.searchNames(queryParts, language)
		if err != nil {
			return nil, err
		}
		response.Names = results
		response.TabResults = tabResults
		response.Tabs, response.TabResults = dropFirstTabIfEmpty(queryParts, tabResults, results)
		if len(response.Tabs) < len(queryParts) {
			response.FullQueryNoRecord = queryParts[0]
		}
	case "places":
		results, tabResults, err := s.searchPlaces(queryParts, language)
		if err != nil {
			return nil, err
		}
		response.Places = results
		response.TabResults = tabResults
		response.Tabs, response.TabResults = dropFirstTabIfEmpty(queryParts, tabResults, results)
		if len(response.Tabs) < len(queryParts) {
			response.FullQueryNoRecord = queryParts[0]
		}
	case "both":
		// 人名：分词搜索，生成分词tab
		names, namesTabResults, err := s.searchNames(queryParts, language)
		if err != nil {
			return nil, err
		}
		// 地名：只搜索完整查询词，不分词，不生成tab
		places, _, err := s.searchPlaces([]string{query}, language)
		if err != nil {
			return nil, err
		}
		response.Names = names
		response.Places = places
		// 只为人名生成tab
		response.Tabs, response.TabResults = buildNameTabsOnly(queryParts, namesTabResults, names)
		if len(queryParts) > 1 && !hasExactMatch(namesTabResults[queryParts[0]], queryParts[0]) {
			response.FullQueryNoRecord = queryParts[0]
		}
	default:
		return nil, fmt.Errorf("invalid search type: %s", searchType)
	}

	return response, nil
}

// searchTable is the consolidated search function that replaces searchNames and searchPlaces
// It searches a single table (names or places) with the given query parts and language
func (s *SearchService) searchTable(queryParts []string, language, tableName string) ([]SearchResult, map[string][]SearchResult, error) {
	var allResults []SearchResult
	tabResults := make(map[string][]SearchResult)

	queryField := s.config.Fields.QueryField
	nameField := s.config.Fields.NameField
	nationField := s.config.Fields.NationField
	translationField := s.config.Fields.TranslationField

	for _, query := range queryParts {
		// Process query using util functions directly
		processedQuery := util.ProcessQueryByLanguage(query, util.DetectLanguage(query))
		processedQuery = strings.ReplaceAll(processedQuery, "-", " ")
		// 处理 St/St. -> Saint，以匹配数据库中的格式
		processedQuery = normalizeSaint(processedQuery)

		// 构建查找独立词的SQL查询
		var sqlQuery string
		var args []interface{}

		if language == "zh" {
			// 中文查询translation字段
			sqlQuery = fmt.Sprintf(
				"SELECT DISTINCT %s, %s, %s FROM %s WHERE %s LIKE ? ORDER BY %s",
				nameField, nationField, translationField,
				tableName, translationField, nameField,
			)
			args = []interface{}{
				"%" + processedQuery + "%", // 包含查询词
			}
		} else {
			// 其他语言查询name_for_query字段
			// 注意：数据库中queryField前后都有空格，所以使用 LIKE '% word %'
			sqlQuery = fmt.Sprintf(
				"SELECT DISTINCT %s, %s, %s FROM %s WHERE %s LIKE ? ORDER BY %s",
				nameField, nationField, translationField,
				tableName, queryField, nameField,
			)
			args = []interface{}{
				"% " + processedQuery + " %", // 前后都有空格匹配
			}
		}

		rows, err := s.db.DB.Query(sqlQuery, args...)
		if err != nil {
			return nil, nil, fmt.Errorf("database query failed: %w", err)
		}

		var queryResults []SearchResult
		for rows.Next() {
			var name, nation, translation string
			if err := rows.Scan(&name, &nation, &translation); err != nil {
				rows.Close()
				return nil, nil, fmt.Errorf("failed to scan row: %w", err)
			}

			result := SearchResult{
				Name:        name,
				Nation:      nation,
				Translation: translation,
			}
			queryResults = append(queryResults, result)
		}
		rows.Close()

		// 完全匹配查询词的结果排最前，其次以查询词开头，再为包含查询词
		queryResults = sortResultsByQueryMatch(queryResults, query)

		// 保存该tab的结果
		tabResults[query] = queryResults

		// 如果是第一个查询（完整查询词），也添加到总结果中
		if query == queryParts[0] {
			allResults = queryResults
		}
	}

	// 完整短语无结果时：合并各词的结果并去重，避免多词查询（如 George Washington）无显示
	if len(allResults) == 0 && len(tabResults) > 0 {
		var slices [][]SearchResult
		for _, r := range tabResults {
			slices = append(slices, r)
		}
		allResults = mergeAndDeduplicateResults(slices...)
	}

	return allResults, tabResults, nil
}

// 搜索人名 - now delegates to searchTable
func (s *SearchService) searchNames(queryParts []string, language string) ([]SearchResult, map[string][]SearchResult, error) {
	return s.searchTable(queryParts, language, s.config.Tables.Names)
}

// 搜索地名 - now delegates to searchTable
func (s *SearchService) searchPlaces(queryParts []string, language string) ([]SearchResult, map[string][]SearchResult, error) {
	return s.searchTable(queryParts, language, s.config.Tables.Places)
}

// 转换缓存数据为搜索结果
func (s *SearchService) convertToSearchResults(data interface{}) []SearchResult {
	if data == nil {
		return []SearchResult{}
	}

	if results, ok := data.([]SearchResult); ok {
		return results
	}

	if results, ok := data.([]interface{}); ok {
		searchResults := make([]SearchResult, len(results))
		for i, r := range results {
			if item, ok := r.(map[string]interface{}); ok {
				searchResults[i] = SearchResult{
					Name:        getString(item, "name"),
					Nation:      getString(item, "nation"),
					Translation: getString(item, "translation"),
				}
			}
		}
		return searchResults
	}

	return []SearchResult{}
}

// 辅助函数：从map中安全获取字符串
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// normalizeSaint 将 St/St. 转换为 Saint，以匹配数据库中的 name_for_query 格式
func normalizeSaint(query string) string {
	// 替换 "St " 或 "St. " 开头的为 "Saint "
	// 使用正则表达式处理单词边界
	re := regexp.MustCompile(`(?i)\bSt\.?\s+`)
	return re.ReplaceAllString(query, "Saint ")
}

// matchRankForSort 返回 name 相对 query 的匹配等级：0=完全匹配，1=以查询词开头，2=包含为整词，3=其他（数值越小越靠前）
func matchRankForSort(name, query string) int {
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	if query == "" {
		return 3
	}
	nameLower := strings.ToLower(name)
	queryLower := strings.ToLower(query)
	if nameLower == queryLower {
		return 0
	}
	if strings.HasPrefix(nameLower, queryLower+" ") || strings.HasPrefix(nameLower, queryLower+",") {
		return 1
	}
	// 整词包含：前后为空格或逗号（如 "Back, George"）
	needle := " " + queryLower + " "
	if strings.Contains(nameLower, needle) {
		return 2
	}
	if strings.HasSuffix(nameLower, " "+queryLower) {
		return 2
	}
	if strings.Contains(nameLower, ","+queryLower) || strings.Contains(nameLower, queryLower+",") {
		return 2
	}
	return 3
}

// sortResultsByQueryMatch 按“查询词匹配程度”排序：完全匹配最前，其次以查询词开头，再为包含查询词，同等级按 name 排序
func sortResultsByQueryMatch(results []SearchResult, query string) []SearchResult {
	if len(results) <= 1 {
		return results
	}
	out := make([]SearchResult, len(results))
	copy(out, results)
	sort.Slice(out, func(i, j int) bool {
		ri := matchRankForSort(out[i].Name, query)
		rj := matchRankForSort(out[j].Name, query)
		if ri != rj {
			return ri < rj
		}
		return strings.Compare(out[i].Name, out[j].Name) < 0
	})
	return out
}

// hasExactMatch 检查结果中是否有与查询词完全匹配的项（忽略大小写和前后空格）
func hasExactMatch(results []SearchResult, query string) bool {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	for _, r := range results {
		if strings.ToLower(strings.TrimSpace(r.Name)) == queryLower {
			return true
		}
	}
	return false
}

// buildNameTabsOnly 为 both 模式只构建人名tab（地名只搜完整查询词，不生成tab）
func buildNameTabsOnly(queryParts []string, namesTab map[string][]SearchResult, names []SearchResult) (tabs []string, allTabResults map[string][]SearchResult) {
	allTabResults = make(map[string][]SearchResult)

	// 检查完整查询是否有人名完全匹配
	hasFullQueryExactMatch := false
	if len(queryParts) > 1 {
		hasFullQueryExactMatch = hasExactMatch(namesTab[queryParts[0]], queryParts[0])
	}

	// 从哪些 queryParts 开始生成 tab
	startIdx := 0
	if len(queryParts) > 1 && !hasFullQueryExactMatch {
		startIdx = 1 // 跳过完整查询词
	}

	// 只生成人名 tab
	for i := startIdx; i < len(queryParts); i++ {
		query := queryParts[i]
		if results, ok := namesTab[query]; ok && len(results) > 0 {
			tabs = append(tabs, query)
			allTabResults[query] = results
		}
	}

	return tabs, allTabResults
}

// deduplicateTabs 对后续 tabs 去重：排除已在第一个 tab 中的记录
func deduplicateTabs(queryParts []string, tabResults map[string][]SearchResult) {
	if len(queryParts) <= 1 {
		return
	}

	firstKey := queryParts[0]
	firstResults := tabResults[firstKey]

	// 建立第一个 tab 的 key 集合
	firstKeys := make(map[string]bool)
	for _, r := range firstResults {
		key := r.Name + "\x00" + r.Nation + "\x00" + r.Translation
		firstKeys[key] = true
	}

	// 从后续 tabs 中排除已在第一个 tab 中的记录
	for i := 1; i < len(queryParts); i++ {
		query := queryParts[i]
		if results, ok := tabResults[query]; ok {
			var filtered []SearchResult
			for _, r := range results {
				key := r.Name + "\x00" + r.Nation + "\x00" + r.Translation
				if !firstKeys[key] {
					filtered = append(filtered, r)
				}
			}
			tabResults[query] = filtered
		}
	}
}

// dropFirstTabIfEmpty 当完整查询无结果时，第一个 tab 显示包含任意分词的结果
func dropFirstTabIfEmpty(queryParts []string, tabResults map[string][]SearchResult, results []SearchResult) (tabs []string, out map[string][]SearchResult) {
	if len(queryParts) <= 1 || len(results) == 0 {
		return queryParts, tabResults
	}

	firstKey := queryParts[0]
	firstResults, ok := tabResults[firstKey]

	// 如果第一个 tab 有结果，保留所有 tabs
	if ok && len(firstResults) > 0 {
		// 对后续 tabs 去重
		deduplicateTabs(queryParts, tabResults)
		return queryParts, tabResults
	}

	// 第一个 tab 无结果，合并分词结果到第一个 tab
	var mergedResults []SearchResult
	seen := make(map[string]bool)

	for i := 1; i < len(queryParts); i++ {
		word := queryParts[i]
		if wordResults, exists := tabResults[word]; exists {
			for _, r := range wordResults {
				key := r.Name + "\x00" + r.Nation + "\x00" + r.Translation
				if !seen[key] {
					seen[key] = true
					mergedResults = append(mergedResults, r)
				}
			}
		}
	}

	// 更新第一个 tab 的结果
	tabResults[firstKey] = mergedResults

	// 对后续 tabs 去重
	if len(mergedResults) > 0 {
		deduplicateTabs(queryParts, tabResults)
		return queryParts, tabResults
	}

	// 合并后仍无结果，去掉第一个 tab
	tabs = queryParts[1:]
	out = make(map[string][]SearchResult)
	for k, v := range tabResults {
		if k != firstKey {
			out[k] = v
		}
	}
	return tabs, out
}

// mergeAndDeduplicateResults 合并多组搜索结果并按 (Name, Nation, Translation) 去重，保留首次出现顺序
func mergeAndDeduplicateResults(slices ...[]SearchResult) []SearchResult {
	seen := make(map[string]bool)
	var out []SearchResult
	for _, slice := range slices {
		for _, r := range slice {
			key := r.Name + "\x00" + r.Nation + "\x00" + r.Translation
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, r)
		}
	}
	return out
}