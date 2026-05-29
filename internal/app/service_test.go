package app_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"outlook-pst-mcp/internal/app"
	"outlook-pst-mcp/internal/mail"
	"outlook-pst-mcp/internal/store"
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

func TestServiceImportCopiesExtractedMessagesToCanonicalStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake readpst is only used on Unix-like systems")
	}
	workspace := t.TempDir()
	bin := filepath.Join(workspace, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(bin, "readpst")
	body := "#!/bin/sh\nout=\"\"\nwhile [ $# -gt 0 ]; do case \"$1\" in -o) out=\"$2\"; shift 2;; *) shift;; esac; done\nmkdir -p \"$out/Inbox\"\nprintf 'Date: Mon, 02 Jan 2006 15:04:05 +0000\\r\\nSubject: Imported\\r\\nFrom: a@example.com\\r\\nTo: b@example.com\\r\\n\\r\\nBody\\r\\n' > \"$out/Inbox/1.eml\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	pstPath := filepath.Join(workspace, "backup.pst")
	if err := os.WriteFile(pstPath, []byte("pst"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Open(workspace)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer svc.Close()

	folders, messages, skipped, err := svc.ImportMailbox(pstPath)
	if err != nil {
		t.Fatalf("ImportMailbox returned error: %v", err)
	}
	if folders != 1 || messages != 1 || skipped != 0 {
		t.Fatalf("import counts folders=%d messages=%d skipped=%d", folders, messages, skipped)
	}

	listed, total, err := svc.ListMessages(store.MessageFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if total != 1 || len(listed) != 1 {
		t.Fatalf("listed total=%d messages=%#v", total, listed)
	}
	wantPath := filepath.Join(workspace, "messages", listed[0].ID+".eml")
	if listed[0].EMLPath != wantPath {
		t.Fatalf("EMLPath = %q, want canonical path %q", listed[0].EMLPath, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("canonical EML missing: %v", err)
	}
}

func stringPtr(v string) *string {
	return &v
}
