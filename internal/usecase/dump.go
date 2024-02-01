package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dbupdater/helper"
	"dbupdater/internal/domain"
)

type DumpInfrastructure interface {
	Create(ctx context.Context, pathForSaveDumps string) (*domain.Dump, error)
	Restore(ctx context.Context, dump *domain.Dump) error
	GetCommandToRestoreDump(dump *domain.Dump) (string, error)
}

type DumpUseCase struct {
	isVerbose        bool
	pathForSaveDumps string
	infrastructure   DumpInfrastructure
}

func NewDumpUseCase(infrastructure DumpInfrastructure, isVerbose bool) (*DumpUseCase, error) {
	pathForSaveDumps, err := getPathForSaveDumps()
	if err != nil {
		return nil, fmt.Errorf("failed to set the path to the directory where to save migrations: %w", err)
	}

	return &DumpUseCase{
		isVerbose:        isVerbose,
		pathForSaveDumps: pathForSaveDumps,
		infrastructure:   infrastructure,
	}, nil
}

func (uc *DumpUseCase) Create(ctx context.Context) (*domain.Dump, error) {
	helper.ShowIfVerbose(uc.isVerbose, "Dump is created...")
	newDump, err := uc.infrastructure.Create(ctx, uc.pathForSaveDumps)
	if err != nil {
		return nil, err
	}
	helper.ShowIfVerbose(uc.isVerbose, "Dump created.")
	return newDump, nil
}

// There must be no connections to the database
func (uc *DumpUseCase) RestoreDatabaseFromDumpAndDeleteDump(ctx context.Context, dump *domain.Dump) error {
	fmt.Println("The database is being restored from the dump...")
	if err := uc.infrastructure.Restore(ctx, dump); err != nil {
		if err != nil {
			return err
		}
	}
	fmt.Println("The database from the dump has been restored.")

	if err := os.Remove(dump.Path()); err != nil {
		fmt.Println("Db recovery was successful, error in deleting dump file after recovery: %w", err)
		return nil
	}
	helper.ShowIfVerbose(uc.isVerbose, "Dump deleted.")
	return nil
}

func (uc *DumpUseCase) GetErrorForBadRestore(dump *domain.Dump) error {
	commandToRestoreDump, err := uc.infrastructure.GetCommandToRestoreDump(dump)
	if err != nil {
		return fmt.Errorf("manually restore the database. The path to the dump: %s: %w", dump.Path(), err)
	}
	return fmt.Errorf("manually restore the database. "+
		"You can try to restore the dump manually using the command: %s", commandToRestoreDump)
}

const nameDirForDumps = "dumps"

// Returns the path to the directory to save the dumps.
func getPathForSaveDumps() (string, error) {
	pathToExecutable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	pathToDirWithExecutable := filepath.Dir(pathToExecutable)
	pathToDumps := fmt.Sprintf("%s/%s", pathToDirWithExecutable, nameDirForDumps)

	if err := os.MkdirAll(pathToDumps, 600); err != nil {
		return "", fmt.Errorf("trying to create a directory '%s' to save the dumps, error: %w", nameDirForDumps, err)
	}

	return pathToDumps, nil
}
