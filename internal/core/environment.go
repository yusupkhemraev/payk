package core

// Environment vars serialize with sorted keys (yaml.v3), keeping diffs stable.
type Environment struct {
	Name string            `yaml:"name"`
	Vars map[string]string `yaml:"vars,omitempty"`
}

type Environments struct {
	Active       string        `yaml:"active,omitempty"`
	Environments []Environment `yaml:"environments,omitempty"`
}

func (e *Environments) Get(name string) *Environment {
	for i := range e.Environments {
		if e.Environments[i].Name == name {
			return &e.Environments[i]
		}
	}
	return nil
}

func (e *Environments) ActiveEnv() *Environment {
	return e.Get(e.Active)
}
