package app

import (
	"os"
	"path/filepath"

	"outlook-pst-mcp/internal/exporter"
	"outlook-pst-mcp/internal/mail"
	"outlook-pst-mcp/internal/pst"
	"outlook-pst-mcp/internal/store"
)

type Service struct {
	workspace string
	store     *store.Store
}

type CreateMessageRequest struct {
	FolderID string
	Message  mail.Message
}

func Open(workspace string) (*Service, error) {
	if err := os.MkdirAll(filepath.Join(workspace, "messages"), 0o755); err != nil {
		return nil, err
	}
	st, err := store.Open(filepath.Join(workspace, "mailbox.db"))
	if err != nil {
		return nil, err
	}
	return &Service{workspace: workspace, store: st}, nil
}

func (s *Service) Close() error {
	return s.store.Close()
}

func (s *Service) EnsureFolder(path string) (store.Folder, error) {
	if folder, found, err := s.store.GetFolderByPath(path); err != nil || found {
		return folder, err
	}
	return s.store.CreateFolder(filepath.Base(path), path, nil)
}

func (s *Service) ImportMailbox(pstPath string) (int, int, error) {
	result, err := pst.Import(pst.Options{PSTPath: pstPath, OutputDir: filepath.Join(s.workspace, "extracted")})
	if err != nil {
		return 0, 0, err
	}
	folders := map[string]store.Folder{}
	for _, extracted := range result.Messages {
		folder, ok := folders[extracted.FolderPath]
		if !ok {
			folder, err = s.EnsureFolder(extracted.FolderPath)
			if err != nil {
				return 0, 0, err
			}
			folders[extracted.FolderPath] = folder
		}
		data, err := os.ReadFile(extracted.Path)
		if err != nil {
			return 0, 0, err
		}
		parsed, err := mail.ParseEML(data)
		if err != nil {
			return 0, 0, err
		}
		if _, err := s.CreateMessage(CreateMessageRequest{FolderID: folder.ID, Message: parsed}); err != nil {
			return 0, 0, err
		}
	}
	return len(folders), len(result.Messages), nil
}

func (s *Service) ListFolders() ([]store.Folder, error) {
	return s.store.ListFolders()
}

func (s *Service) ListMessages(filter store.MessageFilter) ([]store.Message, int, error) {
	return s.store.ListMessages(filter)
}

func (s *Service) GetMessage(id string, includeBody, includeHeaders bool) (mail.Message, error) {
	message, found, err := s.store.GetMessage(id)
	if err != nil {
		return mail.Message{}, err
	}
	if !found {
		return mail.Message{}, os.ErrNotExist
	}
	parsed, err := readMessage(message.EMLPath)
	if err != nil {
		return mail.Message{}, err
	}
	if !includeBody {
		parsed.BodyText = ""
	}
	if !includeHeaders {
		parsed.Headers = nil
	}
	return parsed, nil
}

func (s *Service) CreateMessage(request CreateMessageRequest) (store.Message, error) {
	data, err := mail.BuildEML(request.Message)
	if err != nil {
		return store.Message{}, err
	}
	created, err := s.store.CreateMessage(store.Message{
		FolderID: request.FolderID,
		Subject:  request.Message.Subject,
		FromAddr: request.Message.From,
		ToAddrs:  request.Message.To,
		CcAddrs:  request.Message.Cc,
		EMLPath:  "pending",
	})
	if err != nil {
		return store.Message{}, err
	}
	path := filepath.Join(s.workspace, "messages", created.ID+".eml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return store.Message{}, err
	}
	if err := s.store.UpdateMessage(created.ID, store.MessagePatch{EMLPath: &path}); err != nil {
		return store.Message{}, err
	}
	created.EMLPath = path
	return created, nil
}

func (s *Service) UpdateMessage(id string, patch mail.Patch) error {
	message, found, err := s.store.GetMessage(id)
	if err != nil {
		return err
	}
	if !found {
		return os.ErrNotExist
	}
	data, err := os.ReadFile(message.EMLPath)
	if err != nil {
		return err
	}
	patched, err := mail.PatchEML(data, patch)
	if err != nil {
		return err
	}
	if err := os.WriteFile(message.EMLPath, patched, 0o644); err != nil {
		return err
	}
	parsed, err := mail.ParseEML(patched)
	if err != nil {
		return err
	}
	return s.store.UpdateMessage(id, store.MessagePatch{
		Subject:  &parsed.Subject,
		FromAddr: &parsed.From,
		ToAddrs:  &parsed.To,
		CcAddrs:  &parsed.Cc,
	})
}

func (s *Service) DeleteMessage(id string) error {
	return s.store.DeleteMessage(id)
}

func (s *Service) MoveMessage(id, folderID string) error {
	return s.store.MoveMessage(id, folderID)
}

func (s *Service) ExportEML(outputDir string, includeDeleted bool) error {
	folders, err := s.store.ListFolders()
	if err != nil {
		return err
	}
	var messages []store.Message
	for offset := 0; ; offset += 200 {
		page, total, err := s.store.ListMessages(store.MessageFilter{IncludeDeleted: true, Limit: 200, Offset: offset})
		if err != nil {
			return err
		}
		messages = append(messages, page...)
		if len(messages) >= total || len(page) == 0 {
			break
		}
	}
	return exporter.Export(exporter.Request{OutputDir: outputDir, Folders: folders, Messages: messages, IncludeDeleted: includeDeleted})
}

func readMessage(path string) (mail.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return mail.Message{}, err
	}
	return mail.ParseEML(data)
}
