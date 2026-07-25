package panels

import (
	"sort"
	"strings"
)

const maxSuggestions = 6

type suggestState struct {
	// start is the index right after the opening "{{".
	start   int
	matches []string
	index   int
}

func (s *suggestState) active() bool {
	return s != nil && len(s.matches) > 0
}

func (s *suggestState) selected() string {
	return s.matches[s.index]
}

func (s *suggestState) move(delta int) {
	s.index = (s.index + delta + len(s.matches)) % len(s.matches)
}

// varSuggestions inspects the text before the cursor for an unclosed {{ and
// returns completion candidates: active environment variables and, for the
// env: prefix, process environment names.
func varSuggestions(value string, cursor int, envVars, osEnv []string) *suggestState {
	if cursor > len(value) {
		cursor = len(value)
	}
	before := value[:cursor]
	open := strings.LastIndex(before, "{{")
	if open == -1 || strings.Contains(before[open:], "}}") {
		return nil
	}

	start := open + 2
	prefix := strings.TrimSpace(before[start:])

	var matches []string
	if envPrefix, ok := strings.CutPrefix(prefix, "env:"); ok {
		for _, name := range osEnv {
			if strings.HasPrefix(name, envPrefix) {
				matches = append(matches, "env:"+name)
			}
		}
	} else {
		for _, name := range envVars {
			if strings.HasPrefix(name, prefix) {
				matches = append(matches, name)
			}
		}
		if strings.HasPrefix("env:", prefix) {
			matches = append(matches, "env:")
		}
	}

	sort.Strings(matches)
	if len(matches) > maxSuggestions {
		matches = matches[:maxSuggestions]
	}
	if len(matches) == 0 {
		return nil
	}
	return &suggestState{start: start, matches: matches}
}

// apply completes the selected candidate into value, returning the new value
// and cursor position. The bare "env:" candidate keeps the placeholder open
// for further typing.
func (s *suggestState) apply(value string, cursor int) (string, int) {
	name := s.selected()
	closing := "}}"
	if name == "env:" {
		closing = ""
	}
	newValue := value[:s.start] + name + closing + value[cursor:]
	return newValue, s.start + len(name) + len(closing)
}
