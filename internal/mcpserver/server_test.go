package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"outlook-pst-mcp/internal/app"
	"outlook-pst-mcp/internal/mcpserver"
)

func TestServerListsRequiredToolsAndRejectsUnknownTool(t *testing.T) {
	server := mcpserver.New(nil)
	tools := server.ToolNames()

	required := []string{
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
	for _, name := range required {
		if !contains(tools, name) {
			t.Fatalf("tool %q missing from %#v", name, tools)
		}
	}

	if _, err := server.CallTool(context.Background(), "missing", nil); err == nil {
		t.Fatal("expected unknown tool error")
	}
}

func TestServerDispatchesCRUDTools(t *testing.T) {
	svc, err := app.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer svc.Close()
	folder, err := svc.EnsureFolder("Inbox")
	if err != nil {
		t.Fatalf("EnsureFolder returned error: %v", err)
	}
	archive, err := svc.EnsureFolder("Archive")
	if err != nil {
		t.Fatalf("EnsureFolder archive returned error: %v", err)
	}
	server := mcpserver.New(svc)

	created := callTool(t, server, "create_message", map[string]any{
		"folder_id": folder.ID,
		"subject":   "Created",
		"from":      "a@example.com",
		"to":        []string{"b@example.com"},
		"body_text": "Body",
	})
	messageID, ok := created.Content.(map[string]any)["id"].(string)
	if !ok || messageID == "" {
		t.Fatalf("created result = %#v", created.Content)
	}

	callTool(t, server, "update_message", map[string]any{
		"message_id": messageID,
		"subject":    "Updated",
	})
	callTool(t, server, "move_message", map[string]any{
		"message_id": messageID,
		"folder_id":  archive.ID,
	})

	got := callTool(t, server, "get_message", map[string]any{
		"message_id":   messageID,
		"include_body": true,
	})
	if got.Content.(map[string]any)["subject"] != "Updated" {
		t.Fatalf("get result = %#v", got.Content)
	}

	listed := callTool(t, server, "list_messages", map[string]any{
		"folder_id": archive.ID,
	})
	if listed.Content.(map[string]any)["total"].(int) != 1 {
		t.Fatalf("list result = %#v", listed.Content)
	}

	callTool(t, server, "delete_message", map[string]any{"message_id": messageID})
	callTool(t, server, "export_eml", map[string]any{"output_dir": filepath.Join(t.TempDir(), "export")})
}

func TestServeHandlesInitializeAndToolsListFrames(t *testing.T) {
	server := mcpserver.New(nil)
	input := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	var output bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	text := output.String()
	if !strings.Contains(text, `"id":1`) || !strings.Contains(text, `"protocolVersion"`) {
		t.Fatalf("initialize response missing: %s", text)
	}
	if !strings.Contains(text, `"id":2`) || !strings.Contains(text, `"import_mailbox"`) {
		t.Fatalf("tools/list response missing: %s", text)
	}
}

func callTool(t *testing.T, server *mcpserver.Server, name string, args map[string]any) mcpserver.ToolResult {
	t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	result, err := server.CallTool(context.Background(), name, data)
	if err != nil {
		t.Fatalf("CallTool(%s) returned error: %v", name, err)
	}
	return result
}

func frame(payload string) string {
	return "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n" + payload
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
