package store_test

import (
	"path/filepath"
	"testing"

	"outlook-pst-mcp/internal/store"
)

func TestStoreFolderMessageCRUDAndChanges(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "mailbox.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	folder, err := st.CreateFolder("Inbox", "Inbox", nil)
	if err != nil {
		t.Fatalf("CreateFolder returned error: %v", err)
	}

	msg, err := st.CreateMessage(store.Message{
		FolderID: folder.ID,
		Subject:  "Hello",
		FromAddr: "alice@example.com",
		ToAddrs:  []string{"bob@example.com"},
		EMLPath:  "messages/1.eml",
	})
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}

	if err := st.UpdateMessage(msg.ID, store.MessagePatch{Subject: strPtr("Updated"), ToAddrs: &[]string{"carol@example.com"}}); err != nil {
		t.Fatalf("UpdateMessage returned error: %v", err)
	}

	archive, err := st.CreateFolder("Archive", "Archive", nil)
	if err != nil {
		t.Fatalf("CreateFolder archive returned error: %v", err)
	}
	if err := st.MoveMessage(msg.ID, archive.ID); err != nil {
		t.Fatalf("MoveMessage returned error: %v", err)
	}
	if err := st.DeleteMessage(msg.ID); err != nil {
		t.Fatalf("DeleteMessage returned error: %v", err)
	}

	visible, total, err := st.ListMessages(store.MessageFilter{Limit: 20})
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if total != 0 || len(visible) != 0 {
		t.Fatalf("visible messages total=%d len=%d", total, len(visible))
	}

	deleted, total, err := st.ListMessages(store.MessageFilter{IncludeDeleted: true, Limit: 20})
	if err != nil {
		t.Fatalf("ListMessages include deleted returned error: %v", err)
	}
	if total != 1 || len(deleted) != 1 || deleted[0].Subject != "Updated" || deleted[0].FolderID != archive.ID {
		t.Fatalf("deleted messages total=%d messages=%#v", total, deleted)
	}

	changes, err := st.ListChanges()
	if err != nil {
		t.Fatalf("ListChanges returned error: %v", err)
	}
	if got, want := len(changes), 4; got != want {
		t.Fatalf("changes len=%d want %d: %#v", got, want, changes)
	}
}

func strPtr(v string) *string {
	return &v
}
