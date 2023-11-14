package migra

import (
	"context"
	"io"
	"io/fs"
)

type SqlExecuter interface {
	Exec(ctx context.Context, sql string, args ...any) error
}

type Migra struct {
	SqlExecuter
	fs.ReadDirFS
}

// New creates a new Migra instance.
func New(executer SqlExecuter, fileSystem fs.ReadDirFS) *Migra {
	return &Migra{
		SqlExecuter: executer,
		ReadDirFS:   fileSystem,
	}
}

// CreateMigrationTable initializes the database with the migrations table
func (m *Migra) CreateMigrationTable(ctx context.Context) error {
	return m.Exec(ctx, `create table if not exists _migra (
		id serial primary key,
		varchar(255) filename not null,
		migrated_at timestamptz not null default now()
	)`)
}

// Up migrates files up to the given filename.
func (m *Migra) Up(ctx context.Context, filename string) error {
	entries, err := m.ReadDirFS.ReadDir(".")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		f, err := m.ReadDirFS.Open(filename)
		if err != nil {
			return err
		}

		content, err := io.ReadAll(f)
		if err != nil {
			return err
		}

		if err := m.Exec(ctx, string(content)); err != nil {
			return err
		}

		_ = f.Close()

		if entry.Name() == filename {
			break
		}
	}

	return nil
}
