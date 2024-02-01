package main_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dbupdater/config"
	"dbupdater/helper"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var (
	pathToUtility        string
	entryForTestDatabase *config.DbEntry
	connectString        string
)

// It is necessary to have a folder in the folder with the project
// pg_dump_restore_15_2 folder, which should contain the utilities
// pg_dump and pg_restore

// The parameters in the test.env file are required
// DB_HOST
// DB_PORT
// DB_DBNAME
// DB_USER
// DB_PASSWORD

func TestMain(m *testing.M) {
	pathToBuiltUtility, entry, nameTmpTestDatabase := setup()
	pathToUtility = pathToBuiltUtility
	entryForTestDatabase = &config.DbEntry{
		Host:     entry.Host,
		Port:     entry.Port,
		DbName:   nameTmpTestDatabase,
		User:     entry.User,
		Password: entry.Password,
	}
	connectString = fmt.Sprintf("-host %s -port %s -dbname %s -username %s -password %s",
		entry.Host, entry.Port, nameTmpTestDatabase, entry.User, entry.Password)

	m.Run()

	teardown(pathToBuiltUtility, entry, nameTmpTestDatabase)
}

// Returns the path to the built utility whose launch will be tested.
// Returns the connection parameters to the database specified by the user in the test.env file
// Returns the name of the temporary database created for the tests
func setup() (string, *config.DbEntry, string) {
	pathToBuiltUtility, err := buildUtility()
	if err != nil {
		log.Fatal(err)
	}

	entry, err := getEntry()
	if err != nil {
		if err := os.Remove(pathToBuiltUtility); err != nil {
			log.Printf("%v\nDelete the temporary utility binary file yourself: %s", err, pathToBuiltUtility)
		}
		log.Fatal(err)
	}

	ctx := context.Background()
	conn, err := helper.OpenConnect(ctx, entry, false)
	if err != nil {
		if err := os.Remove(pathToBuiltUtility); err != nil {
			log.Printf("%v\nDelete the temporary utility binary file yourself: %s", err, pathToBuiltUtility)
		}
		log.Fatal(err)
	}
	defer conn.Close(ctx)

	nameTmpTestDatabase, err := crateTestDatabase(ctx, conn)
	if err != nil {
		if err := os.Remove(pathToBuiltUtility); err != nil {
			log.Printf("%v\nDelete the temporary utility binary file yourself: %s", err, pathToBuiltUtility)
		}
		conn.Close(ctx)
		log.Fatal(err)
	}

	log.Println("\n-----Setup complete-----")
	return pathToBuiltUtility, entry, nameTmpTestDatabase
}

func teardown(pathToBuiltUtility string, entry *config.DbEntry, nameTmpTestDatabase string) {
	if err := os.Remove(pathToBuiltUtility); err != nil {
		log.Printf("%v\nDelete the temporary utility binary file yourself: %s", err, pathToBuiltUtility)
	}

	ctx := context.Background()
	conn, err := helper.OpenConnect(ctx, entry, false)
	if err != nil {
		log.Fatalf("%s\nDelete the temporary database yourself: %s", err, nameTmpTestDatabase)
	}
	defer conn.Close(ctx)

	sql := "DROP DATABASE IF EXISTS " + nameTmpTestDatabase
	if _, err := conn.Exec(ctx, sql); err != nil {
		conn.Close(ctx)
		log.Fatalf("%v\nDelete the temporary database yourself: %s", err, nameTmpTestDatabase)
	}

	log.Println("\n----Teardown complete----")
}

// Returns the path to the binary file of the utility that is being tested
func buildUtility() (string, error) {
	// It is necessary to have a folder in the folder with the project
	// pg_dump_restore_15_2 folder, which should contain the utilities
	// pg_dump and pg_restore

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("it is impossible to recover information about the location of the project")
	}
	pathToProject := filepath.Dir(filename)

	cmd := exec.Command(`go`, `build`, `-o`, pathToProject+`/test-main.exe`, pathToProject)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}

	return pathToProject + `/test-main.exe`, nil
}

func getEntry() (*config.DbEntry, error) {
	if err := godotenv.Load("test.env"); err != nil {
		return nil, err
	}
	countedEntry := &config.DbEntry{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		DbName:   os.Getenv("DB_DBNAME"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
	}

	return countedEntry, nil
}

// Returns the name of the created database for tests
func crateTestDatabase(ctx context.Context, conn *pgx.Conn) (string, error) {
	t := time.Now()
	milli := t.UnixMilli()
	strMilli := fmt.Sprintf("%d", milli)
	tmpDatabase := "tmp_test_" + strMilli

	sql := "CREATE DATABASE " + tmpDatabase
	if _, err := conn.Exec(ctx, sql); err != nil {
		return "", err
	}

	return tmpDatabase, nil
}

// -------------------------------------------------------TESTS-------------------------------------------------------------------------------------

func TestOpenConnectToDatabase(t *testing.T) {
	t.Parallel()
	t.Run("NoAllParametersForConnect", func(t *testing.T) {
		t.Parallel()
		badConnectString := fmt.Sprintf("-host %s -port %s -dbname %s -username %s",
			entryForTestDatabase.Host, entryForTestDatabase.Port, entryForTestDatabase.DbName, entryForTestDatabase.User)

		output := runUtility(t, badConnectString)
		if !strings.Contains(output, "Not all parameters for connection are specified.") {
			t.Errorf("There is no message that not all parameters for connection are specified")
		}
	})

	t.Run("BadPasswordForConnect", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0003.Clear' as name")
		badConnectString := fmt.Sprintf("-host %s -port %s -dbname %s -username %s -password badPassword -migrations %s",
			entryForTestDatabase.Host, entryForTestDatabase.Port, entryForTestDatabase.DbName, entryForTestDatabase.User, tmpDir)

		output := runUtility(t, badConnectString)
		if !strings.Contains(output, "password authentication failed") {
			t.Errorf("There is no message that the password is incorrect")
		}
	})

	t.Run("GoodConnect", func(t *testing.T) {
		t.Parallel()
		clearTmpDir := t.TempDir()
		output := runUtility(t, connectString+` -migrations `+clearTmpDir)
		if !strings.Contains(output, "GetCurrentVersion.sql: The system cannot find the path specified.") {
			t.Errorf("Failed to join the database with correct connection parameters")
		}
	})
}

func TestGetCurrentMigration(t *testing.T) {
	t.Parallel()
	t.Run("NoPathToMigrations", func(t *testing.T) {
		t.Parallel()
		output := runUtility(t, connectString)
		if !strings.Contains(output, "-migrations") {
			t.Errorf("No recommendation to the user on how to use the parameter -migrations")
		}
	})

	// If the folder with migrations does not contain GetCurrentVersion.sql
	t.Run("GetCurrentVersionFileNoIsExists", func(t *testing.T) {
		t.Parallel()
		clearTmpDir := t.TempDir()
		output := runUtility(t, connectString+` -migrations `+clearTmpDir)
		if !strings.Contains(output, "GetCurrentVersion.sql: The system cannot find the path specified.") {
			t.Errorf("No error if GetCurrentVersion.sql is missing")
		}
	})

	t.Run("ShowCurrentVersion", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0001.Clear' as name")
		output := runUtility(t, connectString+` -migrations `+tmpDir)
		if !strings.Contains(output, "Current database version: v0.0.1") ||
			!strings.Contains(output, "Last migration applied: 0001.Clear") {
			t.Errorf("The current DB version and the last applied migration are not displayed")
		}
	})
}

func TestGetUnappliedSortedMigrations(t *testing.T) {
	t.Parallel()
	t.Run("NoFolderWithCurrentVersionDb", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0001.Clear' as name")
		output := runUtility(t, connectString+` -migrations `+tmpDir)
		if !strings.Contains(output, "v0.0.1: The system cannot find the file specified.") {
			t.Errorf("There is no error about the absence of a folder with the current database version")
		}
	})

	t.Run("CheckMigrationName", func(t *testing.T) {
		t.Parallel()
		t.Run("DifferentDatabaseVersions", func(t *testing.T) {
			t.Parallel()
			t.Run("InCurrentVersion", func(t *testing.T) {
				t.Parallel()
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0003.Clear' as name")
				pathToCurrentVersionDb := tmpDir + `/v0.0.1`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0004.ASDF.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0005.ZXCD.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				isCurrentMigrationAfterUpdates := strings.Count(output, "0003.Clear") > 1
				correctOrder := isCorrectOrder(output, "Updates available:", "0004.ASDF", "0005.ZXCD")
				isExistsOlderVersions := strings.Contains(output, "0001.First") || strings.Contains(output, "0002.Second")

				if isCurrentMigrationAfterUpdates || !correctOrder || isExistsOlderVersions {
					t.Errorf("Incorrect output of migrations available for update")
				}
			})

			t.Run("InAFutureVersion", func(t *testing.T) {
				t.Parallel()
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0003.Clear' as name")
				pathToCurrentVersionDb := tmpDir + `/v0.0.1`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0004.ASDF.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0005.ZXCD.sql`, "")

				createDir(t, tmpDir+`/v0.1.0`)
				createFileAndWrite(t, tmpDir+`/v0.1.0/0001.h10.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.1.0/0002.j10.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				isCurrentMigrationAfterUpdates := strings.Count(output, "0003.Clear") > 1
				correctOrder := isCorrectOrder(output, "Updates available:", "0004.ASDF", "0005.ZXCD", "v0.1.0", "0001.h10", "0002.j10")
				isExistsOlderVersions := strings.Contains(output, "0001.First") || strings.Contains(output, "0002.Second")

				if isCurrentMigrationAfterUpdates || !correctOrder || isExistsOlderVersions {
					t.Errorf("Incorrect output of migrations available for update")
				}
			})

			t.Run("IncorrectOrderInTheCurrentVersion", func(t *testing.T) {
				t.Parallel()
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0003.Clear' as name")
				pathToCurrentVersionDb := tmpDir + `/v0.0.1`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0005.ASDF.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0007.ZXCvsql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				if !strings.Contains(output, `wrong order of migrations, error on v0.0.1 0005.ASDF`) {
					t.Errorf("No error message if the order of migration numbers is violated")
				}
			})

			t.Run("IncorrectOrderInAFutureVersion", func(t *testing.T) {
				t.Parallel()
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0003.Clear' as name")
				pathToCurrentVersionDb := tmpDir + `/v0.0.1`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0004.ASDF.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0005.ZXCvsql`, "")

				createDir(t, tmpDir+`/v0.1.0`)
				createFileAndWrite(t, tmpDir+`/v0.1.0/0001.hhh.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.1.0/0002.jjj.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.1.0/0010.kkk.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				if !strings.Contains(output, `wrong order of migrations, error on v0.1.0 0010.kkk`) {
					t.Errorf("No error message if the order of migration numbers is violated")
				}
			})

			t.Run("IncorrectOrderInPreviousVersions", func(t *testing.T) {
				t.Parallel()
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.5' as version_db, '0003.Clear' as name")
				pathToCurrentVersionDb := tmpDir + `/v0.0.5`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0004.ASDF.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0005.ZXCD.sql`, "")

				createDir(t, tmpDir+`/v0.0.4`)
				createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0010.kkk.sql`, "")

				// The utility does not check the order of past versions of the database
				output := runUtility(t, connectString+` -migrations `+tmpDir)

				correctOrder := isCorrectOrder(output, "Updates available:", "0004.ASDF", "0005.ZXCD")

				if !correctOrder || strings.Contains(output, `v0.0.4`) {
					t.Errorf("Incorrect display of migrations available for update")
				}
			})
		})

		t.Run("CheckingTheOrderOfMigrationNumbers", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0004.ASDF.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0005.ZXCvsql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0004.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			if !strings.Contains(output, `wrong order of migrations, error on v0.0.4 0002.hhh`) {
				t.Errorf("No error message if the order of migration numbers is violated")
			}
		})

		t.Run("InvalidMigrationName", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0001.First' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")

			createFileAndWrite(t, pathToCurrentVersionDb+`/02.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.lash`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.lash.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002..sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear`, "")

			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Good.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			correctOrder := isCorrectOrder(output, "Updates available:", "0002.Good")
			notContainsAll := isNotContainsAll(output, `0002.Clear`, `0002.Clear.lash`, `error`, `wrong order of migrations`)

			if !correctOrder || !notContainsAll {
				t.Errorf("Migrations that have an error in their name should be ignored")
			}
		})

		t.Run("NumberOfDigitsInMigrationName", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0001.First' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/02.Clear.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			if !strings.Contains(output, `No new migrations`) {
				t.Errorf("Migrations that have an error in their name should be ignored")
			}
		})

		t.Run("NumberOfDotsInTheMigrationName", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			if !strings.Contains(output, `No new migrations`) {
				t.Errorf("Migrations that have an error in their name should be ignored")
			}
		})

		t.Run("CheckMigrationFileExtension", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.test.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			if !strings.Contains(output, `No new migrations`) {
				t.Errorf("Migrations that have an error in their name should be ignored")
			}
		})

		t.Run("CheckMigrationFileExtension", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.lash`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			if !strings.Contains(output, `No new migrations`) {
				t.Errorf("Migrations that have an error in their name should be ignored")
			}
		})
	})

	t.Run("GetVersions", func(t *testing.T) {
		t.Parallel()
		t.Run("MigrationSortingCheck", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.1.2' as version_db, '0001.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.1.2`

			createDir(t, pathToCurrentVersionDb)
			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.Clear.sql`, "")

			createDir(t, tmpDir+`/v0.2.5`)
			createFileAndWrite(t, tmpDir+`/v0.2.5/0001.Clear25.sql`, "")

			createDir(t, tmpDir+`/v0.1.11`)
			createFileAndWrite(t, tmpDir+`/v0.1.11/0001.Clear111.sql`, "")

			createDir(t, tmpDir+`/v0.1.9`)
			createFileAndWrite(t, tmpDir+`/v0.1.1/0001.Clear11.sql`, "")

			createDir(t, tmpDir+`/v0.0.52`)
			createFileAndWrite(t, tmpDir+`/v0.0.52/0001.Clear52.sql`, "")

			createDir(t, tmpDir+`/v0.2.15`)
			createFileAndWrite(t, tmpDir+`/v0.2.15/0001.Clear215.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			correctOrder := isCorrectOrder(output, "Updates available:", "v0.1.11", "0001.Clear111", "v0.2.5", "0001.Clear25", "v0.2.15", "0001.Clear215")
			isExistsOlderVersions := strings.Contains(output, "v0.1.9") || strings.Contains(output, "v0.0.52")

			if !correctOrder || isExistsOlderVersions {
				t.Errorf("Wrong order or version list")
			}
		})

		t.Run("InvalidVersionName", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.1.2' as version_db, '0001.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.1.2`

			createDir(t, pathToCurrentVersionDb)
			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.Clear.sql`, "")

			createDir(t, tmpDir+`/vt.0.2.5a`)
			createFileAndWrite(t, tmpDir+`/vt.0.2.5a/0001.Clear.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			// ignored bad version db folder
			if !strings.Contains(output, `No new migrations`) {
				t.Errorf("Must ignore bad version db")
			}
		})
	})

	t.Run("NoNewVersionsDb", func(t *testing.T) {
		t.Parallel()
		t.Run("NoCurrentMigrationInVersion", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			pathToCurrentVersionDb := tmpDir + `/v0.0.1`
			createDir(t, pathToCurrentVersionDb)

			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT '0001.Clear' as name, 'v0.0.1' as version_db")

			t.Run("MigrationsInVersionNoExists", func(t *testing.T) {
				output := runUtility(t, connectString+` -migrations `+tmpDir)

				if !strings.Contains(output, "the v0.0.1 0001.Clear migration is not in the migrations directory") {
					t.Errorf("There is no error about what is not in the folder of the last applied migration")
				}
			})

			t.Run("ThereAreOtherMigrations", func(t *testing.T) {
				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.FirstMigration.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.SecondMigration.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.ThirdMigration.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				if !strings.Contains(output, "the v0.0.1 0001.Clear migration is not in the migrations directory") {
					t.Errorf("There is no error about what is not in the folder of the last applied migration")
				}
			})
		})

		t.Run("CurrentMigrationIsExists", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			pathToCurrentVersionDb := tmpDir + `/v0.0.1`
			createDir(t, pathToCurrentVersionDb)

			t.Run("CurrentMigrationIsOnlyOne", func(t *testing.T) {
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0001.Clear' as name")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.Clear.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				if !strings.Contains(output, "No new migrations") {
					t.Errorf("There should be a message that there are no new migrations")
				}
			})
			t.Run("CurrentMigrationIsFirst", func(t *testing.T) {
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.SecondMigration.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.ThirdMigration.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				isCurrentMigrationAfterUpdates := strings.Count(output, "0001.Clear") > 1
				correctOrder := isCorrectOrder(output, "Updates available:", "0002.SecondMigration", "0003.ThirdMigration")

				if isCurrentMigrationAfterUpdates || !correctOrder {
					t.Errorf("Incorrect definition of migrations available for updating")
				}
			})

			t.Run("CurrentMigrationInTheMiddle", func(t *testing.T) {
				removeAll(t, tmpDir+`/utils/GetCurrentVersion.sql`)
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0002.SecondMigration' as name")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				isCurrentMigrationAfterUpdates := strings.Count(output, "0002.SecondMigration") > 1
				correctOrder := isCorrectOrder(output, "Updates available:", "0003.ThirdMigration")

				if !correctOrder || strings.Contains(output, "0001.Clear") || isCurrentMigrationAfterUpdates {
					t.Errorf("Incorrect definition of migrations available for updating")
				}
			})

			t.Run("CurrentMigrationIsLatest", func(t *testing.T) {
				removeAll(t, tmpDir+`/utils/GetCurrentVersion.sql`)
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.1' as version_db, '0003.ThirdMigration' as name")

				output := runUtility(t, connectString+` -migrations `+tmpDir)

				isCurrentMigrationAfterUpdates := strings.Count(output, "0003.ThirdMigration") > 1
				notContainsAll := isNotContainsAll(output, "v0.0.1 0001.Clear", "0002.SecondMigration")

				if !strings.Contains(output, "No new migrations") || isCurrentMigrationAfterUpdates || !notContainsAll {
					t.Errorf("Incorrect definition of migrations available for updating")
				}
			})
		})
	})

	t.Run("NewVersionsIsExists", func(t *testing.T) {
		t.Parallel()
		t.Run("NewVersionsAreEmpty", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.1.2' as version_db, '0001.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.1.2`

			createDir(t, pathToCurrentVersionDb)
			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.Clear.sql`, "")

			createDir(t, tmpDir+`/v0.2.5`)
			createDir(t, tmpDir+`/v0.1.0`)

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			isCurrentMigrationAfterUpdates := strings.Count(output, "0001.Clear") > 1
			notContainsAll := isNotContainsAll(output, `v0.2.5`, `v0.1.0`)

			if !strings.Contains(output, `No new migrations`) || isCurrentMigrationAfterUpdates || !notContainsAll {
				t.Errorf("There should be a message that there are no new migrations")
			}
		})

		t.Run("CurrentMigrationLast", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.1.2' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.1.2`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.FirstMigration.sql`, "")

			createDir(t, tmpDir+`/v0.2.5`)
			createFileAndWrite(t, tmpDir+`/v0.2.5/0001.First25.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.2.5/0002.Second25.sql`, "")

			createDir(t, tmpDir+`/v0.2.6`)
			createFileAndWrite(t, tmpDir+`/v0.2.6/0001.First26.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.2.6/0002.Second26.sql`, "")

			createDir(t, tmpDir+`/v0.1.1`)
			createFileAndWrite(t, tmpDir+`/v0.1.1/0001.First11.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.1.1/0002.Second11.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			isCurrentMigrationAfterUpdates := strings.Count(output, "0002.Clear") > 1
			isCurrentVersionAfterUpdates := strings.Count(output, "v0.1.2") > 1
			correctOrder := isCorrectOrder(output, "Updates available:", "v0.2.5", "0001.First25", "0002.Second25", `v0.2.6`, `0001.First26`, `0002.Second26`)

			if !correctOrder || strings.Contains(output, `v0.1.1`) || isCurrentMigrationAfterUpdates || isCurrentVersionAfterUpdates {
				t.Errorf("Migrations older than the current one may be displayed")
			}
		})

		t.Run("CurrentMigrationInMiddle", func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.1.2' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.1.2`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.FirstMigration.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.ThirdCurrent.sql`, "")

			createDir(t, tmpDir+`/v0.2.5`)
			createFileAndWrite(t, tmpDir+`/v0.2.5/0001.First25.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.2.5/0002.Second25.sql`, "")

			createDir(t, tmpDir+`/v0.2.6`)
			createFileAndWrite(t, tmpDir+`/v0.2.6/0001.First26.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.2.6/0002.Second26.sql`, "")

			createDir(t, tmpDir+`/v0.1.0`)
			createFileAndWrite(t, tmpDir+`/v0.1.0/0001.First10.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.1.0/0002.Second10.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			isCurrentMigrationAfterUpdates := strings.Count(output, "0002.Clear") > 1
			correctOrder := isCorrectOrder(output, "Updates available:", "0003.ThirdCurrent", "v0.2.5", "0001.First25",
				`0002.Second25`, `v0.2.6`, `0001.First26`, `0002.Second26`)
			notContainsAll := isNotContainsAll(output, "0001.FirstMigration", "v0.1.0", `0001.First10`, `0002.Second10`)

			if !correctOrder || !notContainsAll || isCurrentMigrationAfterUpdates {
				t.Errorf("Incorrect migrations available for updating")
			}
		})
	})
}

func TestGetMigrationsToMigrate(t *testing.T) {
	t.Run("NoVersiondbAndNameMigration", func(t *testing.T) {
		tmpDir := t.TempDir()
		createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
		pathToCurrentVersionDb := tmpDir + `/v0.0.3`

		createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
		createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
		createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

		output := runUtility(t, connectString+` -migrations `+tmpDir)

		correctOrder := isCorrectOrder(output, `Current database version: v0.0.3`, `Last migration applied: 0002.Clear`,
			"Updates available:", "0003.Hqwerty")

		if !correctOrder || strings.Contains(output, `Migrations started to apply`) {
			t.Errorf("Migrations should not be applied")
		}
	})

	t.Run("SpecifiedOnlyVersiondb", func(t *testing.T) {
		t.Run("NoFolderWithThisVersion", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.4`)

			if !strings.Contains(output, `The v0.0.4 version is not in the list of migrations available for updating`) {
				t.Errorf("There should be an error message 'The v0.0.4 version is not in the list of migrations available for updating'")
			}
		})

		t.Run("FolderEmpty ", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			v4 := tmpDir + `/v0.0.4`
			createDir(t, v4)

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.4`)

			if !strings.Contains(output, `The v0.0.4 version is not in the list of migrations available for updating`) {
				t.Errorf("There is no message 'The v0.0.4 version is not in the list of migrations available for updating'")
			}
		})

		t.Run("BadVersiondb", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.4.bad`)

			if !strings.Contains(output, `wrong version in -versiondb: Malformed version: v0.0.4.bad`) {
				t.Errorf("There should be an error message in -versiondb")
			}
		})

		t.Run("VersionLessThanCurrent", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.2`)
			createFileAndWrite(t, tmpDir+`/v0.0.2/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.2/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.2/0003.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.2`)

			if !strings.Contains(output, `The v0.0.2 version is not in the list of migrations available for updating`) {
				t.Errorf("There should be a message 'The v0.0.2 version is not in the list of migrations available for updating'")
			}
		})

		t.Run("VersionIsMoreCurrent", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "SELECT $1, $2")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

			createDir(t, tmpDir+`/v0.0.5`)
			createFileAndWrite(t, tmpDir+`/v0.0.5/0001.fone.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.5/0002.ftwo.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.5/0003.fthree.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.5 -verbose`)

			if !strings.Contains(output, `Applied: v0.0.5 0003.fthree`) || !strings.Contains(output, `Migrations have been applied.`) {
				t.Errorf("Migrations should apply")
			}
		})

		t.Run("VersionIsEqualToCurrent", func(t *testing.T) {
			t.Run("CurrentMigrationIsFirst", func(t *testing.T) {
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0001.First' as name")
				createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "SELECT $1, $2")
				pathToCurrentVersionDb := tmpDir + `/v0.0.3`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

				createDir(t, tmpDir+`/v0.0.4`)
				createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.3 -verbose`)

				correctOrder := isCorrectOrder(output, `Applied: v0.0.3 0002.Clear`,
					"Applied: v0.0.3 0003.Hqwerty", `Migrations have been applied.`)
				notContainsAll := isNotContainsAll(output, "Applied: v0.0.4 0001.hhh", "Applied: v0.0.4 0003.kkk")

				if !correctOrder || !notContainsAll {
					t.Errorf("Incorrect list of applied migrations")
				}
			})

			t.Run("CurrentMigrationInMiddle", func(t *testing.T) {
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
				createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "SELECT $1, $2")
				pathToCurrentVersionDb := tmpDir + `/v0.0.3`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

				createDir(t, tmpDir+`/v0.0.4`)
				createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.3 -verbose`)

				correctOrder := isCorrectOrder(output, `Applied: v0.0.3 0003.Hqwerty`, `Migrations have been applied.`)
				notContainsAll := isNotContainsAll(output, "Applied: v0.0.3 0002.Clear", "Applied: v0.0.4 0001.hhh",
					`Applied: v0.0.4 0003.kkk`)
				if !correctOrder || !notContainsAll {
					t.Errorf("Incorrect list of applied migrations")
				}
			})

			t.Run("CurrentMigrationLatest", func(t *testing.T) {
				tmpDir := t.TempDir()
				createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Hqwerty' as name")
				pathToCurrentVersionDb := tmpDir + `/v0.0.3`

				createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
				createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

				createDir(t, tmpDir+`/v0.0.4`)
				createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
				createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

				output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.3 -verbose`)

				notContainsAll := isNotContainsAll(output, "Migrations have been applied.", "Applied: v0.0.3 0003.Hqwerty",
					`Applied: v0.0.4 0001.hhh`, `Applied: v0.0.4 0003.kkk`, `Applied: v0.0.3 0002.Clear`)
				if !strings.Contains(output, `The v0.0.3 version is not in the list of migrations available for updating`) ||
					!notContainsAll {
					t.Errorf("no message 'The v0.0.3 version is not in the list of migrations available for updating'")
				}
			})
		})
	})

	t.Run("SpecifiedOnlyNameMigration", func(t *testing.T) {
		t.Run("SheIsNotInCurrentVersion", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Hqwerty' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -migration 0001.hhh -verbose`)

			if strings.Contains(output, `Migrations started to apply`) ||
				!strings.Contains(output, `The v0.0.3 0001.hhh migration is not in the list of migrations available for updating`) {
				t.Errorf("There should be a message: The v0.0.3 0001.hhh migration is not in the list of migrations available for updating")
			}
		})

		t.Run("ItIsLessThanCurrentMigration", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Hqwerty' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -migration 0002.Clear -verbose`)

			if strings.Contains(output, `Migrations started to apply`) ||
				!strings.Contains(output, `The v0.0.3 0002.Clear migration is not in the list of migrations available for updating`) {
				t.Errorf("There should be a message: The v0.0.3 0002.Clear migration is not in the list of migrations available for updating")
			}
		})

		t.Run("SheIsEqualCurrentMigration", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Hqwerty' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -migration 0003.Hqwerty -verbose`)

			if strings.Contains(output, `Migrations started to apply`) ||
				!strings.Contains(output, `The v0.0.3 0003.Hqwerty migration is not in the list of migrations available for updating`) {
				t.Errorf("There should be a message: The v0.0.3 0003.Hqwerty migration is not in the list of migrations available for updating")
			}
		})

		t.Run("SheIsMoreInCurrentVersion", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0002.Clear' as name")
			createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "SELECT $1, $2")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -migration 0003.Hqwerty -verbose`)

			correctOrder := isCorrectOrder(output, `Applied: v0.0.3 0003.Hqwerty`, `Migrations have been applied.`)
			notContainsAll := isNotContainsAll(output, "Applied: v0.0.3 0002.Clear", "Applied: v0.0.4 0001.hhh",
				`Applied: v0.0.4 0003.kkk`)

			if !correctOrder || !notContainsAll {
				t.Errorf("Must be applied in the correct order")
			}
		})
	})

	t.Run("VersiondbAndNameMigrationAreSpecified", func(t *testing.T) {
		t.Run("IsNoMigrationInVersiondb", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Hqwerty' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -migration 0003.Hqwerty -versiondb v0.0.4 -verbose`)

			if strings.Contains(output, `Migrations started to apply`) ||
				!strings.Contains(output, `The v0.0.4 0003.Hqwerty migration is not in the list of migrations available for updating`) {
				t.Errorf("There should be a message that 'migration is not in the list of migrations available for updating'")
			}
		})

		t.Run("IsNoVersiondb", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Hqwerty' as name")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -migration 0003.Hqwerty -versiondb v0.0.5 -verbose`)

			if strings.Contains(output, `Migrations started to apply`) ||
				!strings.Contains(output, `The v0.0.5 0003.Hqwerty migration is not in the list of migrations available for updating`) {
				t.Errorf("There should be a message that there 'migration is not in the list of migrations available for updating' ")
			}
		})

		t.Run("ThereIsMigrationAndVersion", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0003.Hqwerty' as name")
			createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "SELECT $1, $2")
			pathToCurrentVersionDb := tmpDir + `/v0.0.3`

			createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")
			createFileAndWrite(t, pathToCurrentVersionDb+`/0003.Hqwerty.sql`, "")

			createDir(t, tmpDir+`/v0.0.4`)
			createFileAndWrite(t, tmpDir+`/v0.0.4/0001.hhh.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0002.jjj.sql`, "")
			createFileAndWrite(t, tmpDir+`/v0.0.4/0003.kkk.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -migration 0002.jjj -versiondb v0.0.4 -verbose`)

			correctOrder := isCorrectOrder(output, `Applied: v0.0.4 0001.hhh`, `Applied: v0.0.4 0002.jjj`, `Migrations have been applied.`)
			notContainsAll := isNotContainsAll(output, "Applied: v0.0.4 0003.kkk", "Applied: v0.0.3 0003.Hqwerty")

			if !correctOrder || !notContainsAll {
				t.Errorf("Wrong migrations applied or wrong order")
			}
		})
	})
}

func TestCreateDump(t *testing.T) {
	t.Run("CheckingForDumpCreatedInTheOutput", func(t *testing.T) {
		tmpDir := t.TempDir()
		createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 'v0.0.3' as version_db, '0001.First' as name")
		createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "SELECT $1, $2")
		pathToCurrentVersionDb := tmpDir + `/v0.0.3`

		createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
		createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Clear.sql`, "")

		output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.3 -verbose`)

		if !strings.Contains(output, `Dump is created`) || !strings.Contains(output, `Dump created`) {
			t.Errorf("There should be a message about successful dump creation in -verbose mode")
		}
	})
}

func TestUpdateTheLastAppliedMigration(t *testing.T) {
	ctx := context.Background()
	conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
	if err != nil {
		t.Fatalf("Error when establishing a connection to the database: %v", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "create table lastMigration ( version_db varchar, name varchar ); "+
		"insert into lastMigration values ('v0.0.5', '0003.InsertInitData');"); err != nil {
		t.Fatalf("Error when creating the table lastMigration: %v", err)
	}

	conn.Close(ctx)

	defer func() {
		conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		if _, err := conn.Exec(ctx, "DROP TABLE lastMigration"); err != nil {
			t.Fatalf("Error when deleting a test table: %v", err)
		}
	}()

	tmpDir := t.TempDir()
	createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT version_db, name FROM lastMigration")

	pathToCurrentVersionDb := tmpDir + `/v0.0.5`
	createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
	createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
	createFileAndWrite(t, pathToCurrentVersionDb+`/0003.InsertInitData.sql`, "")

	t.Run("CheckFileUpdateCurrentVersion", func(t *testing.T) {
		createFileAndWrite(t, pathToCurrentVersionDb+`/0004.Test.sql`, "")

		t.Run("NoFile", func(t *testing.T) {
			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.5`)

			correctOrder := isCorrectOrder(output, `utils/UpdateCurrentVersion.sql: The system cannot find the file specified.`)
			if !correctOrder {
				t.Errorf("There should be a message that the utils/UpdateCurrentVersion.sql file is missing")
			}
		})

		t.Run("EmptyFile", func(t *testing.T) {
			createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.5 -verbose`)

			if !strings.Contains(output, `must have arguments $1 and $2`) {
				t.Errorf("No message about missing $1 and $2 in utils/UpdateCurrentVersion.sql")
			}
		})

		removeAll(t, pathToCurrentVersionDb+`/0004.Test.sql`)
	})

	createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "UPDATE lastMigration SET version_db=$1, name=$2;")

	t.Run("CorrectUpdate", func(t *testing.T) {
		conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		createDir(t, tmpDir+`/v0.0.7`)
		createFileAndWrite(t, tmpDir+`/v0.0.7/0001.onee.sql`, "")
		createFileAndWrite(t, tmpDir+`/v0.0.7/0002.twoo.sql`, "")

		output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.7 -verbose`)

		var newVersionDb, newLastMigration string
		if err := conn.QueryRow(ctx, "SELECT * FROM lastMigration").Scan(&newVersionDb, &newLastMigration); err != nil {
			t.Fatalf("Error when QueryRow: %v", err)
		}

		if newVersionDb != "v0.0.7" || newLastMigration != "0002.twoo" {
			t.Errorf("Incorrect updated information about new version and last applied migration")
		}

		if !strings.Contains(output, `Migrations have been applied.`) || !strings.Contains(output, `Dump deleted.`) {
			t.Errorf("No messages about successful application of migrations and dump deletion")
		}

		if _, err := conn.Exec(ctx, "UPDATE lastMigration SET version_db='v0.0.5', name='0003.InsertInitData';"); err != nil {
			t.Fatalf("Error when creating the table lastMigration: %v", err)
		}
		removeAll(t, tmpDir+`/v0.0.7`)
	})

	t.Run("ErrorInTheMigration", func(t *testing.T) {
		createDir(t, tmpDir+`/v0.0.7`)
		createFileAndWrite(t, tmpDir+`/v0.0.7/0001.wrong.sql`, "create table lastMigration ( version_db varchar, name varchar );")
		createFileAndWrite(t, tmpDir+`/v0.0.7/0002.twoo.sql`, "")

		output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.7 -verbose`)
		conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		var newVersionDb, newLastMigration string
		if err := conn.QueryRow(ctx, "SELECT * FROM lastMigration").Scan(&newVersionDb, &newLastMigration); err != nil {
			t.Fatalf("Error when QueryRow: %v", err)
		}

		if newVersionDb != "v0.0.5" || newLastMigration != "0003.InsertInitData" {
			t.Errorf("Version DB and latest migration should remain")
		}

		correctOrder := isCorrectOrder(output, `Error applying migration:`, `relation "lastmigration" already exists`,
			`The database from the dump has been restored.`, `Dump deleted`)

		if !correctOrder || strings.Contains(output, `There is 1 other session using the database.`) {
			t.Errorf("No messages about successful application of migrations and dump deletion")
		}

		removeAll(t, tmpDir+`/v0.0.7`)
	})
}

func TestMigrate(t *testing.T) {
	ctx := context.Background()
	conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
	if err != nil {
		t.Fatalf("Error when establishing a connection to the database: %v", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "create table lastMigration ( version_db varchar, name varchar ); "+
		"insert into lastMigration values ('v0.0.5', '0003.InsertInitData');"); err != nil {
		t.Fatalf("Error when creating the table lastMigration: %v", err)
	}

	conn.Close(ctx)

	defer func() {
		conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		if _, err := conn.Exec(ctx, "DROP TABLE lastMigration"); err != nil {
			t.Fatalf("Error when deleting a table lastMigration: %v", err)
		}
	}()

	tmpDir := t.TempDir()
	createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT version_db, name FROM lastMigration")
	createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "UPDATE lastMigration SET version_db=$1, name=$2;")
	pathToCurrentVersionDb := tmpDir + `/v0.0.5`

	createFileAndWrite(t, pathToCurrentVersionDb+`/0001.First.sql`, "")
	createFileAndWrite(t, pathToCurrentVersionDb+`/0002.Second.sql`, "")
	createFileAndWrite(t, pathToCurrentVersionDb+`/0003.InsertInitData.sql`, "")

	t.Run("CorrectMigrate", func(t *testing.T) {
		conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		createDir(t, tmpDir+`/v0.0.7`)
		createFileAndWrite(t, tmpDir+`/v0.0.7/0001.First.sql`, "")
		createFileAndWrite(t, tmpDir+`/v0.0.7/0002.Second.sql`,
			"create table testTable ( test1 varchar, test2 varchar );")
		createFileAndWrite(t, tmpDir+`/v0.0.7/0003.Insert.sql`,
			"insert into testTable values ('test string one', 'test string two');")

		output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.7 -verbose`)

		var str1, str2 string
		if err := conn.QueryRow(ctx, "SELECT * FROM testTable").Scan(&str1, &str2); err != nil {
			t.Fatalf("Error when QueryRow: %v", err)
		}

		if str1 != "test string one" || str2 != "test string two" {
			t.Errorf("Migrations were not applied properly")
		}

		if !strings.Contains(output, `Migrations have been applied.`) ||
			!strings.Contains(output, `Dump deleted.`) {
			t.Errorf("No messages about successful application of migrations and dump deletion")
		}

		if _, err := conn.Exec(ctx, "DROP TABLE testTable; UPDATE lastMigration SET version_db='v0.0.5', name='0003.InsertInitData';"); err != nil {
			t.Fatalf("Error when deleting a test table: %v", err)
		}
		removeAll(t, tmpDir+`/v0.0.7`)
	})

	t.Run("SecondCorrectMigrate", func(t *testing.T) {
		conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		if _, err := conn.Exec(ctx, "create table testTable ( test1 varchar, test2 varchar ); insert into testTable values ('old str1', 'old str2');"); err != nil {
			t.Fatalf("Error when deleting a test table: %v", err)
		}

		createDir(t, tmpDir+`/v0.0.7`)
		createFileAndWrite(t, tmpDir+`/v0.0.7/0001.First.sql`, "UPDATE testTable SET test1='new str1', test2='new str2'")

		output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.7 -verbose`)

		var str1, str2 string
		if err := conn.QueryRow(ctx, "SELECT * FROM testTable").Scan(&str1, &str2); err != nil {
			t.Fatalf("Error when QueryRow: %v", err)
		}

		if str1 != "new str1" || str2 != "new str2" {
			t.Errorf("The applied migration did not apply")
		}

		if !strings.Contains(output, `Applied: v0.0.7 0001.First`) || !strings.Contains(output, `Migrations have been applied.`) {
			t.Errorf("No successful application of migrations reported")
		}

		if _, err := conn.Exec(ctx, "DROP TABLE testTable; UPDATE lastMigration SET version_db='v0.0.5', name='0003.InsertInitData';"); err != nil {
			t.Fatalf("Error when deleting a test table: %v", err)
		}
		removeAll(t, tmpDir+`/v0.0.7`)
	})

	t.Run("ErrorInMigration", func(t *testing.T) {
		conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		if _, err := conn.Exec(ctx, "create table testTable ( test1 varchar, test2 varchar ); insert into testTable values ('old str1', 'old str2');"); err != nil {
			t.Fatalf("Error when deleting a test table: %v", err)
		}

		createDir(t, tmpDir+`/v0.0.7`)
		createFileAndWrite(t, tmpDir+`/v0.0.7/0001.First.sql`, "UPDATE testTable SET test1='new str1', test2='new str2'")
		createFileAndWrite(t, tmpDir+`/v0.0.7/0002.Wrong.sql`, "create table testTable ( test1 varchar, test2 varchar )")

		conn.Close(ctx)
		output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb v0.0.7 -verbose`)

		conn, err = helper.OpenConnect(ctx, entryForTestDatabase, false)
		if err != nil {
			t.Fatalf("Error when establishing a connection to the database: %v", err)
		}
		defer conn.Close(ctx)

		var str1, str2 string
		if err := conn.QueryRow(ctx, "SELECT * FROM testTable").Scan(&str1, &str2); err != nil {
			t.Fatalf("Error when QueryRow: %v", err)
		}

		if str1 != "old str1" || str2 != "old str2" {
			t.Errorf("The database should have rolled back ")
		}

		correctOrder := isCorrectOrder(output, `Applied: v0.0.7 0001.First`, `Applied: v0.0.7 0002.Wrong`,
			`relation "testtable" already exists`, `The database from the dump has been restored.`)
		if !correctOrder {
			t.Errorf("No error message on migration")
		}

		if _, err := conn.Exec(ctx, "DROP TABLE testTable"); err != nil {
			t.Fatalf("Error when deleting a test table: %v", err)
		}
		removeAll(t, tmpDir+`/v0.0.7`)
	})
}

func TestInitMod(t *testing.T) {
	// For the initialization mod you need
	// 1. To have a file utils/HasCurrentVersion.sql
	// 2. That this file returned false, when executing a sql query. Or executed with an error.
	// 3. To have at least one migration file in the v0.0.0 folder.

	t.Run("NotExistsHasCurrentVersionFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 123")

		output := runUtility(t, connectString+` -migrations `+tmpDir)

		if !strings.Contains(output, "cannot find field version_db in returned row") {
			t.Errorf("there has to be a message: 'cannot find field version_db in returned row'")
		}
	})

	t.Run("HasCurrentVersionReturnTrue", func(t *testing.T) {
		t.Run("Correct default process", func(t *testing.T) {
			// the initialization mode will not be activated
			ctx := context.Background()
			conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
			if err != nil {
				t.Fatalf("Error when establishing a connection to the database: %v", err)
			}
			defer conn.Close(ctx)

			if _, err := conn.Exec(ctx, "create table lastMigration ( version_db varchar, name varchar );"+
				"insert into lastMigration values ('v0.0.1', '0002.InsetInitData');"); err != nil {
				t.Fatalf("Error when creation a lastMigration table: %v", err)
			}

			defer func() {
				if _, err := conn.Exec(ctx, "DROP TABLE lastMigration;"); err != nil {
					t.Fatalf("Error when deleting a lastMigration table: %v", err)
				}
			}()

			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT version_db, name FROM lastMigration")
			createFileAndWrite(t, tmpDir+`/utils/HasCurrentVersion.sql`, "SELECT COUNT(*) <> 0 FROM lastMigration")
			createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "UPDATE lastMigration SET version_db=$1, name=$2;")

			v0 := "v0.0.0"
			createDir(t, tmpDir+"/"+v0)
			createFileAndWrite(t, tmpDir+`/`+v0+`/0001.ZeroVersion.sql`, "")

			v3 := "v0.0.1"
			createDir(t, tmpDir+"/"+v3)
			createFileAndWrite(t, tmpDir+`/`+v3+`/0001.Clear.sql`, "")
			createFileAndWrite(t, tmpDir+`/`+v3+`/0002.InsetInitData.sql`, "")
			createFileAndWrite(t, tmpDir+`/`+v3+`/0003.New.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			correctOrder := isCorrectOrder(output, "Current database version: v0.0.1",
				"Last migration applied: 0002.InsetInitData", "0003.New")

			if !correctOrder || strings.Contains(output, "Initialization mode - ON") {
				t.Errorf("the initialization mode should not be activated")
			}
		})

		t.Run("Bad process", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/GetCurrentVersion.sql`, "SELECT 123")
			createFileAndWrite(t, tmpDir+`/utils/HasCurrentVersion.sql`, "SELECT true")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			if !strings.Contains(output, "cannot find field version_db in returned row") {
				t.Errorf("there has to be a message: 'cannot find field version_db in returned row'")
			}
		})
	})

	t.Run("HasCurrentVersionReturnFalse", func(t *testing.T) {
		t.Run("Correct initializing process", func(t *testing.T) {
			ctx := context.Background()
			conn, err := helper.OpenConnect(ctx, entryForTestDatabase, false)
			if err != nil {
				t.Fatalf("Error when establishing a connection to the database: %v", err)
			}
			defer conn.Close(ctx)
			defer func() {
				if _, err := conn.Exec(ctx, "DROP TABLE lastMigration;"); err != nil {
					t.Fatalf("Error when deleting a lastMigration table: %v", err)
				}
			}()

			tmpDir := t.TempDir()
			// will return false, because there is no table lastMigration (error = false)
			createFileAndWrite(t, tmpDir+`/utils/HasCurrentVersion.sql`, "SELECT COUNT(*) <> 0 FROM lastMigration")
			createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "UPDATE lastMigration SET version_db=$1, name=$2;")

			v0 := "v0.0.0"
			createDir(t, tmpDir+"/"+v0)
			createFileAndWrite(t, tmpDir+`/`+v0+`/0001.CreateLastMigrationTable.sql`, "create table lastMigration ( version_db varchar, name varchar );")
			createFileAndWrite(t, tmpDir+`/`+v0+`/0002.InsetInitData.sql`, "insert into lastMigration values "+
				"('v0.0.0', '0002.InsetInitData');")

			v3 := "v0.0.3"
			createDir(t, tmpDir+"/"+v3)
			createFileAndWrite(t, tmpDir+`/`+v3+`/0001.Clear.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb `+v3)

			correctOrder := isCorrectOrder(output, "Initialization mode - ON",
				"Updates available:", "0001.CreateLastMigrationTable", "0002.InsetInitData", "0001.Clear", "Migrations have been applied.")

			// Migrations must begin to be applied
			if !correctOrder {
				t.Errorf(`The initialization mode must be running and there must be no errors`)
			}
		})
		t.Run("there is no folder with v0.0.0", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/HasCurrentVersion.sql`, "SELECT false")

			v4 := "v0.0.4"
			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb `+v4)

			correctOrder := isCorrectOrder(output, "Initialization mode - ON",
				"Migrations will be applied starting from version v0.0.0",
				"Current database version: v0.0.0",
				"Last migration applied: 0000.InitMod",
				"v0.0.0: The system cannot find the file specified.")

			if !correctOrder {
				t.Errorf("There should be a message that there is no v0.0.0 folder.")
			}
		})

		t.Run("v0.0.0 has no migrations", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/HasCurrentVersion.sql`, "SELECT false")

			createDir(t, tmpDir+"/v0.0.0")

			v4 := "v0.0.4"
			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb `+v4)

			correctOrder := isCorrectOrder(output, "Initialization mode - ON",
				"Migrations will be applied starting from version v0.0.0",
				"Current database version: v0.0.0",
				"Last migration applied: 0000.InitMod",
				"in initialization mode there should be migrations with version v0.0.0")

			if !correctOrder {
				t.Errorf("there should be a message: in initialization mode there should be migrations with version v0.0.0")
			}
		})

		t.Run("Without -versiondb", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/HasCurrentVersion.sql`, "SELECT false")

			v0 := "v0.0.0"
			createDir(t, tmpDir+"/"+v0)
			createFileAndWrite(t, tmpDir+`/`+v0+`/0001.FirstBadMigration.sql`, "")

			output := runUtility(t, connectString+` -migrations `+tmpDir)

			correctOrder := isCorrectOrder(output, "Initialization mode - ON",
				"Current database version: v0.0.0",
				"Last migration applied: 0000.InitMod",
				"Updates available:", "0001.FirstBadMigration",
				"WARNING. If you specify some version in -versiondb, migrations will be applied starting from v0.0.0 version.")

			if !correctOrder {
				t.Errorf("There has to be a message: 'WARNING. If you specify some version in -versiondb, migrations will be applied starting from v0.0.0 version.'")
			}
		})

		t.Run("Bad migration in v0.0.0", func(t *testing.T) {
			tmpDir := t.TempDir()
			createFileAndWrite(t, tmpDir+`/utils/HasCurrentVersion.sql`, "SELECT false")
			createFileAndWrite(t, tmpDir+`/utils/UpdateCurrentVersion.sql`, "$1 and $2")

			v0 := "v0.0.0"
			createDir(t, tmpDir+"/"+v0)
			createFileAndWrite(t, tmpDir+`/`+v0+`/0001.FirstBadMigration.sql`, "DELETE 12312")

			output := runUtility(t, connectString+` -migrations `+tmpDir+` -versiondb `+v0)

			fmt.Println(output)

			correctOrder := isCorrectOrder(output, "Initialization mode - ON",
				"Updates available:", "0001.FirstBadMigration", "Migrations started to apply...",
				"Error applying migration:", `ERROR: syntax error at or near "12312" (SQLSTATE 42601)`,
				"The database from the dump has been restored.")

			// Migrations must begin to be applied
			if !correctOrder {
				t.Errorf(`there should be an error message: "ERROR: syntax error at or near "12312" (SQLSTATE 42601)"`)
			}
		})
	})
}

// ---------------------------------------------------------------FUNCTIONS-------------------------------------------------------------------------------------

// Starts the utility with the passed parameters
func runUtility(t *testing.T, parameters string) string {
	t.Helper()
	params := strings.Split(parameters, " ")
	cmd := exec.Command(pathToUtility, params...)
	output, _ := cmd.CombinedOutput()

	return string(output)
}

func createFileAndWrite(t *testing.T, path string, content string) {
	t.Helper()
	createDir(t, filepath.Dir(path))
	file, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("File creation error: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("Error writing to file: %v", err)
	}
}

func createDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o600); err != nil {
		t.Fatalf("Error when creating directories for a file: %v", err)
	}
}

func removeAll(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("Deletion error: %v", err)
	}
}

// Checks that all strings are in the string and that they follow each other
func isCorrectOrder(input string, args ...string) bool {
	lastIndex := -1
	for _, str := range args {
		newIndex := strings.Index(input, str)
		if newIndex == -1 || newIndex <= lastIndex {
			return false
		}
		lastIndex = newIndex
	}
	return true
}

func isNotContainsAll(input string, args ...string) bool {
	for _, str := range args {
		isContains := strings.Contains(input, str)
		if isContains {
			return false
		}
	}
	return true
}
