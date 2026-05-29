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
- `make run`: run the server over stdio; optional `WORKSPACE=dir` overrides the default `<cwd>/.outlook-pst-mcp_data`.
- `make clean`: remove build output.

`GOCACHE` defaults to `/tmp/outlook-pst-mcp-go-build` so tests and builds work in restricted environments where the default Go cache may not be writable.

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
./bin/outlook-pst-mcp import -pst /absolute/path/to/backup.pst
```

Use `-workspace <dir>` only when the mailbox state should live outside the project default.

## Build

Builds use `-buildvcs=false` because some local workspaces may contain a read-only or synthetic `.git` directory that prevents Go VCS stamping.

Source package directories under `internal/` must not be hidden by ignore rules.
For example, `internal/workspace` is application source code, while `/workspace/`
is a root-level local scratch directory.

## Release Binaries

GitHub Actions uses three workflows for CI and releases:

- `Tests on PR`: runs project verification for pull requests.
- `Tag release on merge`: bumps the latest patch SemVer tag after a merge to
  `main`, unless the commit is already tagged.
- `Release binaries`: builds and publishes release assets for the tag.

The release binary workflow is callable from the tag workflow because tags
pushed with `GITHUB_TOKEN` do not trigger a separate `push: tags` workflow run.
It also supports direct SemVer tag pushes and manual dispatch.

Release assets are named:

```text
outlook-pst-mcp_<version>_<os>_<arch>.tar.gz
outlook-pst-mcp_<version>_<os>_<arch>.zip
SHA256SUMS
```

Linux assets use `.tar.gz`. Windows assets use `.zip`. The checksum file
contains SHA-256 hashes for all uploaded archives.

The project uses `github.com/mattn/go-sqlite3`, so release builds keep CGO
enabled and install the required C cross-compilers.

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

To install a prebuilt release locally, download the archive for the current
operating system and CPU architecture from the GitHub Release, verify it against
`SHA256SUMS`, unpack it, and place the `outlook-pst-mcp` binary in a directory
on `PATH`, such as `~/.local/bin`.
