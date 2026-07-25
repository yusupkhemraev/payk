package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultsWithoutFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v, want defaults %+v", cfg, Default())
	}
}

func TestWorkspaceOverridesGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	write(t, filepath.Join(home, ".config", "payk", FileName),
		"theme: latte\nicons: none\nsidebar_width: 40\n")

	ws := t.TempDir()
	write(t, filepath.Join(ws, FileName), "icons: nerd\nlayout: columns\n")

	cfg, err := Load(ws)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Icons != IconsNerd || cfg.Layout != LayoutColumns {
		t.Errorf("workspace values not applied: %+v", cfg)
	}
	// Values only the global file sets must survive.
	if cfg.Theme != "latte" || cfg.SidebarWidth != 40 {
		t.Errorf("global values lost: %+v", cfg)
	}
}

func TestNormalizeRejectsUnknownValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := t.TempDir()
	write(t, filepath.Join(ws, FileName),
		"theme: neon\nicons: emoji\nlayout: diagonal\nsidebar_width: 500\n")

	cfg, err := Load(ws)
	if err != nil {
		t.Fatal(err)
	}
	def := Default()
	if cfg.Theme != def.Theme || cfg.Icons != def.Icons ||
		cfg.Layout != def.Layout || cfg.SidebarWidth != def.SidebarWidth {
		t.Errorf("unknown values should fall back to defaults, got %+v", cfg)
	}
}

func TestLoadReportsBrokenFileButKeepsDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := t.TempDir()
	write(t, filepath.Join(ws, FileName), "{{{not yaml")

	cfg, err := Load(ws)
	if err == nil {
		t.Fatal("want an error for malformed yaml")
	}
	if cfg.Theme != Default().Theme {
		t.Errorf("defaults should still apply, got %+v", cfg)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := t.TempDir()

	cfg := Default()
	cfg.Icons = IconsNerd
	cfg.Layout = LayoutColumns
	cfg.Wrap = true
	if err := Save(ws, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(ws)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != cfg {
		t.Errorf("round trip mismatch:\nsaved  %+v\nloaded %+v", cfg, loaded)
	}
}
