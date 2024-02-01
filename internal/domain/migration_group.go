package domain

// MigrationGroup is a domain MigrationGroup. Possible nil migrations
type MigrationGroup struct {
	VersionDb  *VersionDb
	Migrations []Migration
}

func NewMigrationGroup(versionDb *VersionDb, migrations []Migration) *MigrationGroup {
	return &MigrationGroup{
		VersionDb:  versionDb,
		Migrations: migrations,
	}
}

func (m *MigrationGroup) Copy() MigrationGroup {
	result := make([]Migration, len(m.Migrations))
	copy(result, m.Migrations)
	version, _ := NewVersionDb(m.VersionDb.String())
	return MigrationGroup{
		VersionDb:  version,
		Migrations: result,
	}
}
