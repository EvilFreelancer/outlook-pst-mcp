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

The PST path is provided to `import_mailbox`. The server validates that the path exists and is a regular file before running `readpst`.

The source PST is treated as immutable input. The server must not write, truncate, replace, or delete it.

## `readpst` Extraction

The `internal/pst` package locates `readpst` on `PATH` and runs it with an output directory under the workspace. Extraction output is raw import material.

The importer walks the extraction directory and discovers `.eml` files. Each discovered file is reported with its file path and logical folder path.

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

