package migra

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"time"
)

type Migration struct {
	ID         int64
	Filename   string
	MigratedAt time.Time
}

type Migra struct {
	*sql.DB
	fs.ReadDirFS
}

// ListMigrations returns all the executed migrations
func (m *Migra) ListMigrations(ctx context.Context) ([]Migration, error) {
	rows, err := m.QueryContext(ctx, "select id, filename, migrated_at from _migra")

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	migrations := make([]Migration, 0)
	for rows.Next() {
		var migration Migration
		if err := rows.Scan(&migration.ID, &migration.Filename, &migration.MigratedAt); err != nil {
			return migrations, err
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// New creates a new Migra instance.
func New(db *sql.DB, fileSystem fs.ReadDirFS) *Migra {
	return &Migra{
		DB:        db,
		ReadDirFS: fileSystem,
	}
}

// CreateMigrationTable initializes the database with the migrations table
func CreateMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `create table if not exists _migra(
		id serial primary key,
		filename varchar(255) not null,
		migrated_at timestamptz not null default now()
	);`)

	return err
}

func DropMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "drop table _migra")
	return err
}

// Up migrates files up to the given filename.
func (m *Migra) Up(ctx context.Context, filename string) error {
	return fs.WalkDir(m.ReadDirFS, ".", func(path string, entry fs.DirEntry, err error) error {
		if entry.IsDir() {
			return nil
		}

		f, err := m.ReadDirFS.Open(path)
		if err != nil {
			return err
		}

		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			return err
		}

		// we need to execute in transaction
		tx, err := m.DB.Begin()
		if err != nil {
			return err
		}

		defer tx.Rollback()

		// execute the file
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `insert into _migra (filename) values ($1)`, entry.Name()); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		// here we weould break
		if entry.Name() == filename {
			return nil
		}

		return nil
	})
}
