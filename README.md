# Migra

Easy to use migrations for go

Stack architecture migration system written in go

## Installation

`go get -u github.com/cristosa/migra`

migra also contains a CLI tool that can be built from source to install simply clone the repo and run

`go install ./cmd/migra.go`

## Getting Started

Create a new instance of migra from `*sql.DB`

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
m.Push(context.Background(), &migra.Migration{
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
m.Pop(context.Background())
```
