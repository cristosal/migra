package migra_test

import (
	"context"
	"os"
	"testing"

	"github.com/cristosa/migra"
	"github.com/jackc/pgx/v5"
)

func TestUpMigrations(t *testing.T) {
	conn, err := pgx.Connect(context.Background(), os.Getenv("CONNECTION_STRING"))
	if err != nil {
		t.Fatal(err)
	}

	m := migra.New(conn, nil)
}
