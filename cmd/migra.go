package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/cristosal/migra"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

var (
	connectionString string
	driver           string

	root = &cobra.Command{
		Use:   "migra",
		Short: "Migration commands",
	}

	initialize = &cobra.Command{
		Use:   "init",
		Short: "Creates migration tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open(driver, getConnectionString())
			if err != nil {
				return err
			}

			m := migra.New(db)
			return m.Init(cmd.Context())
		},
	}

	pop = &cobra.Command{
		Use:          "pop",
		Short:        "Undo last migration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open(driver, getConnectionString())
			if err != nil {
				return err
			}

			m := migra.New(db)
			if err := m.Pop(cmd.Context()); err != nil {
				return err
			}

			fmt.Println("migration popped")
			return nil
		},
	}

	push = &cobra.Command{
		Use:          "push",
		Short:        "Pushes a new migration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open(driver, getConnectionString())
			if err != nil {
				return err
			}

			m := migra.New(db)
			if err := m.Push(cmd.Context(), &migration); err != nil {
				return err
			}

			fmt.Println("done")
			return nil
		},
	}

	migration = migra.Migration{
		//
	}
)

func main() {
	root.AddCommand(initialize, push, pop)
	root.Execute()
}

func init() {
	root.PersistentFlags().StringVar(&connectionString, "conn", "", "database connection string. If unset, defaults to environment variable MIGRA_CONNECTION_STRING")
	root.PersistentFlags().StringVar(&driver, "driver", "pgx", "database driver")

	push.Flags().StringVar(&migration.Name, "name", "", "name of migration")
	push.Flags().StringVar(&migration.Description, "desc", "", "description of migration")
	push.Flags().StringVar(&migration.Up, "up", "", "up migration sql")
	push.Flags().StringVar(&migration.Down, "down", "", "down migration sql")
}

func getConnectionString() string {
	if connectionString != "" {
		return connectionString
	}

	return os.Getenv("MIGRA_CONNECTION_STRING")
}
