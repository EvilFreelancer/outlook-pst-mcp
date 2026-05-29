# Product Specification

## Goal

Build a local Go MCP server that lets an MCP client inspect and edit messages from an Outlook 2013 PST archive without modifying the source PST file.

The server reads the PST through `readpst`, indexes extracted mail, applies CRUD operations to a local workspace, and exports the final state as an EML folder tree that can be imported into mail software.

## Constraints

- The implementation language is Go.
- The source `.pst` file is read-only input and must never be modified.
- PST extraction is delegated to the external `readpst` tool from `libpst`.
- Export output is an EML directory tree plus a machine-readable `manifest.json`.
- The server runs locally over stdio as an MCP server.
- Documentation, tests, source code comments, and README content are written in English.
- Development order is documentation first, then tests, then implementation, then README updates.

## Non-Goals

- Direct write-back to PST.
- Creating a new PST file.
- Synchronizing with Outlook, Exchange, IMAP, Microsoft Graph, or any remote mailbox.
- Running `readpst` automatically on every server startup.
- Providing a web UI.

## User Workflow

1. Import a PST into a workspace with the `outlook-pst-mcp import` CLI subcommand.
2. Start the MCP server with the same workspace directory.
3. Browse folders and message summaries through MCP tools.
4. Read full message content when needed.
5. Create, update, delete, or move messages in the editable workspace.
6. Call `export_eml` to produce an importable EML folder tree and `manifest.json`.

## Safety Model

The PST file is never opened for writing. All mutable state lives under the configured workspace. Edits are represented as SQLite metadata plus canonical `.eml` files in the workspace.
