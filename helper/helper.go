package helper

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"dbupdater/config"

	"github.com/jackc/pgx/v5"
)

// Executes Println if isVerbose is true
func ShowIfVerbose(isVerbose bool, str string) {
	if isVerbose {
		fmt.Println(str)
	}
}

// Opens a connection to the postgres database
func OpenConnect(ctx context.Context, config *config.DbEntry, isVerbose bool) (*pgx.Conn, error) {
	ShowIfVerbose(isVerbose, "Connecting to the database...")

	connString := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s target_session_attrs=read-write",
		config.Host, config.Port, config.DbName, config.User, config.Password)

	connConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, err
	}

	ShowIfVerbose(isVerbose, "Connection established.")
	return conn, nil
}

// Checks that the string starts with "v"
func IsFirstV(name string) bool {
	if name[:len("v")] == "v" {
		return true
	}
	return false
}

func GetFilesInDir(pathToDir string) ([]fs.DirEntry, error) {
	dir, err := os.Open(pathToDir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	files, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func ReadFile(pathToSqlFile string) (string, error) {
	file, err := os.ReadFile(pathToSqlFile)
	if err != nil {
		return "", err
	}

	sql := string(file)
	return sql, nil
}

// Search for the first substring starting at the specified index
func IndexAt(s, sep string, n int) int {
	idx := strings.Index(s[n:], sep)
	if idx > -1 {
		idx += n
	}
	return idx
}
