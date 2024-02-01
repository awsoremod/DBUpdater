package config

import "flag"

type (
	// Config -.
	Config struct {
		App
		Parameters
		DbEntry
	}

	// App -.
	App struct {
		Version string
	}

	// Parameters -.
	Parameters struct {
		IsVersion bool
		IsVerbose bool

		PathToMigrations string

		StringVersionDb     string
		StringNameMigration string
	}

	// DbEntry -.
	DbEntry struct {
		Host     string
		Port     string
		DbName   string
		User     string
		Password string
	}
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	const versionApp = "1.0.0"
	configApp := &App{
		Version: versionApp,
	}

	configParameters, configDbEntry := parseCmdParameters()

	cfg := &Config{
		App:        *configApp,
		Parameters: *configParameters,
		DbEntry:    *configDbEntry,
	}

	return cfg, nil
}

func parseCmdParameters() (*Parameters, *DbEntry) {
	isVersion := flag.Bool("version", false, "Print the dbupdater version and exit.")
	isVerbose := flag.Bool("verbose", false, "Specifies verbose mode. This will cause dbupdater to output "+
		"detailed object comments and information about creating/deleting the dump file, and progress messages to standard out.")

	host := flag.String("host", "", "Specifies the host name of the machine on which the server is running.")
	port := flag.String("port", "", "Specifies the port on which the server is listening for connections.")
	dbname := flag.String("dbname", "", "Specifies the name of the database to which migrations should be applied. "+
		"If errors occur when applying migrations, there should be no active connections to the database when restoring the database.")
	user := flag.String("username", "", "User name to connect as. The user must have permission to connect to the database "+
		"specified by -dbname. If errors occur when applying migrations, the user must have the right to restore the database:\n"+
		"1. Connecting to the database 'postgres'\n"+
		"2. Deleting the database that is specified in -dbname\n"+
		"3. Must be a member of the database owner role that is specified in -dbname")
	password := flag.String("password", "", "Password to connect to the database.")

	pathToMigrations := flag.String("migrations", "", "Path to the directory with migration scripts")

	stringVersionDb := flag.String("versiondb", "", "To upgrade the database to the specified version. "+
		"Or to specify the database version when applied with the -migration parameter.")

	stringNameMigration := flag.String("migration", "", "Migration file name without extension. To update the database to "+
		"the specified migration within the database version that is specified in the -versiondb parameter. "+
		"If -versiondb is not specified, it updates to the specified migration in the current version of the database.")

	flag.Parse()

	configParameters := &Parameters{
		IsVersion:           *isVersion,
		IsVerbose:           *isVerbose,
		PathToMigrations:    *pathToMigrations,
		StringVersionDb:     *stringVersionDb,
		StringNameMigration: *stringNameMigration,
	}

	configDbEntry := &DbEntry{
		Host:     *host,
		Port:     *port,
		DbName:   *dbname,
		User:     *user,
		Password: *password,
	}

	return configParameters, configDbEntry
}
