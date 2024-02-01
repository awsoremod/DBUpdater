package migration_disk

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dbupdater/helper"
	"dbupdater/internal/domain"
)

type MigrationDiskRepo struct {
	IsVerbose        bool
	PathToMigrations string
}

func NewMigrationDiskRepoo(pathToMigrations string, isVerbose bool) *MigrationDiskRepo {
	return &MigrationDiskRepo{
		IsVerbose:        isVerbose,
		PathToMigrations: pathToMigrations,
	}
}

// Returns migrations in the specified database version in ascending order starting from 0001
func (r *MigrationDiskRepo) GetSortedMigrations(_ context.Context, version *domain.VersionDb) (*domain.MigrationGroup, error) {
	versionString := version.String()
	migrationFiles, err := helper.GetFilesInDir(r.PathToMigrations + "/" + versionString)
	if err != nil {
		return nil, err
	}

	migrations := make([]domain.Migration, 0)
	for _, file := range migrationFiles {
		name := file.Name()
		extension := filepath.Ext(name)
		const correctExtension string = ".sql"
		if extension != correctExtension {
			helper.ShowIfVerbose(r.IsVerbose, fmt.Sprintf("%s %s is ignored because of a bad file extension.", versionString, name))
			continue
		}

		nameWithoutExtension := name[:len(name)-len(extension)]
		newMigration, err := domain.NewMigration(nameWithoutExtension, version)
		if err != nil {
			helper.ShowIfVerbose(r.IsVerbose, fmt.Sprintf("%s %s is ignored, err: %s", versionString, name, err))
			continue
		}
		migrations = append(migrations, *newMigration)
	}

	mg := domain.NewMigrationGroup(version, migrations)
	if err := migrationsStructureCheck(mg); err != nil {
		return nil, err
	}
	return mg, nil
}

// Returns sorted versionDb
func (r *MigrationDiskRepo) GetSortedVersions(_ context.Context) ([]*domain.VersionDb, error) {
	versionDirs, err := helper.GetFilesInDir(r.PathToMigrations)
	if err != nil {
		return nil, err
	}
	versions := make([]*domain.VersionDb, 0, 10)
	for _, versionOneDir := range versionDirs {
		nameDir := versionOneDir.Name()
		if helper.IsFirstV(nameDir) {
			v, err := domain.NewVersionDb(nameDir)
			if err != nil {
				helper.ShowIfVerbose(r.IsVerbose, fmt.Sprintf("%s is ignored. %v", nameDir, err))
				continue
			}

			versions = append(versions, v)
		}
	}
	sort.Sort(domain.Collection(versions))
	return versions, nil
}

func (r *MigrationDiskRepo) GetSqlFromMigration(_ context.Context, migration *domain.Migration) (string, error) {
	const extensionForMigrationFiles = "sql"
	pathToSqlFile := fmt.Sprintf("%s/%s/%s.%s", r.PathToMigrations, migration.VersionDb.String(), migration.Name, extensionForMigrationFiles)
	sql, err := helper.ReadFile(pathToSqlFile)
	if err != nil {
		return "", err
	}
	return sql, nil
}

// Checking the order of migrations in one database version
func migrationsStructureCheck(mg *domain.MigrationGroup) error {
	for i, migration := range mg.Migrations {
		number, err := getNumberInNameMigration(migration.Name)
		if err != nil {
			return err
		}

		if number != i+1 {
			return fmt.Errorf("wrong order of migrations, error on %s %s",
				migration.VersionDb.String(), migration.Name)
		}
	}
	return nil
}

// Migration name example: 0001.nameMigration
func getNumberInNameMigration(name string) (int, error) {
	strNumber := strings.Split(name, ".")[0]
	number, err := strconv.Atoi(strNumber)
	if err != nil {
		return 0, err
	}
	return number, nil
}
