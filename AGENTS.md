# Agent Instructions

This project builds a Go MCP server for working with Outlook PST archives through exported EML files.

## Required Workflow

All project-facing text must be written in English.

Development must follow this order:

1. Write or update documentation in `docs/`.
2. Write tests before implementation code.
3. Write the implementation code.
4. Write or update `README.md` with project purpose, usage, run instructions, and build instructions.

Do not modify the source PST archive. Treat PST files as read-only inputs.

Use Go for implementation. Use `readpst` from `libpst` for PST extraction. Export edited mailbox state as an EML directory tree with a manifest.

## Project Interface

Use the project `Makefile` for common work:

- `make fmt`
- `make check`
- `make test`
- `make build`
- `make install`
- `make run`

Run `make check`, `make test`, and `make build` before reporting work as complete.

## Documentation Layout

Keep documentation organized by topic:

- `docs/product.md`
- `docs/architecture.md`
- `docs/file-workflow.md`
- `docs/mcp.md`
- `docs/development.md`
- `docs/implementation-plan.md`
