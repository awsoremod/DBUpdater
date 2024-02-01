package domain

import "fmt"

type Dump struct {
	path string
}

func NewDump(path string) (*Dump, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: dump path is required", ErrRequired)
	}

	return &Dump{
		path: path,
	}, nil
}

// Path returns the dump path.
func (d *Dump) Path() string {
	return d.path
}
