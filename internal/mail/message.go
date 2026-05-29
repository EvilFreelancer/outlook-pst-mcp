package mail

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	stdmail "net/mail"
	"strings"
	"time"
)

type Header = stdmail.Header

type Message struct {
	Subject  string
	From     string
	To       []string
	Cc       []string
	Date     *time.Time
	BodyText string
	Headers  Header
}

type Patch struct {
	Subject  *string
	From     *string
	To       *[]string
	Cc       *[]string
	BodyText *string
	Headers  map[string][]string
}

func ParseEML(data []byte) (Message, error) {
	msg, err := stdmail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return Message{}, err
	}
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return Message{}, err
	}

	subject := msg.Header.Get("Subject")
	if decoded, err := (&mime.WordDecoder{}).DecodeHeader(subject); err == nil {
		subject = decoded
	}

	var date *time.Time
	if raw := msg.Header.Get("Date"); raw != "" {
		if parsed, err := stdmail.ParseDate(raw); err == nil {
			utc := parsed.UTC()
			date = &utc
		}
	}

	return Message{
		Subject:  subject,
		From:     msg.Header.Get("From"),
		To:       splitAddressHeader(msg.Header.Get("To")),
		Cc:       splitAddressHeader(msg.Header.Get("Cc")),
		Date:     date,
		BodyText: strings.TrimRight(string(body), "\r\n"),
		Headers:  msg.Header,
	}, nil
}

func BuildEML(message Message) ([]byte, error) {
	var buf bytes.Buffer
	headers := Header{}
	for key, values := range message.Headers {
		copied := append([]string(nil), values...)
		headers[key] = copied
	}
	setHeader(headers, "Subject", message.Subject)
	setHeader(headers, "From", message.From)
	setHeader(headers, "To", strings.Join(message.To, ", "))
	if len(message.Cc) > 0 {
		setHeader(headers, "Cc", strings.Join(message.Cc, ", "))
	}
	if message.Date != nil {
		setHeader(headers, "Date", message.Date.Format(time.RFC1123Z))
	}
	setHeader(headers, "Content-Type", "text/plain; charset=utf-8")

	for key, values := range headers {
		for _, value := range values {
			if value == "" {
				continue
			}
			if _, err := fmt.Fprintf(&buf, "%s: %s\r\n", key, value); err != nil {
				return nil, err
			}
		}
	}
	if _, err := buf.WriteString("\r\n"); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(message.BodyText); err != nil {
		return nil, err
	}
	if !strings.HasSuffix(message.BodyText, "\n") {
		if _, err := buf.WriteString("\r\n"); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func PatchEML(data []byte, patch Patch) ([]byte, error) {
	message, err := ParseEML(data)
	if err != nil {
		return nil, err
	}
	if patch.Subject != nil {
		message.Subject = *patch.Subject
	}
	if patch.From != nil {
		message.From = *patch.From
	}
	if patch.To != nil {
		message.To = append([]string(nil), (*patch.To)...)
	}
	if patch.Cc != nil {
		message.Cc = append([]string(nil), (*patch.Cc)...)
	}
	if patch.BodyText != nil {
		message.BodyText = *patch.BodyText
	}
	if message.Headers == nil {
		message.Headers = Header{}
	}
	for key, values := range patch.Headers {
		message.Headers[key] = append([]string(nil), values...)
	}
	return BuildEML(message)
}

func splitAddressHeader(raw string) []string {
	if raw == "" {
		return nil
	}
	addresses, err := stdmail.ParseAddressList(raw)
	if err != nil {
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address.Name != "" {
			out = append(out, address.Name+" <"+address.Address+">")
			continue
		}
		out = append(out, address.Address)
	}
	return out
}

func setHeader(headers Header, key, value string) {
	if value == "" {
		delete(headers, key)
		return
	}
	headers[key] = []string{value}
}
