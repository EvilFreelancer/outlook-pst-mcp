package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"outlook-pst-mcp/internal/app"
	"outlook-pst-mcp/internal/mail"
)

func TestServiceCRUDAndExportWorkflow(t *testing.T) {
	workspace := t.TempDir()
	svc, err := app.Open(workspace)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer svc.Close()

	folder, err := svc.EnsureFolder("Inbox")
	if err != nil {
		t.Fatalf("EnsureFolder returned error: %v", err)
	}
	created, err := svc.CreateMessage(app.CreateMessageRequest{
		FolderID: folder.ID,
		Message:  mail.Message{Subject: "Created", From: "a@example.com", To: []string{"b@example.com"}, BodyText: "Body"},
	})
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}
	if err := svc.UpdateMessage(created.ID, mail.Patch{Subject: stringPtr("Updated")}); err != nil {
		t.Fatalf("UpdateMessage returned error: %v", err)
	}
	got, err := svc.GetMessage(created.ID, true, false)
	if err != nil {
		t.Fatalf("GetMessage returned error: %v", err)
	}
	if got.Subject != "Updated" || got.BodyText != "Body" {
		t.Fatalf("message = %#v", got)
	}
	out := filepath.Join(workspace, "exported")
	if err := svc.ExportEML(out, false); err != nil {
		t.Fatalf("ExportEML returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "Inbox", created.ID+".eml")); err != nil {
		t.Fatalf("exported eml missing: %v", err)
	}
}

func TestServiceExportIncludesMoreThanOnePageOfMessages(t *testing.T) {
	workspace := t.TempDir()
	svc, err := app.Open(workspace)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer svc.Close()

	folder, err := svc.EnsureFolder("Inbox")
	if err != nil {
		t.Fatalf("EnsureFolder returned error: %v", err)
	}
	for i := 0; i < 205; i++ {
		if _, err := svc.CreateMessage(app.CreateMessageRequest{
			FolderID: folder.ID,
			Message:  mail.Message{Subject: "Bulk", From: "a@example.com", To: []string{"b@example.com"}, BodyText: "Body"},
		}); err != nil {
			t.Fatalf("CreateMessage %d returned error: %v", i, err)
		}
	}

	out := filepath.Join(workspace, "bulk-export")
	if err := svc.ExportEML(out, false); err != nil {
		t.Fatalf("ExportEML returned error: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(out, "Inbox"))
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	if len(entries) != 205 {
		t.Fatalf("exported entries = %d, want 205", len(entries))
	}
}

func stringPtr(v string) *string {
	return &v
}
