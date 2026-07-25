package theme

import "github.com/yusupkhemraev/payk/internal/config"

// Icons is the glyph set for one config.Icons mode. Nerd glyphs need a Nerd
// Font; unicode works in any terminal; none keeps only the plain arrows the
// tree cannot do without.
type Icons struct {
	Folder     string
	FolderOpen string
	Search     string
	Send       string
	History    string
	Env        string
	Dirty      string
	Warning    string
}

func iconsFor(mode string) Icons {
	switch mode {
	case config.IconsNerd:
		return Icons{
			Folder:     "", // folder outline
			FolderOpen: "", // folder outline open
			Search:     "", // magnifier
			Send:       "", // paper plane
			History:    "", // history
			Env:        "", // globe
			Dirty:      "", // filled circle
			Warning:    "", // triangle
		}
	case config.IconsNone:
		return Icons{Folder: "▸", FolderOpen: "▾"}
	default:
		return Icons{
			Folder:     "▸",
			FolderOpen: "▾",
			Search:     "/",
			Send:       "→",
			History:    "↺",
			Env:        "◆",
			Dirty:      "●",
			Warning:    "!",
		}
	}
}
