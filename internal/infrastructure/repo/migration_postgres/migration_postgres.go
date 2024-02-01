package migration_postgres

import (
	"context"
	"fmt"

	"dbupdater/internal/domain"

	"github.com/jackc/pgx/v5"
)

type MigrationPostgresRepo struct {
	conn *pgx.Conn
}

func NewMigrationPostgresRepo(conn *pgx.Conn) *MigrationPostgresRepo {
	return &MigrationPostgresRepo{
		conn: conn,
	}
}

func (mRepo *MigrationPostgresRepo) HasCurrentMigration(ctx context.Context, sqlForCheckMigration string) (bool, error) {
	isAvailable := false
	err := mRepo.conn.QueryRow(ctx, sqlForCheckMigration).Scan(&isAvailable)
	if err != nil {
		return false, err
	}
	return isAvailable, nil
}

// Returns the last applied migration by executing a sql query
func (mRepo *MigrationPostgresRepo) GetCurrentMigration(ctx context.Context, sqlForGetMigration string) (*domain.Migration, error) {
	row, err := mRepo.conn.Query(ctx, sqlForGetMigration)
	if err != nil {
		return nil, err
	}
	defer row.Close()

	migrationFromDb, err := pgx.CollectOneRow(row, pgx.RowToStructByName[migration])
	if err != nil {
		return nil, err
	}
	migration, err := migrationRepoToDomain(&migrationFromDb)
	if err != nil {
		return nil, fmt.Errorf("migrationRepoToDomain failed: %w", err)
	}
	return migration, nil
}

func (mRepo *MigrationPostgresRepo) UpdateCurrentMigration(ctx context.Context, sqlForUpdateMigration string, lastAppliedMigration *domain.Migration) (err error) {
	migration := migrationDomainToRepo(lastAppliedMigration)
	// Example sqlForUpdateCurrentVersion: UPDATE lastMigration SET version_db=$1, name=$2;
	if _, err := mRepo.conn.Exec(ctx, sqlForUpdateMigration, migration.VersionDb, migration.Name); err != nil {
		return err
	}
	return nil
}

func (mRepo *MigrationPostgresRepo) ExecSql(ctx context.Context, sql string) (err error) {
	if _, err := mRepo.conn.Exec(ctx, sql); err != nil {
		return err
	}
	return nil
}
