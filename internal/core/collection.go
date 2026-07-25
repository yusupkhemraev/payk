package core

type Folder struct {
	Name     string
	Folders  []*Folder
	Requests []*Request
}

type Collection struct {
	Name     string
	Folders  []*Folder
	Requests []*Request
}

func (c *Collection) Empty() bool {
	return len(c.Folders) == 0 && len(c.Requests) == 0
}
