package exporter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/EvilFreelancer/outlook-pst-mcp/internal/exporter"
	"github.com/EvilFreelancer/outlook-pst-mcp/internal/store"
)

func TestExportCopiesNonDeletedMessagesAndWritesManifest(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "messages")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "one.eml"), []byte("Subject: One\r\n\r\nBody\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "two.eml"), []byte("Subject: Two\r\n\r\nBody\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(tmp, "export")
	err := exporter.Export(exporter.Request{
		OutputDir: out,
		Folders: []store.Folder{
			{ID: "f1", Name: "Inbox", Path: "Inbox"},
		},
		Messages: []store.Message{
			{ID: "m1", FolderID: "f1", Subject: "One", EMLPath: filepath.Join(source, "one.eml")},
			{ID: "m2", FolderID: "f1", Subject: "Two", EMLPath: filepath.Join(source, "two.eml"), Deleted: true},
		},
	})
	if err != nil {
		t.Fatalf("Export returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(out, "Inbox", "m1.eml")); err != nil {
		t.Fatalf("exported message missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "Inbox", "m2.eml")); !os.IsNotExist(err) {
		t.Fatalf("deleted message should be skipped, stat err=%v", err)
	}

	var manifest exporter.Manifest
	data, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("manifest json invalid: %v", err)
	}
	if manifest.FolderCount != 1 || manifest.MessageCount != 1 || manifest.SkippedDeletedCount != 1 {
		t.Fatalf("manifest = %#v", manifest)
	}
}
