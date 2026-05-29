package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/EvilFreelancer/outlook-pst-mcp/internal/mail"
	"github.com/EvilFreelancer/outlook-pst-mcp/internal/pst"
	"github.com/EvilFreelancer/outlook-pst-mcp/internal/store"
)

func (s *Service) ImportMailbox(pstPath string) (int, int, int, error) {
	result, err := pst.Import(pst.Options{PSTPath: pstPath, OutputDir: filepath.Join(s.workspace, "extracted")})
	if err != nil {
		return 0, 0, 0, err
	}

	folders := map[string]store.Folder{}
	usedIDs := map[string]struct{}{}
	batch := make([]store.Message, 0, len(result.Messages))
	skipped := 0
	fallbackOrder := int64(0)

	for _, extracted := range result.Messages {
		folder, ok := folders[extracted.FolderPath]
		if !ok {
			folder, err = s.EnsureFolder(extracted.FolderPath)
			if err != nil {
				return 0, 0, 0, err
			}
			folders[extracted.FolderPath] = folder
		}

		meta, metaErr := mail.ParseEMLMetadataFile(extracted.Path)
		messageAt := meta.Unix()
		if messageAt == 0 {
			if info, statErr := os.Stat(extracted.Path); statErr == nil {
				messageAt = info.ModTime().Unix()
			} else {
				fallbackOrder++
				messageAt = time.Now().Unix() - fallbackOrder
			}
		}

		id, err := s.allocateImportID(messageAt, usedIDs)
		if err != nil {
			return 0, 0, skipped, err
		}
		dest := filepath.Join(s.workspace, "messages", id+".eml")
		if err := copyFile(extracted.Path, dest); err != nil {
			skipped++
			continue
		}

		subject := meta.Subject
		from := meta.From
		var to, cc []string
		if metaErr == nil {
			to, cc = meta.To, meta.Cc
		}

		batch = append(batch, store.Message{
			ID:        id,
			FolderID:  folder.ID,
			Subject:   subject,
			FromAddr:  from,
			ToAddrs:   to,
			CcAddrs:   cc,
			EMLPath:   dest,
			MessageAt: messageAt,
		})
	}

	if err := s.store.InsertMessages(batch); err != nil {
		return 0, 0, skipped, err
	}
	return len(folders), len(batch), skipped, nil
}

func (s *Service) allocateImportID(messageAt int64, used map[string]struct{}) (string, error) {
	for suffix := 0; ; suffix++ {
		id := strconv.FormatInt(messageAt, 10)
		if suffix > 0 {
			id = fmt.Sprintf("%d_%d", messageAt, suffix)
		}
		if _, exists := used[id]; exists {
			continue
		}
		if _, found, err := s.store.GetMessage(id); err != nil {
			return "", err
		} else if found {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.workspace, "messages", id+".eml")); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		used[id] = struct{}{}
		return id, nil
	}
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
