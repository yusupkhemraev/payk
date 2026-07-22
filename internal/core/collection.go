package core

type Folder struct {
	Name     string
	Folders  []*Folder
	Requests []*Request
}

// Collection is a top-level request tree, e.g. one imported OpenAPI spec or
// one directory under the workspace collections root.
type Collection struct {
	Name     string
	Folders  []*Folder
	Requests []*Request
}

func (c *Collection) Empty() bool {
	return len(c.Folders) == 0 && len(c.Requests) == 0
}
