package core

// Folder is a named group of requests and nested folders.
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

// Empty reports whether the collection has no folders and no requests.
func (c *Collection) Empty() bool {
	return len(c.Folders) == 0 && len(c.Requests) == 0
}
