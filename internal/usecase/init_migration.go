package usecase

import (
	"fmt"

	"dbupdater/internal/domain"
)

type InitMigrationUseCase struct {
	initMigration *domain.Migration
}

const (
	stringZeroVersion       = "v0.0.0"
	stringZeroMigrationName = "0000.InitMod"
)

func NewInitMigrationUseCase() (*InitMigrationUseCase, error) {
	zeroVersion, err := domain.NewVersionDb(stringZeroVersion)
	if err != nil {
		return nil, fmt.Errorf("error when creating a version from %s: %w", stringZeroVersion, err)
	}

	initMigration, err := domain.NewMigration(stringZeroMigrationName, zeroVersion)
	if err != nil {
		return nil, fmt.Errorf("error when creating a migration from %s %s: %w", stringZeroVersion, stringZeroMigrationName, err)
	}

	return &InitMigrationUseCase{
		initMigration: initMigration,
	}, nil
}

func (uc *InitMigrationUseCase) GetInitMigration() *domain.Migration {
	return uc.initMigration
}
