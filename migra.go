package migra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DefaultMigrationTable name
const DefaultMigrationTable = "_migra"

// Migration is a structured change to the database
type Migration struct {
	ID          int64
	Name        string
	Description string
	Up          string
	Down        string
	Position    int64
	MigratedAt  time.Time
}

// Migra is contains methods for migrating an sql database
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

// DB Allows access to the underlying sql database
func (m *Migra) DB() *sql.DB {
	return m.db
}

// SetMigrationsTable sets the default table where migrations will be stored and executed
func (m *Migra) SetMigrationsTable(table string) *Migra {
	m.tableName = table
	return m
}

// Init creates the table where migrations will be stored and executed.
// The name of the table can be set using the SetMigrationsTable method.
func (m *Migra) Init(ctx context.Context) error {
	if m.tableName == "" {
		m.tableName = DefaultMigrationTable
	}

	_, err := m.db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL UNIQUE,
		description TEXT,
		up TEXT,
		down TEXT,
		position SERIAL NOT NULL,
		migrated_at TIMESTAMPTZ
	);`, m.tableName))

	return err
}

// Push adds a migration to the database and executes it
func (m *Migra) Push(ctx context.Context, migration *Migration) error {
	if migration.Name == "" {
		return errors.New("migration name is required")
	}

	if migration.Up == "" {
		return errors.New("no up migration specified")
	}

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT name FROM %s WHERE name = $1", m.tableName), migration.Name)
	var name string
	row.Scan(&name)

	if name == migration.Name {
		// we have already pushed it
		return nil
	}

	if err := m.insertMigrationTx(ctx, tx, migration); err != nil {
		return fmt.Errorf("migration %s failed: %w", migration.Name, err)
	}

	if err := m.upMigrationTx(ctx, tx, migration); err != nil {
		return fmt.Errorf("migration %s failed: %w", migration.Name, err)
	}

	return tx.Commit()
}

// PushMany pushes multiple migrations and returns first error encountered
func (m *Migra) PushMany(ctx context.Context, migrations []Migration) error {
	for i := range migrations {
		if err := m.Push(ctx, &migrations[i]); err != nil {
			return err
		}
	}

	return nil
}

// Pop undoes the last executed migration
func (m *Migra) Pop(ctx context.Context) error {
	sql := fmt.Sprintf(`SELECT name, down FROM %s ORDER BY position DESC`, m.tableName)

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, sql)
	if err := row.Err(); err != nil {
		return err
	}

	var (
		name string
		down string
	)

	if err := row.Scan(&name, &down); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, down); err != nil {
		return err
	}

	sql = fmt.Sprintf("DELETE FROM %s WHERE name = $1", m.tableName)
	if _, err := tx.ExecContext(ctx, sql, name); err != nil {
		return err
	}

	return tx.Commit()
}

// PopAll pops all migrations
func (m *Migra) PopAll(ctx context.Context) error {
	var err error

	for err == nil {
		err = m.Pop(ctx)
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	return err
}

// PopUntil pops until it reaches a migration with given name
func (m *Migra) PopUntil(ctx context.Context, name string) error {
	var (
		mig *Migration
		err error
	)

	for {
		mig, err = m.GetLatest(ctx)

		if err != nil {
			return err
		}

		if mig.Name == name {
			return nil
		}

		if err := m.Pop(ctx); err != nil {
			return err
		}
	}

}

// GetLatest returns the latest migration executed
func (m *Migra) GetLatest(ctx context.Context) (*Migration, error) {
	sql := fmt.Sprintf(`SELECT id, name, description, up, down, position, migrated_at FROM %s ORDER BY position DESC`, m.tableName)
	row := m.db.QueryRowContext(ctx, sql)

	if err := row.Err(); err != nil {
		return nil, err
	}

	var mig Migration
	if err := row.Scan(
		&mig.ID,
		&mig.Name,
		&mig.Description,
		&mig.Up,
		&mig.Down,
		&mig.Position,
		&mig.MigratedAt); err != nil {
		return nil, err
	}

	return &mig, nil

}

// List returns all the executed migrations
func (m *Migra) List(ctx context.Context) ([]Migration, error) {
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

// Drop the migrations table
func (m *Migra) Drop(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", m.tableName))
	return err
}

func (m *Migra) insertMigrationTx(ctx context.Context, tx *sql.Tx, mig *Migration) error {
	sql := fmt.Sprintf("INSERT INTO %s (name, description, up, down) VALUES ($1, $2, $3, $4)", m.tableName)
	_, err := tx.ExecContext(ctx, sql, mig.Name, mig.Description, mig.Up, mig.Down)
	return err
}

func (m *Migra) upMigrationTx(ctx context.Context, tx *sql.Tx, mig *Migration) error {
	if _, err := tx.ExecContext(ctx, mig.Up); err != nil {
		return err
	}

	sql := fmt.Sprintf("UPDATE %s SET migrated_at = NOW() WHERE name = $1", m.tableName)
	_, err := tx.ExecContext(ctx, sql, mig.Name)
	return err
}
