package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Migration is a domain migration.
type Migration struct {
	VersionDb *VersionDb

	// name - is the number + name of the migration. Example: 0001.InitMigration1
	Name string
}

func NewMigration(name string, versionDb *VersionDb) (*Migration, error) {
	if !isCorrectMigrationName(name) {
		return nil, fmt.Errorf("incorrect migration name")
	}

	return &Migration{
		VersionDb: versionDb,
		Name:      name,
	}, nil
}

func (first *Migration) IsEqual(second *Migration) bool {
	return first.VersionDb.Equal(second.VersionDb) && first.Name == second.Name
}

// Migration file name example: 0001.Example
func isCorrectMigrationName(name string) bool {
	partsOfTheName := strings.Split(name, ".")
	if len(partsOfTheName) != 2 {
		return false
	}

	numberInName := partsOfTheName[0]
	nameInName := partsOfTheName[1]

	if len(numberInName) != len("0001") {
		return false
	}
	_, err := strconv.Atoi(numberInName)
	if err != nil {
		return false
	}

	const minNameLength = 3
	if len(nameInName) < minNameLength {
		return false
	}

	return true
}
