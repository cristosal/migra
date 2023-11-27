package migra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"time"

	"github.com/spf13/viper"
)

const (
	DefaultMigrationTable = "_migrations"
	DefaultSchemaName     = "public"
)

var (
	ErrNoMigration = errors.New("no migration found")
)

// Migration is a structured change to the database
type Migration struct {
	ID          int64
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	Up          string `mapstructure:"up"`
	Down        string `mapstructure:"down"`
	Position    int64
	MigratedAt  time.Time
}

// Migra is contains methods for migrating an sql database
type Migra struct {
	db         *sql.DB
	tableName  string
	schemaName string
}

// New creates a new Migra instance.
func New(db *sql.DB) *Migra {
	return &Migra{
		db:         db,
		tableName:  DefaultMigrationTable,
		schemaName: DefaultSchemaName,
	}
}

// Table returns the fully schema prefixed tablename
func (m *Migra) Table() string {
	return m.schemaName + "." + m.tableName
}

// DB Allows access to the underlying sql database
func (m *Migra) DB() *sql.DB {
	return m.db
}

// SetMigrationsTable sets the default table where migrations will be stored and executed
func (m *Migra) SetMigrationsTable(table string) *Migra {
	if table != "" {
		m.tableName = table
	}
	return m
}

func (m *Migra) SetSchema(schema string) *Migra {
	if schema != "" {
		m.schemaName = schema
	}

	return m
}

// Init creates the table where migrations will be stored and executed.
// The name of the table can be set using the SetMigrationsTable method.
func (m *Migra) Init(ctx context.Context) error {
	if m.schemaName == "" {
		m.schemaName = DefaultSchemaName
	}

	if m.tableName == "" {
		m.tableName = DefaultMigrationTable
	}

	_, err := m.db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", m.schemaName))
	if err != nil {
		return err
	}

	_, err = m.db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL UNIQUE,
		description TEXT,
		up TEXT,
		down TEXT,
		position SERIAL NOT NULL,
		migrated_at TIMESTAMPTZ
	);`, m.Table()))

	return err
}

// PushDir pushes all migrations inside a directory
func (m *Migra) PushDir(ctx context.Context, dirname string) error {
	entries, err := os.ReadDir(dirname)
	if err != nil {
		return err
	}

	for i := range entries {
		filepath := path.Join(dirname, entries[i].Name())
		if err := m.PushFile(ctx, filepath); err != nil {
			return err
		}
	}

	return nil
}

// PushFile pusehs a migration from a file
func (m *Migra) PushFile(ctx context.Context, filepath string) error {
	v := viper.New()
	v.SetConfigFile(filepath)
	if err := v.ReadInConfig(); err != nil {
		return err
	}

	var migration Migration

	if err := v.Unmarshal(&migration); err != nil {
		return err
	}

	return m.Push(ctx, &migration)
}

func (m *Migra) PushDirFS(ctx context.Context, filesystem fs.FS, dirpath string) error {
	// here is where we read
	entries, err := fs.ReadDir(filesystem, dirpath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		filename := path.Join(dirpath, entry.Name())

		if entry.IsDir() {
			if err := m.PushDirFS(ctx, filesystem, filename); err != nil {
				return err
			}
		} else {
			if err := m.PushFileFS(ctx, filesystem, filename); err != nil {
				return err
			}
		}
	}

	return nil
}

// PushFS pushes all migrations in a directory using fs.FS
func (m *Migra) PushFS(ctx context.Context, filesystem fs.FS) error {
	return m.PushDirFS(ctx, filesystem, ".")
}

// PushFileFS pushes a file with given name from the filesystem
func (m *Migra) PushFileFS(ctx context.Context, filesystem fs.FS, filename string) error {
	v := viper.New()
	log.Printf("running file %s", filename)

	f, err := filesystem.Open(path.Join(".", filename))

	if err != nil {
		return err
	}

	defer f.Close()
	ext := path.Ext(filename)
	v.SetConfigType(ext[1:])

	if err := v.ReadConfig(f); err != nil {
		return err
	}

	var migration Migration
	if err := v.Unmarshal(&migration); err != nil {
		return err
	}

	return m.Push(ctx, &migration)
}

// Push adds a migration to the database and executes it
func (m *Migra) Push(ctx context.Context, mig *Migration) error {
	if mig.Name == "" {
		return errors.New("migration name is required")
	}

	if mig.Up == "" {
		return errors.New("up sql is required")
	}

	var (
		sql  = fmt.Sprintf("SELECT name FROM %s WHERE name = $1", m.Table())
		name string
		row  = m.db.QueryRowContext(ctx, sql, mig.Name)
	)

	row.Scan(&name)

	if name == mig.Name {
		// we have already pushed it
		return nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	// insert record of the migration
	sql = fmt.Sprintf("INSERT INTO %s (name, description, up, down) VALUES ($1, $2, $3, $4)", m.Table())
	if _, err := tx.ExecContext(ctx, sql, mig.Name, mig.Description, mig.Up, mig.Down); err != nil {
		return err
	}

	// execute up migration
	if _, err := tx.ExecContext(ctx, mig.Up); err != nil {
		return err
	}

	// set migration as executed
	sql = fmt.Sprintf("UPDATE %s SET migrated_at = NOW() WHERE name = $1", m.Table())
	if _, err := tx.ExecContext(ctx, sql, mig.Name); err != nil {
		return err
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

// Pop reverts the last migration
func (m *Migra) Pop(ctx context.Context) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt := fmt.Sprintf(`SELECT name, down FROM %s ORDER BY position DESC`, m.Table())
	row := tx.QueryRowContext(ctx, stmt)

	var (
		name string
		down string
	)

	if err := row.Scan(&name, &down); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoMigration
		}

		return err
	}

	if _, err := tx.ExecContext(ctx, down); err != nil {
		return err
	}

	stmt = fmt.Sprintf("DELETE FROM %s WHERE name = $1", m.Table())
	if _, err := tx.ExecContext(ctx, stmt, name); err != nil {
		return err
	}

	return tx.Commit()
}

// PopAll reverts all migrations
func (m *Migra) PopAll(ctx context.Context) (int, error) {
	var n int

	for {
		if err := m.Pop(ctx); err != nil {
			if errors.Is(err, ErrNoMigration) {
				if n == 0 {
					return 0, ErrNoMigration
				}

				return n, nil
			}

			return n, err
		}
		n++
	}
}

// PopUntil pops until a migration with given name is reached
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
	sql := fmt.Sprintf(`SELECT id, name, description, up, down, position, migrated_at FROM %s ORDER BY position DESC`, m.Table())
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
	sql := fmt.Sprintf(`SELECT id, name, description, up, down, position, migrated_at FROM %s ORDER BY position ASC`, m.Table())
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
	_, err := m.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", m.Table()))
	return err
}
