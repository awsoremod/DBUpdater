package domain

// The file is used to store functions that implement operations on domain entities.
// Functions are not allowed to be used in infrastructure.

// Searches for migrations in a slice of migrations groups. Returns the migration group index and migration index, in the migrations slice.
// If there is no migration, -1 -1 is returned.
func IndicesMigrationInMigrationGroups(mg []MigrationGroup, migration *Migration) (int, int) {
	indexMigrationGroup := -1
	indexM := -1
	for iMigrateGroup, mg := range mg {
		if mg.VersionDb.Equal(migration.VersionDb) {
			indexMigrationGroup = iMigrateGroup
			indexM = IndexMigration(mg.Migrations, migration)
		}
	}
	return indexMigrationGroup, indexM
}

// Returns the migration index. If the migration is not in the slice, returns -1
func IndexMigration(migrations []Migration, migration *Migration) int {
	if len(migrations) == 0 {
		return -1
	}
	if migration.IsEqual(&migrations[len(migrations)-1]) {
		return len(migrations) - 1
	}
	for i := 0; i < len(migrations)-1; i++ {
		if migration.IsEqual(&migrations[i]) {
			return i
		}
	}
	return -1
}
