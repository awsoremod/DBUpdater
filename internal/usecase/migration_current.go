package usecase

import (
	"context"

	"dbupdater/helper"
	"dbupdater/internal/domain"
)

type MigrationRepoForMigrationCurrent interface {
	HasCurrentMigration(ctx context.Context, sqlForCheckMigration string) (bool, error)
	GetCurrentMigration(ctx context.Context, sqlForGetMigration string) (*domain.Migration, error)
	UpdateCurrentMigration(ctx context.Context, sqlForUpdateMigration string, lastAppliedMigration *domain.Migration) (err error)
}

type MigrationCurrentUseCase struct {
	isVerbose bool
	repo      MigrationRepoForMigrationCurrent
}

func NewMigrationCurrentUseCase(repo MigrationRepoForMigrationCurrent, isVerbose bool) *MigrationCurrentUseCase {
	return &MigrationCurrentUseCase{
		repo:      repo,
		isVerbose: isVerbose,
	}
}

func (uc *MigrationCurrentUseCase) HasCurrentMigration(ctx context.Context, sqlForCheckMigration string) (bool, error) {
	helper.ShowIfVerbose(uc.isVerbose, "Checking availability database version and last applied migration...")
	isAvailable, err := uc.repo.HasCurrentMigration(ctx, sqlForCheckMigration)
	if err != nil {
		helper.ShowIfVerbose(uc.isVerbose, "The current version of the database is not available, error: "+err.Error())
		return false, err
	}
	if isAvailable {
		helper.ShowIfVerbose(uc.isVerbose, "The current version of the database is available.")
		return true, nil
	}
	helper.ShowIfVerbose(uc.isVerbose, "The current version of the database is not available.")
	return false, nil
}

func (uc *MigrationCurrentUseCase) GetCurrentMigration(ctx context.Context, sqlForGetMigration string) (*domain.Migration, error) {
	helper.ShowIfVerbose(uc.isVerbose, "Getting the database version and last applied migration...")
	currentMigration, err := uc.repo.GetCurrentMigration(ctx, sqlForGetMigration)
	if err != nil {
		return nil, err
	}
	helper.ShowIfVerbose(uc.isVerbose, "VersionDb and last applied migration successfully retrieved.")
	return currentMigration, nil
}

func (uc *MigrationCurrentUseCase) UpdateCurrentMigration(ctx context.Context, sqlForUpdateMigration string, lastAppliedMigration *domain.Migration) (err error) {
	helper.ShowIfVerbose(uc.isVerbose, "Information about the current database version and the last applied migration is updated...")
	if err := uc.repo.UpdateCurrentMigration(ctx, sqlForUpdateMigration, lastAppliedMigration); err != nil {
		return err
	}
	helper.ShowIfVerbose(uc.isVerbose, "Information about the current database version and the last applied migration has been successfully updated.")
	return nil
}
