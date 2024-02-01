package core

// The file is used to store functions that implement parts of the business logic.
// The file is often used to store functions that generalize business processes, limit the scope, and combine usecase.

import (
	"context"
	"log"

	"dbupdater/internal/domain"
	"dbupdater/internal/usecase"
)

func isInitMode(ucFileReader *usecase.FileReaderUseCase, ucMigrationCurrent *usecase.MigrationCurrentUseCase) bool {
	isExistHasCurrentMigrationFile := ucFileReader.IsExistHasCurrentMigrationFile()
	if !isExistHasCurrentMigrationFile {
		return false
	}
	ctx := context.Background()

	sqlFromHasCurrentMigrationFile, err := ucFileReader.GetSqlFromHasCurrentMigrationFile()
	if err != nil {
		log.Fatalf("Error when retrieving sql text from %s: %s", ucFileReader.ShortPathToHasCurrentMigrationFile, err)
	}
	isAvailableCurrentMigrationInDatabase, err := ucMigrationCurrent.HasCurrentMigration(ctx, sqlFromHasCurrentMigrationFile)
	if err != nil || !isAvailableCurrentMigrationInDatabase {
		return true
	}
	return false
}

func determinationCurrentMigration(isInitMod bool, ucInitMigration *usecase.InitMigrationUseCase,
	ucFileReader *usecase.FileReaderUseCase, ucMigrationCurrent *usecase.MigrationCurrentUseCase,
) *domain.Migration {
	if isInitMod {
		return ucInitMigration.GetInitMigration()
	}
	ctx := context.Background()

	sqlFromGetCurrentMigrationFile, err := ucFileReader.GetSqlFromGetCurrentMigrationFile()
	if err != nil {
		log.Fatalf("Error when retrieving sql text from %s: %s", ucFileReader.ShortPathToGetCurrentMigrationFile, err)
	}
	currentMigration, err := ucMigrationCurrent.GetCurrentMigration(ctx, sqlFromGetCurrentMigrationFile)
	if err != nil {
		log.Fatalf("Error when retrieving the current database version and the last applied migration: %s", err)
	}
	return currentMigration
}

func determinationLastMigrationToMigrate(stringVersionDb string, stringNameMigration string,
	currentMigration *domain.Migration, unappliedMigrations []domain.MigrationGroup,
) *domain.Migration {
	var err error
	var versiondb *domain.VersionDb
	if stringVersionDb == "" {
		versiondb = currentMigration.VersionDb
	} else {
		versiondb, err = domain.NewVersionDb(stringVersionDb)
		if err != nil {
			log.Fatalf("wrong version in -versiondb: %s", err)
		}
	}

	var lastMigrationToMigrate *domain.Migration
	if stringNameMigration == "" {
		for _, mg := range unappliedMigrations {
			if mg.VersionDb.Equal(versiondb) {
				lastMigrationToMigrate = &mg.Migrations[len(mg.Migrations)-1]
				return lastMigrationToMigrate
			}
		}
		log.Fatalf("The %s version is not in the list of migrations available for updating", versiondb.String())
	} else {
		lastMigrationToMigrate, err = domain.NewMigration(stringNameMigration, versiondb)
		if err != nil {
			log.Fatalf("Incorrect migration from -versiondb and -migration parameters: %s", err)
		}
		lastIndexMigrationGroup, lastIndexMigration := domain.IndicesMigrationInMigrationGroups(unappliedMigrations, lastMigrationToMigrate)
		if lastIndexMigrationGroup == -1 || lastIndexMigration == -1 {
			log.Fatalf("The %s %s migration is not in the list of migrations available for updating",
				lastMigrationToMigrate.VersionDb.String(), lastMigrationToMigrate.Name)
		}
	}
	return lastMigrationToMigrate
}

// Returns all migration groups with migrations before the specified migration
func getMigrationGroupsAndMigrationsBeforeMigration(sortedMigrationGroups []domain.MigrationGroup, beforeThisMigration *domain.Migration) []domain.MigrationGroup {
	lastIndexMigrationGroup, lastIndexMigration := domain.IndicesMigrationInMigrationGroups(sortedMigrationGroups, beforeThisMigration)
	if lastIndexMigrationGroup == -1 || lastIndexMigration == -1 {
		log.Fatalf("the %s %s migration or database version is not in the list of migrations available for updating",
			beforeThisMigration.VersionDb, beforeThisMigration.Name)
	}

	migrationGroupsUpTo := make([]domain.MigrationGroup, lastIndexMigrationGroup+1)

	for i := 0; i < len(migrationGroupsUpTo)-1; i++ {
		copiedMigrationGroup := sortedMigrationGroups[i].Copy()
		migrationGroupsUpTo[i] = copiedMigrationGroup
	}

	lastMigrationGroup := sortedMigrationGroups[lastIndexMigrationGroup]
	isLastMigrationInLastMigrationGroup := lastIndexMigration == len(lastMigrationGroup.Migrations)-1
	if isLastMigrationInLastMigrationGroup {
		coppiedLastMigrationGroup := lastMigrationGroup.Copy()
		migrationGroupsUpTo[lastIndexMigrationGroup] = coppiedLastMigrationGroup
		return migrationGroupsUpTo
	}

	numberMigrationsInLastMigrationGroup := lastIndexMigration + 1
	migrationsInLastVersionDbUpTo := make([]domain.Migration, numberMigrationsInLastMigrationGroup)
	for i := 0; i < numberMigrationsInLastMigrationGroup; i++ {
		migrationsInLastVersionDbUpTo[i] = lastMigrationGroup.Migrations[i]
	}

	newMigrationGroup := domain.NewMigrationGroup(lastMigrationGroup.VersionDb, migrationsInLastVersionDbUpTo)
	migrationGroupsUpTo[lastIndexMigrationGroup] = *newMigrationGroup

	return migrationGroupsUpTo
}
