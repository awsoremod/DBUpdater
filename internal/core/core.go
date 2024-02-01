package core

// The file is used to describe the algorithm of application operation.
// It is forbidden to call infrastructure methods.
// The project has a policy that usecase does not depend on usecase, infrastructure does not depend on infrastructure.

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"dbupdater/config"
	"dbupdater/helper"
	"dbupdater/internal/domain"
	"dbupdater/internal/usecase"

	"dbupdater/internal/infrastructure/dump_postgres"
	"dbupdater/internal/infrastructure/repo/migration_disk"
	"dbupdater/internal/infrastructure/repo/migration_postgres"

	"github.com/jackc/pgx/v5"
)

func Run(cfg *config.Config) {
	if cfg.Parameters.IsVersion {
		fmt.Printf("App version: %s", cfg.App.Version)
		return
	}
	checkRequiredParameters(cfg)

	ctx := context.Background()
	conn, err := helper.OpenConnect(ctx, &cfg.DbEntry, cfg.IsVerbose)
	if err != nil {
		log.Fatalf("Error when connecting to the database: %s", err)
	}
	defer conn.Close(ctx)

	repoMigrationPostgres := migration_postgres.NewMigrationPostgresRepo(conn)
	ucFileReader := usecase.NewFileReaderUseCase(cfg.PathToMigrations, cfg.IsVerbose)
	ucMigrationCurrent := usecase.NewMigrationCurrentUseCase(repoMigrationPostgres, cfg.IsVerbose)
	ucInitMigration, err := usecase.NewInitMigrationUseCase()
	if err != nil {
		log.Fatalf("Error when creating ucInitMigration: %s", err)
	}

	isInitModeON := isInitMode(ucFileReader, ucMigrationCurrent)
	if isInitModeON {
		fmt.Printf("Initialization mode - ON\n"+
			"Migrations will be applied starting from version %s\n", ucInitMigration.GetInitMigration().VersionDb.String())
	}

	currentMigration := determinationCurrentMigration(isInitModeON, ucInitMigration, ucFileReader, ucMigrationCurrent)
	showCurrentMigration(currentMigration)

	repoMigrationDisk := migration_disk.NewMigrationDiskRepoo(cfg.PathToMigrations, cfg.IsVerbose)
	ucMigrations := usecase.NewMigrationsUseCase(repoMigrationDisk, cfg.IsVerbose)

	unappliedMigrations, err := ucMigrations.GetUnappliedSortedMigrations(ctx, isInitModeON, currentMigration)
	if err != nil {
		log.Fatalf("Error when receiving unapplied migrations: %s", err)
	}
	if len(unappliedMigrations) == 0 {
		fmt.Printf("No new migrations")
		return
	}
	showUnappliedMigrations(unappliedMigrations)

	if cfg.StringVersionDb == "" {
		if isInitModeON {
			fmt.Printf("WARNING. If you specify some version in -versiondb, migrations will be applied starting from %s version.",
				ucInitMigration.GetInitMigration().VersionDb.String())
			return
		}
		if cfg.StringNameMigration == "" {
			return
		}
	}

	lastMigrationToMigrate := determinationLastMigrationToMigrate(cfg.StringVersionDb, cfg.StringNameMigration, currentMigration, unappliedMigrations)
	migrationsToMigrate := getMigrationGroupsAndMigrationsBeforeMigration(unappliedMigrations, lastMigrationToMigrate)

	sqlFromUpdateCurrentMigrationFile, err := ucFileReader.GetSqlFromUpdateCurrentMigrationFile()
	if err != nil {
		log.Fatalf("Error when retrieving sql text from %s: %s", ucFileReader.ShortPathToUpdateCurrentMigrationFile, err)
	}

	infraDumpPostgres, err := dump_postgres.NewDumpPostgres(cfg.DbEntry)
	if err != nil {
		log.Fatalf("Error when creating infraDumpPostgres: %s", err)
	}
	ucDump, err := usecase.NewDumpUseCase(infraDumpPostgres, cfg.IsVerbose)
	if err != nil {
		log.Fatalf("Error when creating ucDump: %s", err)
	}

	newDump, err := ucDump.Create(ctx)
	if err != nil {
		log.Fatalf("Error when creating a new dump: %s", err)
	}

	setupCloseHandler(ucDump, newDump, conn, ctx)

	ucMigrate := usecase.NewMigrateUseCase(repoMigrationDisk, repoMigrationPostgres, cfg.IsVerbose)
	if err := ucMigrate.Migrate(ctx, cfg.PathToMigrations, migrationsToMigrate); err != nil {
		fmt.Printf("Error when applying migrations: %v\n", err)
		conn.Close(ctx)
		if errFromRestore := ucDump.RestoreDatabaseFromDumpAndDeleteDump(ctx, newDump); errFromRestore != nil {
			err := ucDump.GetErrorForBadRestore(newDump)
			log.Fatalf("Error when restoring database from dump: %s: %s", errFromRestore, err)
		}
		return
	}

	lastMigrationGroup := migrationsToMigrate[len(migrationsToMigrate)-1]
	lastAppliedMigration := lastMigrationGroup.Migrations[len(lastMigrationGroup.Migrations)-1]
	if err := ucMigrationCurrent.UpdateCurrentMigration(ctx, sqlFromUpdateCurrentMigrationFile, &lastAppliedMigration); err != nil {
		fmt.Printf("Error when executing a query from %s: %v\n", ucFileReader.ShortPathToUpdateCurrentMigrationFile, err)
		conn.Close(ctx)
		if errFromRestore := ucDump.RestoreDatabaseFromDumpAndDeleteDump(ctx, newDump); errFromRestore != nil {
			err := ucDump.GetErrorForBadRestore(newDump)
			log.Fatalf("Error when restoring database from dump: %s: %s", errFromRestore, err)
		}
		return
	}

	if err := os.Remove(newDump.Path()); err != nil {
		fmt.Printf("Error when deleting dump file: %s\n", err)
		return
	}
	helper.ShowIfVerbose(cfg.IsVerbose, "Dump deleted.")

	conn.Close(ctx)
	return
}

func showCurrentMigration(currentMigration *domain.Migration) {
	fmt.Printf("Current database version: %s\n"+
		"Last migration applied: %s\n", currentMigration.VersionDb.String(), currentMigration.Name)
}

func showUnappliedMigrations(migrationGroups []domain.MigrationGroup) {
	fmt.Println("Updates available:")

	colorGreen := "\033[32m"
	colorReset := "\033[0m"

	fmt.Print(string(colorGreen))
	for _, mg := range migrationGroups {
		fmt.Printf("\n%s\n", mg.VersionDb.String())
		for i := 0; i < len(mg.Migrations); i++ {
			fmt.Printf("    %s\n", mg.Migrations[i].Name)
		}
	}
	fmt.Println(string(colorReset))
}

func checkRequiredParameters(cfg *config.Config) {
	if cfg.DbEntry.Host == "" || cfg.DbEntry.Port == "" || cfg.DbEntry.DbName == "" ||
		cfg.DbEntry.User == "" || cfg.DbEntry.Password == "" {
		log.Fatalf("Not all parameters for connection are specified. Familiarize yourself with them using -help.")
	}
	if cfg.PathToMigrations == "" {
		log.Fatalf("To view the current version of the database, the last applied migration, " +
			"apply new migrations, specify the path to the directory with migration scripts in the -migrations parameter")
	}
}

// Restore database from dump at Ctrl+C
func setupCloseHandler(ucDump *usecase.DumpUseCase, dump *domain.Dump, conn *pgx.Conn, ctx context.Context) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")

		conn.Close(ctx)

		err := ucDump.GetErrorForBadRestore(dump)
		log.Fatalf("An error may have occurred when applying migrations: %s", err)
	}()
}
