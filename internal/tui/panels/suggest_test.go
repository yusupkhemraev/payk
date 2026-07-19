package panels

import (
	"reflect"
	"testing"
)

var (
	testVars  = []string{"api_token", "base_url"}
	testOSEnv = []string{"HOME", "HOSTNAME", "PATH"}
)

func TestVarSuggestionsMatchesPrefix(t *testing.T) {
	s := varSuggestions("{{ba", 4, testVars, testOSEnv)
	if s == nil || !reflect.DeepEqual(s.matches, []string{"base_url"}) {
		t.Fatalf("suggestions = %+v", s)
	}

	value, cursor := s.apply("{{ba", 4)
	if value != "{{base_url}}" || cursor != len(value) {
		t.Errorf("apply = %q, cursor %d", value, cursor)
	}
}

func TestVarSuggestionsMidString(t *testing.T) {
	input := "https://x/{{ba/users"
	s := varSuggestions(input, 14, testVars, testOSEnv) // cursor after "{{ba"
	if s == nil {
		t.Fatal("no suggestions mid-string")
	}
	value, cursor := s.apply(input, 14)
	if value != "https://x/{{base_url}}/users" {
		t.Errorf("apply = %q", value)
	}
	if value[cursor-2:cursor] != "}}" {
		t.Errorf("cursor %d should sit right after }}", cursor)
	}
}

func TestVarSuggestionsEnvPrefix(t *testing.T) {
	s := varSuggestions("{{env:HO", 8, testVars, testOSEnv)
	if s == nil || !reflect.DeepEqual(s.matches, []string{"env:HOME", "env:HOSTNAME"}) {
		t.Fatalf("suggestions = %+v", s)
	}
	value, _ := s.apply("{{env:HO", 8)
	if value != "{{env:HOME}}" {
		t.Errorf("apply = %q", value)
	}
}

func TestVarSuggestionsEmptyPrefixListsVarsAndEnvEntry(t *testing.T) {
	s := varSuggestions("{{", 2, testVars, testOSEnv)
	if s == nil || !reflect.DeepEqual(s.matches, []string{"api_token", "base_url", "env:"}) {
		t.Fatalf("suggestions = %+v", s)
	}

	// The bare env: entry completes without closing the placeholder.
	s.index = 2
	value, cursor := s.apply("{{", 2)
	if value != "{{env:" || cursor != 6 {
		t.Errorf("apply env: = %q, cursor %d", value, cursor)
	}
}

func TestVarSuggestionsInactiveCases(t *testing.T) {
	for _, tc := range []struct {
		value  string
		cursor int
	}{
		{"no placeholders here", 10},
		{"{{done}} after", 14},
		{"{{zzz", 5},
	} {
		if s := varSuggestions(tc.value, tc.cursor, testVars, testOSEnv); s != nil {
			t.Errorf("varSuggestions(%q, %d) = %+v, want nil", tc.value, tc.cursor, s)
		}
	}
}

func TestSuggestStateCycles(t *testing.T) {
	s := &suggestState{matches: []string{"a", "b", "c"}}
	s.move(1)
	s.move(1)
	if s.selected() != "c" {
		t.Errorf("selected = %q", s.selected())
	}
	s.move(1)
	if s.selected() != "a" {
		t.Errorf("wrap forward failed: %q", s.selected())
	}
	s.move(-1)
	if s.selected() != "c" {
		t.Errorf("wrap backward failed: %q", s.selected())
	}
}
