package migra

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const DefaultMigrationTable = "_migra"

type Migration struct {
	ID          int64
	Name        string
	Description string
	Up          string
	Down        string
	Position    int64
	MigratedAt  time.Time
}

type Migra struct {
	db        *sql.DB
	tableName string
}

// New creates a new Migra instance.
func New(db *sql.DB) *Migra {
	return &Migra{
		db:        db,
		tableName: DefaultMigrationTable,
	}
}

// SetMigrationsTable sets the default table where migrations will be stored and executed
func (m *Migra) SetMigrationsTable(table string) {
	m.tableName = table
}

// CreateMigrationTable creates the table where migrations will be stored and executed.
// The name of the table can be set using the SetMigrationsTable method.
// Otherwise, the value of DefaultMigrationTable is used.
func (m *Migra) CreateMigrationTable(ctx context.Context) error {
	if m.tableName == "" {
		m.tableName = DefaultMigrationTable
	}

	_, err := m.db.ExecContext(ctx, fmt.Sprintf(`create table if not exists %s (
		id serial primary key,
		name varchar(255) not null unique,
		description text,
		up text,
		down text,
		position serial not null,
		migrated_at timestamptz
	);`, m.tableName))

	return err
}

// Push adds a migration to the database and executes it
func (m *Migra) Push(ctx context.Context, migration *Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	if err := m.insertMigrationTx(ctx, tx, migration); err != nil {
		return err
	}

	if err := m.upMigrationTx(ctx, tx, migration); err != nil {
		return err
	}

	return tx.Commit()
}

// ListMigrations returns all the executed migrations
func (m *Migra) ListMigrations(ctx context.Context) ([]Migration, error) {
	sql := fmt.Sprintf(`SELECT id, name, description, up, down, position, migrated_at FROM %s ORDER BY position ASC`, m.tableName)
	rows, err := m.db.QueryContext(ctx, sql)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	migrations := make([]Migration, 0)
	for rows.Next() {
		var migration Migration
		if err := rows.Scan(
			&migration.ID,
			&migration.Name,
			&migration.Description,
			&migration.Up,
			&migration.Down,
			&migration.Position,
			&migration.MigratedAt); err != nil {
			return migrations, err
		}

		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// DropMigrationTable drops the migrations table
func (m *Migra) DropMigrationTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("drop table %s", m.tableName))
	return err
}

func (m *Migra) insertMigrationTx(ctx context.Context, tx *sql.Tx, mig *Migration) error {
	sql := fmt.Sprintf("INSERT INTO %s (name, description, up, down) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", m.tableName)
	_, err := tx.ExecContext(ctx, sql, mig.Name, mig.Description, mig.Up, mig.Down)
	return err
}

func (m *Migra) upMigrationTx(ctx context.Context, tx *sql.Tx, mig *Migration) error {
	if _, err := tx.ExecContext(ctx, mig.Up); err != nil {
		return err
	}

	sql := fmt.Sprintf("update %s set migrated_at = now() where name = $1", m.tableName)
	_, err := tx.ExecContext(ctx, sql, mig.Name)
	return err
}
