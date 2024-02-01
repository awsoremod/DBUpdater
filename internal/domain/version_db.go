package domain

import (
	"fmt"

	"dbupdater/helper"

	"github.com/hashicorp/go-version"
)

// Example: v0.0.1
type VersionDb struct {
	version *version.Version
}

func NewVersionDb(str string) (*VersionDb, error) {
	if !helper.IsFirstV(str) {
		return nil, fmt.Errorf("the database version must start with 'v'")
	}

	version, err := version.NewVersion(str)
	if err != nil {
		return nil, err
	}
	return &VersionDb{
		version: version,
	}, nil
}

func (v *VersionDb) String() string {
	return v.version.Original()
}

func (first *VersionDb) Equal(second *VersionDb) bool {
	return first.version.Equal(second.version)
}

func (first *VersionDb) GreaterThan(second *VersionDb) bool {
	return first.version.GreaterThan(second.version)
}

func (first *VersionDb) LessThan(second *VersionDb) bool {
	return first.version.LessThan(second.version)
}
