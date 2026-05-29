package mail_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EvilFreelancer/outlook-pst-mcp/internal/mail"
)

func TestParseEMLMetadataFileReadsHeadersOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.eml")
	raw := "Date: Mon, 02 Jan 2006 15:04:05 +0000\r\nSubject: Hello\r\nFrom: a@example.com\r\nTo: b@example.com\r\n\r\n" + string(make([]byte, 1024*1024))
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := mail.ParseEMLMetadataFile(path)
	if err != nil {
		t.Fatalf("ParseEMLMetadataFile returned error: %v", err)
	}
	if meta.Subject != "Hello" || meta.From != "a@example.com" {
		t.Fatalf("meta = %#v", meta)
	}
	want := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC).Unix()
	if meta.Unix() != want {
		t.Fatalf("unix = %d want %d", meta.Unix(), want)
	}
}
