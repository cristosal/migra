package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/cristosal/migra"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

var (
	connectionString string
	driver           string
	popUntil         string
	popAll           bool

	root = &cobra.Command{
		Use:          "migra",
		Short:        "Migration commands",
		SilenceUsage: true,
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
		Use:   "pop",
		Short: "Undo last migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open(driver, getConnectionString())
			if err != nil {
				return err
			}

			m := migra.New(db)

			if popAll {
				n, err := m.PopAll(cmd.Context())
				if err != nil {
					return err
				}
				fmt.Printf("popped %d migrations\n", n)
			} else if popUntil == "" {
				if err := m.Pop(cmd.Context()); err != nil {
					return err
				}

				fmt.Println("popped 1 migration")
			} else {
				if err := m.PopUntil(cmd.Context(), popUntil); err != nil {
					return err
				}
			}

			return nil
		},
	}

	push = &cobra.Command{
		Use:   "push",
		Short: "Pushes a new migration",
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

	list = &cobra.Command{
		Use:   "list",
		Short: "list all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open(driver, getConnectionString())
			if err != nil {
				return err
			}

			m := migra.New(db)
			migrations, err := m.List(cmd.Context())
			if err != nil {
				return err
			}

			if len(migrations) == 0 {
				return errors.New("no migrations")
			}

			tw := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
			defer tw.Flush()
			fmt.Fprintf(tw, "| %s\t| %s\t| %s\t| %s\n", "ID", "Name", "Description", "Migrated At")
			for i := range migrations {
				mig := migrations[i]
				fmt.Fprintf(tw, "| %d\t| %s\t| %s\t| %s\n", mig.ID, mig.Name, mig.Description, mig.MigratedAt.Format(time.RFC1123))
			}
			return nil

		},
	}

	migration = migra.Migration{
		//
	}
)

func main() {
	root.AddCommand(initialize, list, push, pop)
	root.Execute()
}

func init() {
	root.PersistentFlags().StringVar(&connectionString, "conn", "", "database connection string. If unset, defaults to environment variable MIGRA_CONNECTION_STRING")
	root.PersistentFlags().StringVar(&driver, "driver", "pgx", "database driver")

	pop.Flags().StringVar(&popUntil, "until", "", "pop until migration with this name is reached")
	pop.Flags().BoolVarP(&popAll, "all", "a", false, "pop all migrations")

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
