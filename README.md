# sqlive

sqlive is a small toolkit to prototype SQL workflows, with a lightweight TUI for running and inspecting queries against a local test database.



## Architecture

- `main.go` — application entrypoint (CLI / server bootstrap).
- `sqlive/` — core runtime package that wires schema, queries, suggestions, and execution.
- `schema/loader.go` — schema loader and parsers used to build runtime models.
- `query/model.go` — domain model for queries and related helpers.
- `suggestion/engine.go` — suggestion/completion helpers and heuristics.
- `ui/` — lightweight handlers and renderers (`parse.go`, `render.go`, `execute.go`, `update.go`, `view.go`).
- `gen_testdb.py` — small helper to create a deterministic test database for development.


## Getting started

### Prerequisites

- Go 1.18+
- Python 3 (optional, for `gen_testdb.py`)

### Build and run locally

```bash
go build -o sqlive-app ./
./sqlive-app
```

### Or run directly for development:

```bash
go run main.go
```

### Generate the test database (optional)

```bash
python3 gen_testdb.py
```

## Usage notes

- The `ui` handlers are simple and intended as examples. You can reuse them directly or implement an HTTP/CLI front-end that calls into the `sqlive` package.
- The suggestion engine provides in-process helpers; adapt or extend `suggestion/engine.go` for more advanced completion or analytics.

## Development

- Formatting: run `gofmt` / `go vet` as part of your workflow.
- Tests: add unit tests alongside packages. For DB-dependent tests, use `gen_testdb.py` to create reproducible fixtures or mock the DB layer.

## Contributing

- Open an issue for design discussions and feature requests.
- Send small, focused PRs. Follow Go conventions for packages and module boundaries.

## License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0).
