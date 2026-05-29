# File Workflow Specification

## Workspace Layout

```text
workspace/
  mailbox.db
  extracted/
    <readpst output>
  messages/
    <message-id>.eml
  export/
    <folder tree>
    manifest.json
```

## PST Input

The PST path is provided to the `outlook-pst-mcp import` CLI subcommand. The importer validates that the path exists and is a regular file before running `readpst`.

The source PST is treated as immutable input. The server must not write, truncate, replace, or delete it.

## `readpst` Extraction

The `internal/pst` package locates `readpst` on `PATH` and runs it with `-e`, `-b`, and `-o` so each message is written as a separate `.eml` file under the workspace extraction directory. The `-b` flag skips RTF body attachments that can crash some `readpst` builds on certain PST items. Extraction output is raw import material.

The importer walks the extraction directory and discovers `.eml` files. Each file is read for header metadata, assigned a stable message ID from the message `Date` header (or file modification time as a fallback), and copied into the canonical message store. SQLite stores the canonical path, not the raw extraction path.

## Canonical EML Store

After import, the editable message payload is stored under:

```text
workspace/messages/<message-id>.eml
```

The canonical message store is the source of truth for message payload content after import. The raw `extracted` directory can be regenerated and is not treated as editable state.

## SQLite Metadata

`workspace/mailbox.db` stores folders, indexed message fields, payload paths, soft-delete state, and change history.

SQLite metadata lets the server list, filter, move, and soft-delete messages without repeatedly parsing every EML file.

## CRUD File Effects

- Create writes a new canonical EML file and inserts message metadata.
- Update patches the canonical EML file and updates indexed metadata.
- Delete marks the message as deleted in SQLite and leaves the EML file on disk.
- Move updates the owning folder ID in SQLite and does not rewrite the EML payload.

## Export

`export_eml` creates an output folder tree that mirrors indexed folders. By default, soft-deleted messages are skipped. Exported filenames use internal message IDs to avoid collisions.

`manifest.json` records export timestamp, folder count, exported message count, and skipped deleted count.
