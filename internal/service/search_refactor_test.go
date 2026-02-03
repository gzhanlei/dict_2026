package service

import (
	"testing"

	"dict_new/config"
	"dict_new/internal/database"
)

// TestSearchTable_Consolidated tests the unified searchTable function
// that replaces the redundant searchNames and searchPlaces functions
func TestSearchTable_Consolidated(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "",
			DBName:   "mysql",
		},
		Tables: config.TablesConfig{
			Names:  "names",
			Places: "places",
		},
		Fields: config.FieldsConfig{
			QueryField:       "name_for_query_en",
			NameField:        "name",
			NationField:      "nation",
			TranslationField: "translation_zh",
		},
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_search_refactor",
			Expiration: 3600,
		},
	}

	db, err := database.NewConnection(cfg)
	if err != nil {
		t.Skipf("Skipping test, cannot connect to database: %v", err)
		return
	}
	defer db.Close()

	cache, err := NewCacheService(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache service: %v", err)
	}

	search, err := NewSearchService(db, cache, cfg)
	if err != nil {
		t.Fatalf("Failed to create search service: %v", err)
	}

	tests := []struct {
		name       string
		queryParts []string
		language   string
		tableName  string
		wantErr    bool
	}{
		{
			name:       "empty query parts returns empty results",
			queryParts: []string{},
			language:   "en",
			tableName:  "names",
			wantErr:    false,
		},
		{
			name:       "single query part english",
			queryParts: []string{"George"},
			language:   "en",
			tableName:  "names",
			wantErr:    false,
		},
		{
			name:       "chinese query uses translation field",
			queryParts: []string{"乔治"},
			language:   "zh",
			tableName:  "names",
			wantErr:    false,
		},
		{
			name:       "places table works same as names",
			queryParts: []string{"London"},
			language:   "en",
			tableName:  "places",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, tabResults, err := search.searchTable(tt.queryParts, tt.language, tt.tableName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("searchTable() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			if err != nil {
				// Skip tests that require database tables
				t.Skipf("Skipping test, database query failed (table may not exist): %v", err)
				return
			}
			// For empty query parts, expect empty results
			if len(tt.queryParts) == 0 && (len(results) != 0 || len(tabResults) != 0) {
				t.Errorf("searchTable() with empty query parts should return empty results, got %d results", len(results))
			}
		})
	}
}

// TestSortResultsByQueryMatch tests the result sorting function
func TestSortResultsByQueryMatch(t *testing.T) {
	tests := []struct {
		name     string
		results  []SearchResult
		query    string
		expected []string // expected order of names
	}{
		{
			name:     "empty results",
			results:  []SearchResult{},
			query:    "test",
			expected: []string{},
		},
		{
			name: "exact match first",
			results: []SearchResult{
				{Name: "John Smith", Nation: "US", Translation: "约翰·史密斯"},
				{Name: "John", Nation: "US", Translation: "约翰"},
				{Name: "Johnny", Nation: "US", Translation: "约翰尼"},
			},
			query:    "John",
			expected: []string{"John", "John Smith", "Johnny"},
		},
		{
			name: "starts with second priority",
			results: []SearchResult{
				{Name: "Smith John", Nation: "US", Translation: "史密斯·约翰"},
				{Name: "John Smith", Nation: "US", Translation: "约翰·史密斯"},
			},
			query:    "John",
			expected: []string{"John Smith", "Smith John"},
		},
		{
			name: "case insensitive match",
			results: []SearchResult{
				{Name: "john", Nation: "US", Translation: "约翰"},
				{Name: "JOHN", Nation: "US", Translation: "约翰"},
			},
			query:    "John",
			expected: []string{"JOHN", "john"}, // same rank (0), sorted by string comparison
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortResultsByQueryMatch(tt.results, tt.query)
			if len(got) != len(tt.expected) {
				t.Errorf("sortResultsByQueryMatch() got %d results, want %d", len(got), len(tt.expected))
				return
			}
			for i, result := range got {
				if result.Name != tt.expected[i] {
					t.Errorf("sortResultsByQueryMatch()[%d].Name = %q, want %q", i, result.Name, tt.expected[i])
				}
			}
		})
	}
}

// TestMatchRankForSort tests the match ranking function
func TestMatchRankForSort(t *testing.T) {
	tests := []struct {
		name     string
		result   string
		query    string
		expected int // 0=exact, 1=starts with, 2=contains word, 3=other
	}{
		{"exact match", "John", "John", 0},
		{"exact case insensitive", "john", "John", 0},
		{"starts with space", "John Smith", "John", 1},
		{"starts with comma", "John, Smith", "John", 1},
		{"contains as word", "Mr John Doe", "John", 2},
		{"ends with word", "Mr John", "John", 2},
		{"contains with comma", "Smith, John", "John", 2},
		{"no match", "Smith", "John", 3},
		{"empty query", "John", "", 3},
		{"substring not word", "Johnson", "John", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchRankForSort(tt.result, tt.query)
			if got != tt.expected {
				t.Errorf("matchRankForSort(%q, %q) = %d, want %d", tt.result, tt.query, got, tt.expected)
			}
		})
	}
}

// TestNormalizeSaint tests the saint normalization function
func TestNormalizeSaint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"St Louis", "Saint Louis"},
		{"St. Louis", "Saint Louis"},
		{"st louis", "Saint louis"},
		{"ST. LOUIS", "Saint LOUIS"},
		{"Saint Louis", "Saint Louis"},
		{"Street", "Street"}, // should not match "St" at end of word
		{"Main St", "Main St"}, // should not match without space
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeSaint(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeSaint(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestHasExactMatch tests the exact match checker
func TestHasExactMatch(t *testing.T) {
	results := []SearchResult{
		{Name: "John Smith", Nation: "US", Translation: "约翰·史密斯"},
		{Name: "John", Nation: "US", Translation: "约翰"},
		{Name: "Jane Doe", Nation: "US", Translation: "简·多伊"},
	}

	tests := []struct {
		name     string
		results  []SearchResult
		query    string
		expected bool
	}{
		{"exact match exists", results, "John", true},
		{"exact match case insensitive", results, "john", true},
		{"exact match with spaces", results, "  John  ", true},
		{"no exact match", results, "Johnny", false},
		{"empty results", []SearchResult{}, "John", false},
		{"partial match not exact", results, "John Smith", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasExactMatch(tt.results, tt.query)
			if got != tt.expected {
				t.Errorf("hasExactMatch() = %v, want %v", got, tt.expected)
			}
		})
	}
}
