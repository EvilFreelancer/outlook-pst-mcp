package pst_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/EvilFreelancer/outlook-pst-mcp/internal/pst"
)

func TestImporterUsesReadpstAndDiscoversExtractedEML(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(bin, "readpst")
	body := "#!/bin/sh\nout=\"\"\nwhile [ $# -gt 0 ]; do case \"$1\" in -o) out=\"$2\"; shift 2;; *) shift;; esac; done\nmkdir -p \"$out/Inbox\"\nprintf 'Date: Mon, 02 Jan 2006 15:04:05 +0000\\r\\nSubject: Imported\\r\\n\\r\\nBody\\r\\n' > \"$out/Inbox/1.eml\"\n"
	if runtime.GOOS == "windows" {
		t.Skip("shell fake readpst is only used on Unix-like systems")
	}
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	pstPath := filepath.Join(tmp, "backup.pst")
	if err := os.WriteFile(pstPath, []byte("pst"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := pst.Import(pst.Options{PSTPath: pstPath, OutputDir: filepath.Join(tmp, "out")})
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("messages = %#v", result.Messages)
	}
	if result.Messages[0].FolderPath != "Inbox" {
		t.Fatalf("folder path = %q", result.Messages[0].FolderPath)
	}
}

func TestImporterValidatesReadpstAndPSTPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)

	_, err := pst.Import(pst.Options{PSTPath: filepath.Join(tmp, "missing.pst"), OutputDir: filepath.Join(tmp, "out")})
	if err == nil {
		t.Fatal("expected missing readpst or PST path error")
	}
}
