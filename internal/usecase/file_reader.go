package usecase

import (
	"fmt"
	"os"
	"strings"

	"dbupdater/helper"
)

type FileReaderUseCase struct {
	isVerbose        bool
	pathToMigrations string

	NameDirForSystemSqlFile               string
	ShortPathToHasCurrentMigrationFile    string
	ShortPathToGetCurrentMigrationFile    string
	ShortPathToUpdateCurrentMigrationFile string
}

const (
	nameDirForSystemSqlFile               = "utils"
	shortPathToHasCurrentMigrationFile    = nameDirForSystemSqlFile + "/HasCurrentVersion.sql"
	shortPathToGetCurrentMigrationFile    = nameDirForSystemSqlFile + "/GetCurrentVersion.sql"
	shortPathToUpdateCurrentMigrationFile = nameDirForSystemSqlFile + "/UpdateCurrentVersion.sql"
)

func NewFileReaderUseCase(pathToMigrations string, isVerbose bool) *FileReaderUseCase {
	return &FileReaderUseCase{
		isVerbose:        isVerbose,
		pathToMigrations: pathToMigrations,

		NameDirForSystemSqlFile:               nameDirForSystemSqlFile,
		ShortPathToHasCurrentMigrationFile:    shortPathToHasCurrentMigrationFile,
		ShortPathToGetCurrentMigrationFile:    shortPathToGetCurrentMigrationFile,
		ShortPathToUpdateCurrentMigrationFile: shortPathToUpdateCurrentMigrationFile,
	}
}

func (uc *FileReaderUseCase) IsExistHasCurrentMigrationFile() bool {
	return uc.isExistsFile(uc.ShortPathToHasCurrentMigrationFile)
}

func (uc *FileReaderUseCase) IsExistGetCurrentMigrationFile() bool {
	return uc.isExistsFile(uc.ShortPathToGetCurrentMigrationFile)
}

func (uc *FileReaderUseCase) IsExistUpdateCurrentMigrationFile() bool {
	return uc.isExistsFile(uc.ShortPathToUpdateCurrentMigrationFile)
}

func (uc *FileReaderUseCase) GetSqlFromHasCurrentMigrationFile() (string, error) {
	return uc.readSqlFile(uc.ShortPathToHasCurrentMigrationFile)
}

func (uc *FileReaderUseCase) GetSqlFromGetCurrentMigrationFile() (string, error) {
	return uc.readSqlFile(uc.ShortPathToGetCurrentMigrationFile)
}

func (uc *FileReaderUseCase) GetSqlFromUpdateCurrentMigrationFile() (string, error) {
	sqlForUpdateMigration, err := uc.readSqlFile(uc.ShortPathToUpdateCurrentMigrationFile)
	if err != nil {
		return "", err
	}
	isContainsTheExpectedArguments := strings.Contains(sqlForUpdateMigration, "$1") && strings.Contains(sqlForUpdateMigration, "$2")
	if !isContainsTheExpectedArguments {
		return "", fmt.Errorf("the %s must have arguments $1 and $2", uc.ShortPathToUpdateCurrentMigrationFile)
	}
	return sqlForUpdateMigration, nil
}

func (uc *FileReaderUseCase) isExistsFile(shortPath string) bool {
	path := fmt.Sprintf("%s/%s", uc.pathToMigrations, shortPath)

	helper.ShowIfVerbose(uc.isVerbose, fmt.Sprintf("Checking for the presence of %s...", shortPath))
	_, err := os.Stat(path)
	if err == nil {
		helper.ShowIfVerbose(uc.isVerbose, fmt.Sprintf("The file %s is available.", shortPath))
		return true
	}
	if os.IsNotExist(err) {
		helper.ShowIfVerbose(uc.isVerbose, fmt.Sprintf("The file %s is not available.", shortPath))
		return false
	}
	helper.ShowIfVerbose(uc.isVerbose, fmt.Sprintf("The file %s is not available, error when checking file.", shortPath))
	return false
}

func (uc *FileReaderUseCase) readSqlFile(shortPath string) (string, error) {
	helper.ShowIfVerbose(uc.isVerbose, fmt.Sprintf("The %s file is being read...", shortPath))
	sqlFromFile, err := helper.ReadFile(uc.pathToMigrations + "/" + shortPath)
	if err != nil {
		return "", fmt.Errorf("error reading %s: %w", shortPath, err)
	}
	helper.ShowIfVerbose(uc.isVerbose, fmt.Sprintf("The %s file has been successfully read.", shortPath))
	return sqlFromFile, nil
}
