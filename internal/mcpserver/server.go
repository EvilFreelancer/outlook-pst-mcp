package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"outlook-pst-mcp/internal/app"
	"outlook-pst-mcp/internal/mail"
	"outlook-pst-mcp/internal/store"
)

type Server struct {
	service   *app.Service
	workspace string
	mu        sync.Mutex
}

type ToolResult struct {
	Content any `json:"content"`
}

func New(service *app.Service) *Server {
	return &Server{service: service}
}

func NewLazy(workspace string) *Server {
	return &Server{workspace: strings.TrimSpace(workspace)}
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service == nil {
		return nil
	}
	err := s.service.Close()
	s.service = nil
	return err
}

func (s *Server) requireService() (*app.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.service != nil {
		return s.service, nil
	}
	if s.workspace == "" {
		return nil, fmt.Errorf("service is not configured")
	}
	svc, err := app.Open(s.workspace)
	if err != nil {
		return nil, err
	}
	s.service = svc
	return s.service, nil
}

func (s *Server) ToolNames() []string {
	return []string{
		"import_pst",
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
	if !knownTool(name, s.ToolNames()) {
		return ToolResult{}, fmt.Errorf("unknown tool %q", name)
	}
	if name == "import_pst" {
		return s.callImportPST(args)
	}
	svc, err := s.requireService()
	if err != nil {
		return ToolResult{}, err
	}
	switch name {
	case "list_folders":
		folders, err := svc.ListFolders()
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
		messages, total, err := svc.ListMessages(store.MessageFilter{
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
		message, err := svc.GetMessage(req.MessageID, req.IncludeBody, req.IncludeHeaders)
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
		created, err := svc.CreateMessage(app.CreateMessageRequest{
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
		err := svc.UpdateMessage(req.MessageID, mail.Patch{
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
		return ToolResult{Content: map[string]string{"status": "ok"}}, svc.DeleteMessage(req.MessageID)
	case "move_message":
		var req struct {
			MessageID string `json:"message_id"`
			FolderID  string `json:"folder_id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Content: map[string]string{"status": "ok"}}, svc.MoveMessage(req.MessageID, req.FolderID)
	case "export_eml":
		var req struct {
			OutputDir      string `json:"output_dir"`
			IncludeDeleted bool   `json:"include_deleted"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Content: map[string]string{"status": "ok"}}, svc.ExportEML(req.OutputDir, req.IncludeDeleted)
	default:
		return ToolResult{}, fmt.Errorf("tool %q is not implemented in this handler yet", name)
	}
}

func (s *Server) callImportPST(args json.RawMessage) (ToolResult, error) {
	var req struct {
		PSTPath string `json:"pst_path"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return ToolResult{}, err
	}
	pstPath := strings.TrimSpace(req.PSTPath)
	if pstPath == "" {
		return ToolResult{}, fmt.Errorf("pst_path is required")
	}
	svc, err := s.requireService()
	if err != nil {
		return ToolResult{}, err
	}
	folders, messages, skipped, err := svc.ImportMailbox(pstPath)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Content: map[string]any{
		"workspace":     s.workspace,
		"folder_count":  folders,
		"message_count": messages,
		"skipped_count": skipped,
	}}, nil
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
	mode := frameModeUnknown
	for {
		payload, requestMode, err := readMessage(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if mode == frameModeUnknown {
			mode = requestMode
		}
		var req rpcRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if err := writeMessage(w, mode, rpcError{JSONRPC: "2.0", ID: nil, Error: rpcErrorBody{Code: -32700, Message: err.Error()}}); err != nil {
				return err
			}
			continue
		}
		resp := s.handleRPC(ctx, req)
		if resp != nil {
			if err := writeMessage(w, mode, resp); err != nil {
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
		proto := "2024-11-05"
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		if params.ProtocolVersion != "" && isSupportedProtocolVersion(params.ProtocolVersion) {
			proto = params.ProtocolVersion
		}
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": proto,
			"serverInfo": map[string]string{
				"name":    "outlook-pst-mcp",
				"version": "0.1.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
		}}
	case "notifications/initialized":
		return nil
	case "ping":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "resources/list":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"resources": []any{}}}
	case "prompts/list":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"prompts": []any{}}}
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.ToolNames()))
		for _, name := range s.ToolNames() {
			tools = append(tools, map[string]any{
				"name":        name,
				"title":       toolTitle(name),
				"description": toolDescription(name),
				"inputSchema": toolInputSchema(name),
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

func isSupportedProtocolVersion(version string) bool {
	switch version {
	case "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05", "2024-10-07":
		return true
	default:
		return false
	}
}

type frameMode int

const (
	frameModeUnknown frameMode = iota
	frameModeContentLength
	frameModeLine
)

func readMessage(reader *bufio.Reader) ([]byte, frameMode, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, frameModeUnknown, err
		}
		line = strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return []byte(line), frameModeLine, nil
		}
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
				return nil, frameModeUnknown, err
			}
			length = parsed
		}
	}
	if length < 0 {
		return nil, frameModeUnknown, fmt.Errorf("missing Content-Length header")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, frameModeUnknown, err
	}
	return payload, frameModeContentLength, nil
}

func writeMessage(w io.Writer, mode frameMode, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if mode == frameModeLine {
		if _, err := w.Write(append(payload, '\n')); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
			return err
		}
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func toolTitle(name string) string {
	switch name {
	case "import_pst":
		return "Import PST"
	case "list_folders":
		return "List Folders"
	case "list_messages":
		return "List Messages"
	case "get_message":
		return "Get Message"
	case "create_message":
		return "Create Message"
	case "update_message":
		return "Update Message"
	case "delete_message":
		return "Delete Message"
	case "move_message":
		return "Move Message"
	case "export_eml":
		return "Export EML"
	default:
		return name
	}
}

func toolDescription(name string) string {
	switch name {
	case "import_pst":
		return "Import messages from a PST file into the workspace using readpst."
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

func toolInputSchema(name string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
	props := schema["properties"].(map[string]any)

	switch name {
	case "import_pst":
		props["pst_path"] = stringProperty("Path to the .pst file to import.")
		schema["required"] = []string{"pst_path"}
		return schema
	case "list_folders":
		schema["required"] = []string{}
		return schema
	case "list_messages":
		props["folder_id"] = stringProperty("Optional folder ID filter.")
		props["query"] = stringProperty("Optional case-insensitive search text.")
		props["limit"] = map[string]any{"type": "number", "description": "Page size, default 50, maximum 200."}
		props["offset"] = map[string]any{"type": "number", "description": "Page offset, default 0."}
		props["include_deleted"] = boolProperty("Include soft-deleted messages.")
		return schema
	case "get_message":
		props["message_id"] = stringProperty("Message ID to read.")
		props["include_body"] = boolProperty("Include plain text body.")
		props["include_headers"] = boolProperty("Include raw headers map.")
		schema["required"] = []string{"message_id"}
		return schema
	case "create_message":
		props["folder_id"] = stringProperty("Target folder ID.")
		props["subject"] = stringProperty("Message subject.")
		props["from"] = stringProperty("Sender address.")
		props["to"] = stringArrayProperty("Recipient addresses.")
		props["cc"] = stringArrayProperty("CC recipient addresses.")
		props["body_text"] = stringProperty("Plain text body.")
		props["headers"] = stringMapProperty("Optional extra headers.")
		schema["required"] = []string{"folder_id", "subject", "from", "to", "body_text"}
		return schema
	case "update_message":
		props["message_id"] = stringProperty("Message ID to update.")
		props["subject"] = stringProperty("New subject.")
		props["from"] = stringProperty("New sender address.")
		props["to"] = stringArrayProperty("New recipient addresses.")
		props["cc"] = stringArrayProperty("New CC recipient addresses.")
		props["body_text"] = stringProperty("New plain text body.")
		props["headers"] = stringMapProperty("Replacement headers map.")
		schema["required"] = []string{"message_id"}
		return schema
	case "delete_message":
		props["message_id"] = stringProperty("Message ID to soft-delete.")
		schema["required"] = []string{"message_id"}
		return schema
	case "move_message":
		props["message_id"] = stringProperty("Message ID to move.")
		props["folder_id"] = stringProperty("Destination folder ID.")
		schema["required"] = []string{"message_id", "folder_id"}
		return schema
	case "export_eml":
		props["output_dir"] = stringProperty("Directory for exported EML tree.")
		props["include_deleted"] = boolProperty("Include soft-deleted messages.")
		schema["required"] = []string{"output_dir"}
		return schema
	default:
		return schema
	}
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func boolProperty(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func stringArrayProperty(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string"},
	}
}

func stringMapProperty(description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          description,
		"additionalProperties": map[string]any{"type": "string"},
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
