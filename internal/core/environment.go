package core

// Environment is a named set of {{var}} substitution values.
// Vars serialize with sorted keys (yaml.v3), keeping diffs stable.
type Environment struct {
	Name string            `yaml:"name"`
	Vars map[string]string `yaml:"vars,omitempty"`
}

// Environments is the full environment config of a workspace.
type Environments struct {
	// Active is the name of the currently selected environment.
	Active       string        `yaml:"active,omitempty"`
	Environments []Environment `yaml:"environments,omitempty"`
}

// Get returns the environment with the given name, or nil.
func (e *Environments) Get(name string) *Environment {
	for i := range e.Environments {
		if e.Environments[i].Name == name {
			return &e.Environments[i]
		}
	}
	return nil
}

// ActiveEnv returns the active environment, or nil when none is selected.
func (e *Environments) ActiveEnv() *Environment {
	return e.Get(e.Active)
}
