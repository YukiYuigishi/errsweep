# Contributing to errsweep

Thanks for your interest in contributing.

## Development setup

```bash
make dev-setup
```

This installs required tools, builds binaries, and configures git hooks.

## Before opening a PR

```bash
make lint-go
make test-all
```

`pre-commit` also runs `golangci-lint --fix` and `make test-all`.

## Coding guidelines

- Keep changes focused and small.
- Follow existing naming and package conventions.
- Add or update tests for behavior changes.
- Wrap propagated errors with context using `%w`.

## Commit messages

Use concise, imperative summaries, for example:

- `analyzer: add net/http ReadRequest mapping`
- `proxy: throttle cache refresh frequency`

## Reporting issues

Please include:

- Reproduction steps
- Expected behavior
- Actual behavior
- Go version and OS
