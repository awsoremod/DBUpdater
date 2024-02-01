package dump_postgres

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dbupdater/config"
	"dbupdater/helper"
	"dbupdater/internal/domain"
)

type DumpPostgres struct {
	pathToDumpUtility    string
	pathToRestoreUtility string
	dbEntry              config.DbEntry
}

func NewDumpPostgres(dbEntry config.DbEntry) (*DumpPostgres, error) {
	pathToDumpUtility, err := getPathToDumpUtility()
	if err != nil {
		return nil, err
	}
	pathToRestoreUtility, err := getPathToRestoreUtility()
	if err != nil {
		return nil, err
	}

	return &DumpPostgres{
		dbEntry:              dbEntry,
		pathToDumpUtility:    pathToDumpUtility,
		pathToRestoreUtility: pathToRestoreUtility,
	}, nil
}

// Creates a database dump using the pg_dump utility. The dump file
// is placed in the specified folder. A connection string is also created in the pgpass file.
// The dump contains information about the database owner. The path to the dump is returned.
func (uc *DumpPostgres) Create(_ context.Context, pathForSaveDumps string) (*domain.Dump, error) {
	pathToDump := uc.getPathForNewDump(pathForSaveDumps)
	dump, err := domain.NewDump(pathToDump)
	if err != nil {
		return nil, err
	}
	parameters := uc.getParametersForDumpUtility(pathToDump)

	cmd := exec.Command(uc.pathToDumpUtility, parameters...)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+uc.dbEntry.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If an error occurs, an empty dump file is created
		return nil, fmt.Errorf("%w: %s", err, output)
	}

	return dump, nil
}

// Starts the pg_restore utility
func (infra *DumpPostgres) Restore(_ context.Context, dump *domain.Dump) error {
	parameters := infra.getParametersForRestoreUtility(dump.Path())

	cmd := exec.Command(infra.pathToRestoreUtility, parameters...)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+infra.dbEntry.Password)

	output, err := cmd.CombinedOutput()
	strOutput := string(output)

	detailIndex := strings.Index(strOutput, "DETAIL:")
	if detailIndex != -1 {
		lastIndex := helper.IndexAt(strOutput, "\n", detailIndex+1)
		strOutput = strOutput[:lastIndex+1]
	}
	if err != nil {
		return fmt.Errorf("%s: %w", strOutput, err)
	}
	return nil
}

func (infra *DumpPostgres) GetCommandToRestoreDump(dump *domain.Dump) (string, error) {
	pathToRestoreUtility, err := getPathToRestoreUtility()
	if err != nil {
		return "", err
	}

	params := infra.getParametersForRestoreUtility(dump.Path())
	strParams := strings.Join(params, " ")
	commandToRestoreDump := pathToRestoreUtility + ` ` + strParams
	commandToRestoreDump = strings.Replace(commandToRestoreDump, " --no-password ", " ", 1)
	return commandToRestoreDump, nil
}

// Forms the name and path to the dump file
func (infra *DumpPostgres) getPathForNewDump(pathForSaveDumps string) string {
	t := time.Now()
	year := t.Year()
	month := int(t.Month())
	day := t.Day()
	hour := t.Hour()
	minute := t.Minute()
	milli := t.UnixMilli()

	pathToDump := fmt.Sprintf("%s/%s_%d-%d-%d_%d-%d-%d.dump", pathForSaveDumps, infra.dbEntry.DbName, year, month, day, hour, minute, milli)

	return pathToDump
}

func (infra *DumpPostgres) getParametersForDumpUtility(pathToDump string) []string {
	parameters := []string{
		`--host=` + infra.dbEntry.Host,
		`--port=` + infra.dbEntry.Port,
		`--username=` + infra.dbEntry.User,
		`--no-password`,
		`--format=` + `custom`,
		`--create`,
		`--clean`,
		`--if-exists`,
		`--dbname=` + infra.dbEntry.DbName,
		`--file=` + pathToDump,
	}

	return parameters
}

func getPathToDumpUtility() (string, error) {
	pathToExecutable, err := os.Executable()
	if err != nil {
		return "", err
	}
	pathToDirWithCurrentlyExecutable := filepath.Dir(pathToExecutable)
	pathToDumpUtility := fmt.Sprintf("%s/pg_dump_restore_15_2/pg_dump.exe", pathToDirWithCurrentlyExecutable)
	return pathToDumpUtility, nil
}

// Forms a parameters to run the pg_restore utility.
// First the existing database is deleted, then the database from the dump is created.
// Connects to the database 'postgres'
// To restore the database:
// 1) You need permissions to connect to the 'postgres' database
// 1) You need permissions to delete the database with the same name as in the dump.
// 3) You must be a member of the database owner role that is specified in the dump.
// 4) There should be no active connections to the database with the same name as in the dump.
func (infra *DumpPostgres) getParametersForRestoreUtility(pathToDump string) []string {
	parameters := []string{
		`--host=` + infra.dbEntry.Host,
		`--port=` + infra.dbEntry.Port,
		`--username=` + infra.dbEntry.User,
		`--no-password`,
		`--format=` + `custom`,
		`--create`,
		`--clean`,
		`--if-exists`,
		`--dbname=` + `postgres`,
		pathToDump,
	}
	return parameters
}

func getPathToRestoreUtility() (string, error) {
	pathToExecutable, err := os.Executable()
	if err != nil {
		return "", err
	}
	pathToDirWithCurrentlyExecutable := filepath.Dir(pathToExecutable)
	pathToRestoreUtility := fmt.Sprintf("%s/pg_dump_restore_15_2/pg_restore.exe", pathToDirWithCurrentlyExecutable)
	return pathToRestoreUtility, nil
}
