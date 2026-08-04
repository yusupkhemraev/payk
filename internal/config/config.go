// Package config loads user preferences from config.yaml. A project-local
// .payk/config.yaml wins over the global ~/.config/payk/config.yaml, and
// unknown or missing values fall back to the defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	IconsNerd    = "nerd"
	IconsUnicode = "unicode"
	IconsNone    = "none"

	LayoutStacked = "stacked"
	LayoutColumns = "columns"
)

type Config struct {
	// Theme is a Catppuccin flavor: latte, frappe, macchiato, mocha.
	Theme string `yaml:"theme"`
	// Icons picks the glyph set: nerd needs a Nerd Font, unicode works
	// everywhere, none drops icons entirely.
	Icons string `yaml:"icons"`
	// Layout is stacked (request above response) or columns (three panes).
	Layout string `yaml:"layout"`

	// Transparent leaves the background unpainted so a blurred or
	// translucent terminal shows through.
	Transparent bool `yaml:"transparent"`

	LineNumbers      bool `yaml:"line_numbers"`
	SidebarWidth     int  `yaml:"sidebar_width"`
	ShowStatusInTree bool `yaml:"show_status_in_tree"`
	Wrap             bool `yaml:"wrap"`

	// Editor overrides $EDITOR for the body escape hatch.
	Editor string `yaml:"editor,omitempty"`
}

func Default() Config {
	return Config{
		Theme:            "mocha",
		Icons:            IconsUnicode,
		Layout:           LayoutStacked,
		LineNumbers:      true,
		SidebarWidth:     34,
		ShowStatusInTree: true,
	}
}

// FileName is the config file name inside a workspace or the global dir.
const FileName = "config.yaml"

// Load reads the config for a workspace directory, falling back to the
// global config and then to defaults. A missing file is not an error; a
// malformed one is reported with defaults still applied.
func Load(workspaceDir string) (Config, error) {
	cfg := Default()

	paths := make([]string, 0, 2)
	if global, err := globalPath(); err == nil {
		paths = append(paths, global)
	}
	if workspaceDir != "" {
		paths = append(paths, filepath.Join(workspaceDir, FileName))
	}

	var firstErr error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("config: read %s: %w", path, err)
			}
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("config: parse %s: %w", path, err)
		}
	}

	cfg.Normalize()
	return cfg, firstErr
}

// Save writes the config into a workspace directory.
func Save(workspaceDir string, cfg Config) error {
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return fmt.Errorf("config: create dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	path := filepath.Join(workspaceDir, FileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	return nil
}

// Normalize replaces unknown enum values and out-of-range numbers with
// defaults so a typo degrades gracefully instead of breaking the UI.
func (c *Config) Normalize() {
	def := Default()

	switch c.Theme {
	case "latte", "frappe", "macchiato", "mocha":
	case "espresso":
		// Espresso is Macchiato over a transparent background, matching the
		// terminal theme of the same name.
		c.Transparent = true
	default:
		c.Theme = def.Theme
	}
	switch c.Icons {
	case IconsNerd, IconsUnicode, IconsNone:
	default:
		c.Icons = def.Icons
	}
	switch c.Layout {
	case LayoutStacked, LayoutColumns:
	default:
		c.Layout = def.Layout
	}
	if c.SidebarWidth < 20 || c.SidebarWidth > 80 {
		c.SidebarWidth = def.SidebarWidth
	}
}

func globalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "payk", FileName), nil
}
