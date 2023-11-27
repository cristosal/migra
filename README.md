# Migra

Migra is a command line interface and library for managing sql migrations.

## Installation

In order to use migra as a library, import the package as follows.

`go get -u github.com/cristosal/migra`

To build and install the CLI from source run the following command.
This assumes you have `go` installed in your local environment, and have cloned the repo

`go install ./cmd/migra.go`

## Getting Started

Create a new instance of migra from an `*sql.DB`

```go
m := migra.New(db)
```

Initialize migration tables

```go
err := m.Init(context.Background())
```

The core of migra functionality is encompassed in the following methods

```go
// Push adds a migration to the database and executes it.
// No error is returned if migration was already executed
func (m *Migra) Push(ctx context.Context, migration *Migration) error

// Pop executes the down migration removes the migration from db
func (m *Migra) Pop(ctx context.Context) error
```

Example of pushing a migration

```go
m.Push(context.TODO(), &migra.Migration{
	Name:        "Create Users Table",
	Description: "Add Users Table with username and password fields",
	Up: `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) NOT NULL UNIQUE,
		password TEXT NOT NULL
	)`,
	Down: "DROP TABLE users"
})
```

This Migration can then be reversed by calling the Pop method

```go
m.Pop(context.TODO())
```

## Using Migration Files

Migra also supports defining migrations in files.

Any file format that is compatible with [viper](https://github.com/spf13/viper) can be used. This includes `json` `yaml` `toml` `ini` among many others.

Each migration file must define the following properties

- `name` - unique name which identifies the migration.
- `description` - description of the migration which provides context for users
- `up` - sql to be executed for the migration
- `down` - sql to be executed in order to reverse the migration

Here is an example of a migration file using `toml`

```toml
# 1-users-table.toml

name = "users-table"
description = "Creates a users table with username and password fields"
up = """
CREATE TABLE users (
	id SERIAL PRIMARY KEY,
	username VARCHAR(255) NOT NULL UNIQUE,
	password TEXT NOT NULL
);
"""
down = "DROP TABLE users;"
```

To execute the migrations from files, several `Push` methods exist

```go
// PushFile pushes the migration file located at filepath
func (m *Migra) PushFile(ctx context.Context, filepath string) error

// PushDir pushes all migration files located within the specified directory
func (m *Migra) PushDir(ctx context.Context, dirpath string) error
```

Migra supports using an `fs.FS` filesystem to help support embedding migrations into the binary.

```go
//PushFileFS is same as PushFile but looks for the filepath in the filesystem
func PushFileFS(ctx context.Context, filesystem fs.FS, filepath string) error

//PushDirFS is same as PushDir but looks for the dirpath in the filesystem
func PushDirFS(ctx context.Context, filesystem fs.FS, dirpath string) error
```

> NOTE: PushDirFS and PushFS are recursive and will push any migration files found in subdirectories

## CLI

When using the CLI, many of migra's methods map to commands with flags. For example:

`PushDir` becomes `migra push -d <directory>`
`PopAll` becomes `migra pop -a`

```
A Command Line Interface for managing sql migrations

Usage:
  migra [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Creates migration tables and schema if specified.
  list        list all migrations
  pop         Undo migration
  push        Pushes a new migration

Flags:
      --conn string     database connection string. If unset, defaults to environment variable MIGRA_CONNECTION_STRING
      --driver string   database driver to use. If unset the environment variable for MIGRA_DRIVER is used otherwise the default driver is pgx.
  -h, --help            help for migra
  -s, --schema string   schema to use (default "public")
  -t, --table string    migrations table to use (default "_migrations")

Use "migra [command] --help" for more information about a command.
```
