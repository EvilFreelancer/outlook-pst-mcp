package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExplicit(t *testing.T) {
	got, err := Resolve("/tmp/mailbox")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/mailbox" {
		t.Fatalf("got %q want /tmp/mailbox", got)
	}
}

func TestResolveDefaultUsesCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, DefaultDir)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
