# Contributing to pgkit

Thank you for your interest in contributing to `pgkit`! We welcome contributions from the community to help make `pgkit` more robust, efficient, and user-friendly.

---

## Code of Conduct

By participating in this project, you agree to abide by basic open-source collaboration standards: be respectful, helpful, and constructive.

---

## How Can I Contribute?

### 1. Reporting Bugs

- First, check the [Issue Tracker](https://github.com/retailancer/pgkit/issues) to ensure the bug hasn't already been reported.
- If it's a new issue, open a bug report using our template. Include a clear description, reproduction steps, and ideally a minimal failing Go test/code snippet.

### 2. Suggesting Features

- Open a Feature Request issue detailing your proposal, target use cases, and how it aligns with `pgkit`'s philosophy (lightweight, native `pgx`, no code generation, simple AST builders).

### 3. Submitting Pull Requests

- Fork the repository and create a branch for your work.
- Ensure your changes follow the existing project structure and guidelines.
- Write tests for any new behavior or fixes.
- Submit a Pull Request targeting the `master` branch.

### Running Tests

To run all tests including integration tests:

```bash
PG_DSN="postgres://postgres:postgres@localhost:54360/postgres?sslmode=disable" go test -v -race ./...
```

### Code Formatting and Linting

Please format and lint your code before committing:

```bash
# Format
go fmt ./...

# Lint (uses golangci-lint)
golangci-lint run ./...
```

---

## Architectural & Design Guidelines

1. **Native over Custom**: Keep dependency count to a minimum. `pgkit` is built directly on top of `pgx/v5` and should avoid adding extra dependencies unless absolutely necessary.
2. **Deterministic Builders**: Any SQL statement building logic in `internal/builder` must be deterministic (e.g. sorting map keys before serializing criteria lists).
3. **No Unsafe Concurrency**: Ensure all client and transaction lifecycles are safe under Go's race detector. Keep shared maps behind synchronization primitives (`sync.Mutex` or `sync.RWMutex`).
