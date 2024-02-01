package migration_postgres

import (
	"dbupdater/internal/domain"
)

type migration struct {
	// Example: v0.0.1
	VersionDb string `db:"version_db"`

	// name - is the number + name of the migration. Example: 0001.InitMigration1
	Name string `db:"name"`
}

func migrationRepoToDomain(m *migration) (*domain.Migration, error) {
	version, err := domain.NewVersionDb(m.VersionDb)
	if err != nil {
		return nil, err
	}
	return domain.NewMigration(m.Name, version)
}

func migrationDomainToRepo(m *domain.Migration) *migration {
	return &migration{
		VersionDb: m.VersionDb.String(),
		Name:      m.Name,
	}
}
