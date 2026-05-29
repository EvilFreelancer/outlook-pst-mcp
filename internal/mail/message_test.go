package mail_test

import (
	"strings"
	"testing"
	"time"

	"outlook-pst-mcp/internal/mail"
)

func TestParseEMLExtractsMetadataAndPlainTextBody(t *testing.T) {
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>, Carol <carol@example.com>",
		"Cc: Dan <dan@example.com>",
		"Subject: Quarterly Update",
		"Date: Mon, 02 Jan 2006 15:04:05 +0000",
		"Message-ID: <abc@example.com>",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello from the archive.",
	}, "\r\n")

	msg, err := mail.ParseEML([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}

	if msg.Subject != "Quarterly Update" {
		t.Fatalf("subject = %q", msg.Subject)
	}
	if msg.From != "Alice <alice@example.com>" {
		t.Fatalf("from = %q", msg.From)
	}
	if len(msg.To) != 2 || msg.To[0] != "Bob <bob@example.com>" || msg.To[1] != "Carol <carol@example.com>" {
		t.Fatalf("to = %#v", msg.To)
	}
	if len(msg.Cc) != 1 || msg.Cc[0] != "Dan <dan@example.com>" {
		t.Fatalf("cc = %#v", msg.Cc)
	}
	if msg.Date == nil || !msg.Date.Equal(time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)) {
		t.Fatalf("date = %#v", msg.Date)
	}
	if msg.BodyText != "Hello from the archive." {
		t.Fatalf("body = %q", msg.BodyText)
	}
	if msg.Headers.Get("Message-ID") != "<abc@example.com>" {
		t.Fatalf("message id header = %q", msg.Headers.Get("Message-ID"))
	}
}

func TestBuildAndPatchEML(t *testing.T) {
	built, err := mail.BuildEML(mail.Message{
		Subject:  "Draft",
		From:     "me@example.com",
		To:       []string{"you@example.com"},
		Cc:       []string{"copy@example.com"},
		BodyText: "Initial body",
		Headers:  map[string][]string{"X-Source": {"test"}},
	})
	if err != nil {
		t.Fatalf("BuildEML returned error: %v", err)
	}

	parsed, err := mail.ParseEML(built)
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.Subject != "Draft" || parsed.BodyText != "Initial body" || parsed.Headers.Get("X-Source") != "test" {
		t.Fatalf("parsed built message = %#v", parsed)
	}

	patched, err := mail.PatchEML(built, mail.Patch{
		Subject:  ptr("Updated"),
		To:       &[]string{"new@example.com"},
		BodyText: ptr("Updated body"),
		Headers:  map[string][]string{"X-Reviewed": {"yes"}},
	})
	if err != nil {
		t.Fatalf("PatchEML returned error: %v", err)
	}

	updated, err := mail.ParseEML(patched)
	if err != nil {
		t.Fatalf("ParseEML patched returned error: %v", err)
	}
	if updated.Subject != "Updated" {
		t.Fatalf("updated subject = %q", updated.Subject)
	}
	if len(updated.To) != 1 || updated.To[0] != "new@example.com" {
		t.Fatalf("updated to = %#v", updated.To)
	}
	if updated.BodyText != "Updated body" {
		t.Fatalf("updated body = %q", updated.BodyText)
	}
	if updated.Headers.Get("X-Reviewed") != "yes" {
		t.Fatalf("updated header = %q", updated.Headers.Get("X-Reviewed"))
	}
}

func ptr(v string) *string {
	return &v
}
