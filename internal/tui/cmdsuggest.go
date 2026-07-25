package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type commandSpec struct {
	name string
	desc string
	// args marks commands that take an argument: accepting them appends a
	// space so completion drills into the argument level.
	args bool
}

var commandSpecs = []commandSpec{
	{name: "send", desc: "send current request"},
	{name: "w", desc: "write request to disk"},
	{name: "env", desc: "environments: list · switch · new", args: true},
	{name: "set", desc: "set variable in active environment", args: true},
	{name: "unset", desc: "remove variable", args: true},
	{name: "import", desc: "import curl / OpenAPI / FastAPI", args: true},
	{name: "reimport", desc: "re-run recorded imports", args: true},
	{name: "messages", desc: "message log"},
	{name: "q", desc: "quit"},
}

func (m *Model) refreshCmdSuggestions() {
	m.cmdline.setSuggestions(m.commandSuggestions())
}

func (m *Model) commandSuggestions() []cmdSuggestion {
	input := strings.TrimLeft(m.cmdline.value(), " ")
	head, rest, hasSpace := strings.Cut(input, " ")

	if !hasSpace {
		var out []cmdSuggestion
		for _, spec := range commandSpecs {
			if !strings.HasPrefix(spec.name, head) {
				continue
			}
			full := spec.name
			if spec.args {
				full += " "
			}
			out = append(out, cmdSuggestion{full: full, label: spec.name, desc: spec.desc})
		}
		return out
	}

	switch head {
	case "env":
		return m.envArgSuggestions(rest)
	case "set", "unset":
		return m.varArgSuggestions(head, rest)
	case "reimport":
		return m.reimportArgSuggestions(rest)
	}
	return nil
}

func (m *Model) envArgSuggestions(rest string) []cmdSuggestion {
	var out []cmdSuggestion
	for _, env := range m.envs.Environments {
		if !strings.HasPrefix(env.Name, rest) {
			continue
		}
		desc := "switch environment"
		if env.Name == m.envs.Active {
			desc = "active now"
		}
		out = append(out, cmdSuggestion{full: "env " + env.Name, label: env.Name, desc: desc})
	}
	if strings.HasPrefix("new", rest) {
		out = append(out, cmdSuggestion{full: "env new ", label: "new", desc: "create environment"})
	}
	return out
}

func (m *Model) varArgSuggestions(head, rest string) []cmdSuggestion {
	env := m.envs.ActiveEnv()
	if env == nil {
		return nil
	}
	names := make([]string, 0, len(env.Vars))
	for name := range env.Vars {
		if strings.HasPrefix(name, rest) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	out := make([]cmdSuggestion, 0, len(names))
	for _, name := range names {
		full := head + " " + name
		desc := "current: " + env.Vars[name]
		if head == "set" {
			full += " "
			desc = "was: " + env.Vars[name]
		}
		out = append(out, cmdSuggestion{full: full, label: name, desc: desc})
	}
	return out
}

func (m Model) cmdSuggestionLine(width int) string {
	suggestions := m.cmdline.suggestions
	if len(suggestions) == 0 || width < 10 {
		return ""
	}

	parts := make([]string, 0, len(suggestions))
	for i, s := range suggestions {
		if i == m.cmdline.suggIdx {
			parts = append(parts, m.theme.Selected.Render(" "+s.label+" "))
		} else {
			parts = append(parts, m.theme.StatusHint.Render(s.label))
		}
	}

	line := strings.Join(parts, " ")
	selected := suggestions[m.cmdline.suggIdx]
	if selected.desc != "" {
		line += m.theme.StatusHint.Render(" — " + selected.desc)
	}
	line += m.theme.StatusHint.Render("  · tab complete")
	return ansi.Truncate(line, width, "…")
}

func (m *Model) reimportArgSuggestions(rest string) []cmdSuggestion {
	var out []cmdSuggestion
	for _, name := range m.importSources {
		if strings.HasPrefix(name, rest) {
			out = append(out, cmdSuggestion{
				full: "reimport " + name, label: name, desc: "refresh from source",
			})
		}
	}
	return out
}
