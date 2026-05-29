# Development Specification

## Required Order

Development follows this order:

1. Update documentation in `docs/`.
2. Write or update tests.
3. Update implementation code.
4. Update `README.md`.
5. Run verification.

All project-facing text is written in English.

## Make Targets

The `Makefile` is the primary local interface.

- `make help`: show available targets.
- `make fmt`: run `gofmt` over Go sources.
- `make check`: run Go static analysis with `go vet ./...`.
- `make test`: run the full test suite.
- `make build`: build `bin/outlook-pst-mcp`.
- `make install`: install the binary into `$(BINDIR)`, defaulting to `$(HOME)/.local/bin`.
- `make run`: run the server over stdio with `WORKSPACE`, defaulting to `./workspace`.
- `make clean`: remove build output.

`GOCACHE` defaults to `/tmp/email-parsing-go-build` so tests and builds work in restricted environments where the default Go cache may not be writable.

## Testing

Tests cover:

- EML parsing, building, and patching.
- SQLite folder, message, and change behavior.
- `readpst` execution through a fake executable.
- EML export layout and manifest generation.
- Application CRUD and export workflows.
- MCP tool listing, tool dispatch, and stdio JSON-RPC framing.

## Import

Mailbox import is a CLI subcommand:

```bash
./bin/outlook-pst-mcp import -workspace ./workspace -pst /absolute/path/to/backup.pst
```

## Build

Builds use `-buildvcs=false` because some local workspaces may contain a read-only or synthetic `.git` directory that prevents Go VCS stamping.

## Install

`make install` with no parameters builds the binary and installs it to:

```text
$(HOME)/.local/bin/outlook-pst-mcp
```

Internally the install path is `$(DESTDIR)$(BINDIR)/outlook-pst-mcp`. The default `BINDIR` is `$(HOME)/.local/bin`. Use `PREFIX` or `BINDIR` to install elsewhere:

```bash
make install PREFIX=/usr/local
make install BINDIR=/custom/bin
```
