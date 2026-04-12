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

// validateTableName 验证表名是否合法（防止SQL注入）
func validateTableName(name string) bool {
	if name == "" {
		return false
	}
	// 只允许字母、数字、下划线
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// validateFieldName 验证字段名是否合法（防止SQL注入）
func validateFieldName(name string) bool {
	if name == "" {
		return false
	}
	// 只允许字母、数字、下划线
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// 停用词列表
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true,
	"and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true,
	"from": true, "as": true,
}

// saintRegex 用于匹配 St/St. 开头的单词（预编译以提高性能）
var saintRegex = regexp.MustCompile(`(?i)\bSt\.?\s+`)

// normalizeApostrophes 将排版用弯引号替换为直引号，避免查询 L'Aigle 时因引号类型不同而匹配不到数据
func normalizeApostrophes(query string) string {
	query = strings.ReplaceAll(query, "\u2019", "'") // 右单引号
	query = strings.ReplaceAll(query, "\u2018", "'") // 左单引号
	return query
}

// 分割查询字符串
func (s *SearchService) splitQuery(query string) []string {
	if query == "" {
		return []string{}
	}

	// 将连字符替换为空格，使 "Château-Gontier-sur-Mayenne" 能像按空格拆分一样被拆分
	normalized := strings.ReplaceAll(query, "-", " ")
	words := strings.Fields(normalized) // 按空格分割
	if len(words) == 1 {
		return []string{query}
	}

	// 完整查询词 + 非停用词
	result := []string{query}

	// 对于连字符查询，生成前缀子串组合（如 Château-Gontier-sur-Mayenne 生成 Château-Gontier-sur、Château-Gontier 等）
	if strings.Contains(query, "-") {
		for i := len(words) - 1; i >= 1; i-- {
			sub := strings.Join(words[:i], "-")
			if sub != query {
				result = append(result, sub)
			}
		}
	}

	for _, word := range words {
		lowerWord := strings.ToLower(word)
		if !stopWords[lowerWord] {
			// 避免重复添加
			exists := false
			for _, r := range result {
				if r == word {
					exists = true
					break
				}
			}
			if !exists {
				result = append(result, word)
			}
		}
	}
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
		// 地名：也使用 queryParts 分词搜索，以支持连字符地名匹配，但不在前端生成 tab
		places, _, err := s.searchPlaces(queryParts, language)
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

	// 验证表名和字段名（防止SQL注入）
	if !validateTableName(tableName) {
		return nil, nil, fmt.Errorf("invalid table name: %s", tableName)
	}
	if !validateFieldName(queryField) || !validateFieldName(nameField) ||
		!validateFieldName(nationField) || !validateFieldName(translationField) {
		return nil, nil, fmt.Errorf("invalid field name")
	}

	for _, query := range queryParts {
		// Process query using util functions directly
		processedQuery := util.ProcessQueryByLanguage(query, util.DetectLanguage(query))
		processedQuery = normalizeApostrophes(processedQuery)
		processedQuery = strings.ReplaceAll(processedQuery, "-", " ")
		// 处理括号：将 () 替换为空格，以匹配 "James (Son of Zebedee)" 这样的记录
		processedQuery = strings.ReplaceAll(processedQuery, "(", " ")
		processedQuery = strings.ReplaceAll(processedQuery, ")", " ")
		// 处理 St/St. -> Saint，以匹配数据库中的格式
		processedQuery = normalizeSaint(processedQuery)

		// 构建查找独立词的SQL查询
		var sqlQuery string
		var args []interface{}

		if language == "zh" {
			// 中文查询translation字段
			sqlQuery = fmt.Sprintf(
				"SELECT DISTINCT %s, %s, %s FROM %s WHERE %s LIKE ? ORDER BY %s LIMIT 100",
				nameField, nationField, translationField,
				tableName, translationField, nameField,
			)
			args = []interface{}{
				"%" + processedQuery + "%", // 包含查询词
			}
		} else {
			// 其他语言查询name_for_query字段
			// 注意：数据库中queryField前后都有空格，所以使用 LIKE '% word %'
			// 使用 REPLACE 处理括号，使 "Zebedee)" 能匹配 "Zebedee"
			sqlQuery = fmt.Sprintf(
				"SELECT DISTINCT %s, %s, %s FROM %s WHERE REPLACE(REPLACE(%s, '(', ' '), ')', ' ') LIKE ? ORDER BY %s LIMIT 100",
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

		// 添加到总结果（用于前端显示）
		allResults = append(allResults, queryResults...)
	}

	// 去重
	allResults = mergeAndDeduplicateResults(allResults)

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
	// 使用预编译的正则表达式处理单词边界
	return saintRegex.ReplaceAllString(query, "Saint ")
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

// sortResultsByQueryMatch 按”查询词匹配程度”排序：完全匹配最前，其次以查询词开头，再为包含查询词，同等级按 name 排序
// 原地排序，避免不必要的数组复制
func sortResultsByQueryMatch(results []SearchResult, query string) []SearchResult {
	if len(results) <= 1 {
		return results
	}
	sort.Slice(results, func(i, j int) bool {
		ri := matchRankForSort(results[i].Name, query)
		rj := matchRankForSort(results[j].Name, query)
		if ri != rj {
			return ri < rj
		}
		return strings.Compare(results[i].Name, results[j].Name) < 0
	})
	return results
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

	// 只生成人名 tab
	for i := 0; i < len(queryParts); i++ {
		query := queryParts[i]
		if results, ok := namesTab[query]; ok && len(results) > 0 {
			tabs = append(tabs, query)
			allTabResults[query] = results
		}
	}

	return tabs, allTabResults
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
		return queryParts, tabResults
	}

	// 第一个 tab 无结果，去掉第一个 tab
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