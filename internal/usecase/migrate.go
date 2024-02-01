package usecase

import (
	"context"
	"fmt"
	"os"

	"dbupdater/helper"
	"dbupdater/internal/domain"
)

type GetSqlFromRepo interface {
	GetSqlFromMigration(ctx context.Context, migration *domain.Migration) (string, error)
}

type ExecSqlByUsingRepo interface {
	ExecSql(ctx context.Context, sql string) (err error)
}

type MigrateUseCase struct {
	isVerbose bool
	getRepo   GetSqlFromRepo
	execRepo  ExecSqlByUsingRepo
}

func NewMigrateUseCase(getRepo GetSqlFromRepo, execRepo ExecSqlByUsingRepo, isVerbose bool) *MigrateUseCase {
	return &MigrateUseCase{
		getRepo:   getRepo,
		execRepo:  execRepo,
		isVerbose: isVerbose,
	}
}

func (uc *MigrateUseCase) Migrate(ctx context.Context, pathToMigrations string, migrationsToMigrate []domain.MigrationGroup) error {
	colorGreen := "\033[32m"
	colorRed := "\033[31m"
	colorReset := "\033[0m"

	fmt.Printf("Migrations started to apply...\n")
	for _, mg := range migrationsToMigrate {
		migrations := mg.Migrations
		for _, migration := range migrations {
			mgVersionDbString := mg.VersionDb.String()
			migrationName := migration.Name
			ctx := context.Background()

			helper.ShowIfVerbose(uc.isVerbose, fmt.Sprintf("Applied: %s %s", mgVersionDbString, migrationName))
			sql, err := uc.getRepo.GetSqlFromMigration(ctx, &migration)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying migration: %s%s %s%s\n", colorRed, mgVersionDbString, migrationName, colorReset)
				return err
			}
			if err := uc.execRepo.ExecSql(ctx, sql); err != nil {
				fmt.Fprintf(os.Stderr, "Error applying migration: %s%s %s%s\n", colorRed, mgVersionDbString, migrationName, colorReset)
				return err
			}
			fmt.Println(fmt.Sprintf("Ready: %s%s %s%s", colorGreen, mgVersionDbString, migrationName, colorReset))
		}
	}
	fmt.Printf("Migrations have been applied.\n")
	return nil
}
