package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"outlook-pst-mcp/internal/app"
	"outlook-pst-mcp/internal/mail"
	"outlook-pst-mcp/internal/store"
)

type Server struct {
	service *app.Service
}

type ToolResult struct {
	Content any `json:"content"`
}

func New(service *app.Service) *Server {
	return &Server{service: service}
}

func (s *Server) ToolNames() []string {
	return []string{
		"import_mailbox",
		"list_folders",
		"list_messages",
		"get_message",
		"create_message",
		"update_message",
		"delete_message",
		"move_message",
		"export_eml",
	}
}

func (s *Server) CallTool(ctx context.Context, name string, args json.RawMessage) (ToolResult, error) {
	_ = ctx
	if s.service == nil && name != "tools/list" {
		if !knownTool(name, s.ToolNames()) {
			return ToolResult{}, fmt.Errorf("unknown tool %q", name)
		}
		return ToolResult{}, fmt.Errorf("service is not configured")
	}
	switch name {
	case "import_mailbox":
		var req struct {
			PSTPath string `json:"pst_path"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		folders, messages, err := s.service.ImportMailbox(req.PSTPath)
		return ToolResult{Content: map[string]int{"folder_count": folders, "message_count": messages}}, err
	case "list_folders":
		folders, err := s.service.ListFolders()
		return ToolResult{Content: folders}, err
	case "list_messages":
		var req struct {
			FolderID       string `json:"folder_id"`
			Query          string `json:"query"`
			Limit          int    `json:"limit"`
			Offset         int    `json:"offset"`
			IncludeDeleted bool   `json:"include_deleted"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		messages, total, err := s.service.ListMessages(store.MessageFilter{
			FolderID:       req.FolderID,
			Query:          req.Query,
			Limit:          req.Limit,
			Offset:         req.Offset,
			IncludeDeleted: req.IncludeDeleted,
		})
		return ToolResult{Content: map[string]any{"messages": messages, "total": total}}, err
	case "get_message":
		var req struct {
			MessageID      string `json:"message_id"`
			IncludeBody    bool   `json:"include_body"`
			IncludeHeaders bool   `json:"include_headers"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		message, err := s.service.GetMessage(req.MessageID, req.IncludeBody, req.IncludeHeaders)
		return ToolResult{Content: map[string]any{
			"subject":   message.Subject,
			"from":      message.From,
			"to":        message.To,
			"cc":        message.Cc,
			"body_text": message.BodyText,
			"headers":   message.Headers,
		}}, err
	case "create_message":
		var req struct {
			FolderID string            `json:"folder_id"`
			Subject  string            `json:"subject"`
			From     string            `json:"from"`
			To       []string          `json:"to"`
			Cc       []string          `json:"cc"`
			BodyText string            `json:"body_text"`
			Headers  map[string]string `json:"headers"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		created, err := s.service.CreateMessage(app.CreateMessageRequest{
			FolderID: req.FolderID,
			Message: mail.Message{
				Subject:  req.Subject,
				From:     req.From,
				To:       req.To,
				Cc:       req.Cc,
				BodyText: req.BodyText,
				Headers:  toHeader(req.Headers),
			},
		})
		return ToolResult{Content: map[string]any{"id": created.ID}}, err
	case "update_message":
		var req struct {
			MessageID string            `json:"message_id"`
			Subject   *string           `json:"subject"`
			From      *string           `json:"from"`
			To        *[]string         `json:"to"`
			Cc        *[]string         `json:"cc"`
			BodyText  *string           `json:"body_text"`
			Headers   map[string]string `json:"headers"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		err := s.service.UpdateMessage(req.MessageID, mail.Patch{
			Subject:  req.Subject,
			From:     req.From,
			To:       req.To,
			Cc:       req.Cc,
			BodyText: req.BodyText,
			Headers:  toHeader(req.Headers),
		})
		return ToolResult{Content: map[string]string{"status": "ok"}}, err
	case "delete_message":
		var req struct {
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Content: map[string]string{"status": "ok"}}, s.service.DeleteMessage(req.MessageID)
	case "move_message":
		var req struct {
			MessageID string `json:"message_id"`
			FolderID  string `json:"folder_id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Content: map[string]string{"status": "ok"}}, s.service.MoveMessage(req.MessageID, req.FolderID)
	case "export_eml":
		var req struct {
			OutputDir      string `json:"output_dir"`
			IncludeDeleted bool   `json:"include_deleted"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Content: map[string]string{"status": "ok"}}, s.service.ExportEML(req.OutputDir, req.IncludeDeleted)
	default:
		if knownTool(name, s.ToolNames()) {
			return ToolResult{}, fmt.Errorf("tool %q is not implemented in this handler yet", name)
		}
		return ToolResult{}, fmt.Errorf("unknown tool %q", name)
	}
}

func toHeader(values map[string]string) mail.Header {
	if len(values) == 0 {
		return nil
	}
	headers := mail.Header{}
	for key, value := range values {
		headers[key] = []string{value}
	}
	return headers
}

func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	reader := bufio.NewReader(r)
	for {
		payload, err := readFrame(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if err := writeFrame(w, rpcError{JSONRPC: "2.0", ID: nil, Error: rpcErrorBody{Code: -32700, Message: err.Error()}}); err != nil {
				return err
			}
			continue
		}
		resp := s.handleRPC(ctx, req)
		if resp != nil {
			if err := writeFrame(w, resp); err != nil {
				return err
			}
		}
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result"`
}

type rpcError struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      any          `json:"id"`
	Error   rpcErrorBody `json:"error"`
}

type rpcErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handleRPC(ctx context.Context, req rpcRequest) any {
	switch req.Method {
	case "initialize":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "outlook-pst-mcp",
				"version": "0.1.0",
			},
			"capabilities": map[string]any{"tools": map[string]any{}},
		}}
	case "notifications/initialized":
		return nil
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.ToolNames()))
		for _, name := range s.ToolNames() {
			tools = append(tools, map[string]any{
				"name":        name,
				"description": toolDescription(name),
				"inputSchema": map[string]any{"type": "object"},
			})
		}
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": tools}}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return rpcError{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorBody{Code: -32602, Message: err.Error()}}
		}
		result, err := s.CallTool(ctx, params.Name, params.Arguments)
		if err != nil {
			return rpcError{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorBody{Code: -32000, Message: err.Error()}}
		}
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"content": []map[string]any{{
			"type": "text",
			"text": mustJSON(result.Content),
		}}}}
	default:
		return rpcError{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorBody{Code: -32601, Message: "method not found"}}
	}
}

func readFrame(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			length = parsed
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeFrame(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func toolDescription(name string) string {
	switch name {
	case "import_mailbox":
		return "Import a PST file into the editable workspace through readpst."
	case "list_folders":
		return "List indexed mailbox folders."
	case "list_messages":
		return "List indexed message summaries with filtering and pagination."
	case "get_message":
		return "Read a message from the editable workspace."
	case "create_message":
		return "Create a new EML message in a folder."
	case "update_message":
		return "Patch message headers, recipients, subject, or plain text body."
	case "delete_message":
		return "Soft-delete a message from export output."
	case "move_message":
		return "Move a message to another folder."
	case "export_eml":
		return "Export the current workspace state as an EML folder tree."
	default:
		return name
	}
}

func knownTool(name string, names []string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}
