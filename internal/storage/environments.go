package storage

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/yusupkhemraev/payk/internal/core"
)

// LoadEnvironments reads environments.yaml. A missing file yields an empty
// config, not an error.
func (w *Workspace) LoadEnvironments() (*core.Environments, error) {
	data, err := os.ReadFile(w.EnvironmentsFile())
	if os.IsNotExist(err) {
		return &core.Environments{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: read environments: %w", err)
	}
	var envs core.Environments
	if err := yaml.Unmarshal(data, &envs); err != nil {
		return nil, fmt.Errorf("storage: parse environments: %w", err)
	}
	return &envs, nil
}

// SaveEnvironments writes environments.yaml. Var maps marshal with sorted
// keys, so output is deterministic. Callers must never pass resolved
// {{env:...}} secrets here; substitution happens only in memory at send time.
func (w *Workspace) SaveEnvironments(envs *core.Environments) error {
	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return fmt.Errorf("storage: create workspace dir: %w", err)
	}
	data, err := marshalYAML(envs)
	if err != nil {
		return fmt.Errorf("storage: marshal environments: %w", err)
	}
	if err := os.WriteFile(w.EnvironmentsFile(), data, 0o644); err != nil {
		return fmt.Errorf("storage: write environments: %w", err)
	}
	return nil
}
