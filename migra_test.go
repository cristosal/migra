package migra_test

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"testing"

	"github.com/cristosa/migra"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMigrateUp(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("pgx", os.Getenv("CONNECTION_STRING"))

	if err != nil {
		t.Fatal(err)
	}

	if err := migra.CreateMigrationTable(ctx, db); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		migra.DropMigrationTable(ctx, db)
	})

	// ok we point to the directory
	fs := os.DirFS("sql_test").(fs.ReadDirFS)
	m := migra.New(db, fs)

	if err := m.Up(ctx, ""); err != nil {
		t.Fatal(err)
	}

	migrations, err := m.ListMigrations(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(migrations) != 1 {
		t.Fatal("expected 1 migration")
	}
}

func TestInitMigrationTables(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("pgx", os.Getenv("CONNECTION_STRING"))
	if err != nil {
		t.Fatal(err)
	}

	if err := migra.CreateMigrationTable(ctx, db); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		migra.DropMigrationTable(ctx, db)
	})
}
