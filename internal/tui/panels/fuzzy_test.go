package panels

import "testing"

func TestFuzzyMatchesSubsequences(t *testing.T) {
	tests := []struct {
		target, query string
		want          bool
	}{
		{"get user", "gu", true},
		{"get user", "gtu", true},
		{"get user", "user", true},
		{"get user", "xyz", false},
		{"get user", "usre", false},
		{"Отправка OTP", "отп", true},
		{"anything", "", true},
	}
	for _, tt := range tests {
		if _, ok := fuzzyScore(tt.target, tt.query); ok != tt.want {
			t.Errorf("fuzzyScore(%q, %q) matched = %v, want %v",
				tt.target, tt.query, ok, tt.want)
		}
	}
}

func TestFuzzyPrefersWordStartsAndRuns(t *testing.T) {
	// Initials of two words beat the same letters buried mid-word.
	initials, _ := fuzzyScore("get user", "gu")
	buried, _ := fuzzyScore("generate-upload-url", "gu")
	if initials <= buried {
		t.Errorf("word-start match should win: %d vs %d", initials, buried)
	}

	// A consecutive run beats the same characters spread out.
	run, _ := fuzzyScore("user", "use")
	spread, _ := fuzzyScore("update session end", "use")
	if run <= spread {
		t.Errorf("consecutive match should win: %d vs %d", run, spread)
	}
}

func TestFuzzyIsCaseInsensitive(t *testing.T) {
	upper, ok1 := fuzzyScore("Create User", "cu")
	lower, ok2 := fuzzyScore("create user", "CU")
	if !ok1 || !ok2 || upper != lower {
		t.Errorf("case must not change the result: %d/%v vs %d/%v", upper, ok1, lower, ok2)
	}
}
