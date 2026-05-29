package mail

import (
	"io"
	stdmail "net/mail"
	"os"
	"time"
)

const defaultMetadataReadLimit = 256 * 1024

// Metadata holds indexed header fields without reading the message body.
type Metadata struct {
	Subject string
	From    string
	To      []string
	Cc      []string
	Date    *time.Time
}

func (m Metadata) Unix() int64 {
	if m.Date != nil {
		return m.Date.Unix()
	}
	return 0
}

// ParseEMLMetadataFile reads only the beginning of a file to extract headers.
func ParseEMLMetadataFile(path string) (Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return Metadata{}, err
	}
	defer file.Close()
	return ParseEMLMetadata(io.LimitReader(file, defaultMetadataReadLimit))
}

func ParseEMLMetadata(r io.Reader) (Metadata, error) {
	msg, err := stdmail.ReadMessage(r)
	if err != nil {
		return Metadata{}, err
	}
	_, _ = io.Copy(io.Discard, msg.Body)

	subject := msg.Header.Get("Subject")
	var date *time.Time
	if raw := msg.Header.Get("Date"); raw != "" {
		if parsed, err := stdmail.ParseDate(raw); err == nil {
			utc := parsed.UTC()
			date = &utc
		}
	}

	return Metadata{
		Subject: subject,
		From:    msg.Header.Get("From"),
		To:      splitAddressHeader(msg.Header.Get("To")),
		Cc:      splitAddressHeader(msg.Header.Get("Cc")),
		Date:    date,
	}, nil
}
