package migra_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cristosa/migra"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ctx = context.Background()
var connectionString = "postgres://migra:migra@localhost:5432/migra"

func initMigra(t *testing.T) *migra.Migra {
	db, err := sql.Open("pgx", connectionString)

	if err != nil {
		t.Fatal(err)
	}

	m := migra.New(db)

	if err := m.CreateMigrationTable(ctx); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		m.DropMigrationTable(ctx)
	})

	return m
}

func TestMigrateUp(t *testing.T) {
	m := initMigra(t)
	migrations := []migra.Migration{
		{
			Name:        "Test Users",
			Description: "Creates test users table with username and password fields",
			Up: `CREATE TABLE test_users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) NOT NULL UNIQUE,
			password VARCHAR(1024) NOT NULL
		);`,
			Down: `DROP TABLE test_users;`,
		},
		{
			Name:        "Default User",
			Description: "Adds a default test user",
			Up:          "INSERT INTO test_users (username, password) VALUES ('foo', 'bar')",
			Down:        "DELETE FROM test_users WHERE username = 'foo'",
		},
	}

	for i := range migrations {
		if err := m.Push(ctx, &migrations[i]); err != nil {
			t.Fatal(err)
		}
	}

	found, err := m.ListMigrations(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != len(migrations) {
		t.Fatalf("expected %d migrations, got %d", len(migrations), len(found))
	}
}

func TestInitMigrationTables(t *testing.T) {
	m := initMigra(t)

	if err := m.CreateMigrationTable(ctx); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		m.DropMigrationTable(ctx)
	})
}
