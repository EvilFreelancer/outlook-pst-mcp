package exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"outlook-pst-mcp/internal/store"
)

type Request struct {
	OutputDir      string
	Folders        []store.Folder
	Messages       []store.Message
	IncludeDeleted bool
}

type Manifest struct {
	ExportedAt          time.Time `json:"exported_at"`
	FolderCount         int       `json:"folder_count"`
	MessageCount        int       `json:"message_count"`
	SkippedDeletedCount int       `json:"skipped_deleted_count"`
}

func Export(request Request) error {
	if err := os.MkdirAll(request.OutputDir, 0o755); err != nil {
		return err
	}
	folders := map[string]store.Folder{}
	for _, folder := range request.Folders {
		folders[folder.ID] = folder
		if err := os.MkdirAll(filepath.Join(request.OutputDir, safePath(folder.Path)), 0o755); err != nil {
			return err
		}
	}

	manifest := Manifest{ExportedAt: time.Now().UTC(), FolderCount: len(request.Folders)}
	for _, message := range request.Messages {
		if message.Deleted && !request.IncludeDeleted {
			manifest.SkippedDeletedCount++
			continue
		}
		folder, ok := folders[message.FolderID]
		if !ok {
			return fmt.Errorf("message %q references missing folder %q", message.ID, message.FolderID)
		}
		dst := filepath.Join(request.OutputDir, safePath(folder.Path), message.ID+".eml")
		if err := copyFile(message.EMLPath, dst); err != nil {
			return err
		}
		manifest.MessageCount++
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(request.OutputDir, "manifest.json"), append(data, '\n'), 0o644)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func safePath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		return "Root"
	}
	return filepath.Join(clean...)
}
