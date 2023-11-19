package migra_test

import (
	"context"
	"database/sql"
	"os"
	"path"
	"testing"

	"github.com/cristosal/migra"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ctx = context.Background()
var connectionString = ""

func TestPushDir(t *testing.T) {
	m := initMigra(t)
	dirpath, err := os.MkdirTemp(os.TempDir(), "pushdir")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dirpath)
		m.PopAll(context.Background())
	})

	content := `
name: "First Migration"
description: "Description of my first migration"
up: "CREATE TABLE test_first_migration_table(id serial primary key)"
down: "DROP TABLE test_first_migration_table;"`

	if err := os.WriteFile(path.Join(dirpath, "1.yml"), []byte(content), 0777); err != nil {
		t.Fatal(err)
	}

	if err := m.PushDir(context.Background(), dirpath); err != nil {
		t.Fatal(err)
	}

}

func TestRepeatedUp(t *testing.T) {
	m := initMigra(t)

	migration := migra.Migration{
		Name: "Migration",
		Up:   "CREATE TABLE test_table(id SERIAL PRIMARY KEY)",
		Down: "DROP TABLE test_table",
	}

	t.Cleanup(func() {
		m.PopAll(ctx)
	})

	if err := m.Push(ctx, &migration); err != nil {
		t.Fatal(err)
	}

	if err := m.Push(ctx, &migration); err != nil {
		t.Fatal(err)
	}
}

func initMigra(t *testing.T) *migra.Migra {
	db, err := sql.Open("pgx", connectionString)

	if err != nil {
		t.Fatal(err)
	}

	m := migra.New(db)

	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		m.Drop(ctx)
	})

	return m
}

func TestMigrateUp(t *testing.T) {
	m := initMigra(t)
	if _, err := m.PopAll(ctx); err != nil {
		t.Fatal(err)
	}

	migrations := []migra.Migration{
		{
			Name:        "Test Users",
			Description: "Creates a test users table with username and password fields",
			Up: `CREATE TABLE test_users (
				id SERIAL PRIMARY KEY,
				username VARCHAR(255) NOT NULL UNIQUE,
				password VARCHAR(1024) NOT NULL,
				created_at TIMESTAMPTZ DEFAULT NOW()
			);`,
			Down: `DROP TABLE test_users;`,
		},
		{
			Name:        "First Test User",
			Description: "Adds first test user",
			Up:          "INSERT INTO test_users (username, password) VALUES ('first', 'password')",
			Down:        "DELETE FROM test_users WHERE username = 'first'",
		},
		{
			Name:        "Second Test User",
			Description: "Adds a second test user",
			Up:          "INSERT INTO test_users (username, password) VALUES ('second', 'password')",
			Down:        "DELETE FROM test_users WHERE username = 'second'",
		},
	}

	for i := range migrations {
		mig := &migrations[i]
		if err := m.Push(ctx, mig); err != nil {
			t.Fatalf("error while executing miration %s: %v", mig.Name, err)
		}
	}

	t.Cleanup(func() {
		m.PopAll(ctx)
	})

	// check that migrations show up in list migrations
	found, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != len(migrations) {
		t.Fatalf("expected %d migrations, got %d", len(migrations), len(found))
	}

	expectUsername := func(t *testing.T, username string) {
		row := m.DB().QueryRow("SELECT username FROM test_users ORDER BY created_at DESC")
		if err := row.Err(); err != nil {
			t.Fatal(err)
		}

		var found string
		if err := row.Scan(&found); err != nil {
			t.Fatal(err)
		}

		if found != username {
			t.Fatalf("expected username %s got %s", username, found)
		}
	}

	expectUsername(t, "second")
	if err := m.Pop(ctx); err != nil {
		t.Fatal(err)
	}

	expectUsername(t, "first")
}
