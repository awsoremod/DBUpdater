package usecase

import (
	"context"
	"fmt"

	"dbupdater/helper"

	"dbupdater/internal/domain"
)

type MigrationRepoForMigrations interface {
	//  Sorted migrations that start with 0001 are expected
	GetSortedMigrations(ctx context.Context, version *domain.VersionDb) (*domain.MigrationGroup, error)

	// Expect sorted versions that are not repeated
	GetSortedVersions(ctx context.Context) ([]*domain.VersionDb, error)
}

type MigrationsUseCase struct {
	IsVerbose bool
	repo      MigrationRepoForMigrations
}

// New -.
func NewMigrationsUseCase(repo MigrationRepoForMigrations, isVerbose bool) *MigrationsUseCase {
	return &MigrationsUseCase{
		IsVerbose: isVerbose,
		repo:      repo,
	}
}

// Returns migrations of all database versions that are newer or equal than the specified one. In ascending order.
// If isInitMod true, all migrations will be returned for a database version that is equal to the currentMigration database version.
// Does not return a migrationGroup with empty migrations. The migrations in the migrationGroup are sorted as well.
func (uc *MigrationsUseCase) GetUnappliedSortedMigrations(ctx context.Context, isInitMod bool, currentMigration *domain.Migration) ([]domain.MigrationGroup, error) {
	helper.ShowIfVerbose(uc.IsVerbose, "The presence of new migrations in -migrations is analyzed...")

	unappliedMigrationGroups := make([]domain.MigrationGroup, 0)

	newSortedMigrationsInCurrentVersion, err := uc.getUnappliedSortedMigrationsForVersion(ctx, isInitMod, currentMigration)
	if err != nil {
		return nil, err
	}

	if len(newSortedMigrationsInCurrentVersion.Migrations) != 0 {
		unappliedMigrationGroups = append(unappliedMigrationGroups, *newSortedMigrationsInCurrentVersion)
	}
	versions, err := uc.repo.GetSortedVersions(ctx)
	if err != nil {
		return nil, err
	}
	for _, ver := range versions {
		isMore := ver.GreaterThan(currentMigration.VersionDb)
		if isMore {
			mg, err := uc.repo.GetSortedMigrations(ctx, ver)
			if err != nil {
				return nil, err
			}

			if len(mg.Migrations) != 0 {
				unappliedMigrationGroups = append(unappliedMigrationGroups, *mg)
			}
		}
	}

	helper.ShowIfVerbose(uc.IsVerbose, "Analysis successfully completed.")
	return unappliedMigrationGroups, nil
}

// Returns all migrations of the same database version that are newer than the specified migration. In ascending order.
// If isInitMod true, all migrations will be returned for a database version that is equal to the currentMigration database version.
func (uc *MigrationsUseCase) getUnappliedSortedMigrationsForVersion(ctx context.Context, isInitMod bool, currentMigration *domain.Migration) (*domain.MigrationGroup, error) {
	sortedMigrationsInSpecifiedVersion, err := uc.repo.GetSortedMigrations(ctx, currentMigration.VersionDb)
	if err != nil {
		return nil, err
	}

	if isInitMod {
		if len(sortedMigrationsInSpecifiedVersion.Migrations) == 0 {
			return nil, fmt.Errorf("in initialization mode there should be migrations with version %s", currentMigration.VersionDb.String())
		}
		return sortedMigrationsInSpecifiedVersion, nil
	}

	index := domain.IndexMigration(sortedMigrationsInSpecifiedVersion.Migrations, currentMigration)
	if index == -1 {
		return nil, fmt.Errorf("the %s %s migration is not in the migrations directory", currentMigration.VersionDb.String(), currentMigration.Name)
	}

	numberOfUnappliedMigrations := len(sortedMigrationsInSpecifiedVersion.Migrations) - index - 1
	unappliedMigrations := make([]domain.Migration, 0, numberOfUnappliedMigrations)

	for i := index + 1; i < len(sortedMigrationsInSpecifiedVersion.Migrations); i++ {
		unappliedMigrations = append(unappliedMigrations, sortedMigrationsInSpecifiedVersion.Migrations[i])
	}

	return domain.NewMigrationGroup(currentMigration.VersionDb, unappliedMigrations), nil
}
