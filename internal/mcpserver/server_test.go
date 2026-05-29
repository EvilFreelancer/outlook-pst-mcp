package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"outlook-pst-mcp/internal/app"
	"outlook-pst-mcp/internal/mcpserver"
	"outlook-pst-mcp/internal/store"
)

func TestServerListsRequiredToolsAndRejectsUnknownTool(t *testing.T) {
	server := mcpserver.New(nil)
	tools := server.ToolNames()

	required := []string{
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
	input := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`) +
		frame(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	var output bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	text := output.String()
	if !strings.Contains(text, `"id":1`) || !strings.Contains(text, `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize response missing: %s", text)
	}
	if !strings.Contains(text, `"id":2`) || !strings.Contains(text, `"list_folders"`) {
		t.Fatalf("tools/list response missing: %s", text)
	}
	if strings.Contains(text, `"$schema"`) {
		t.Fatalf("tools/list must not include $schema in inputSchema: %s", text)
	}
}

func TestServeHandlesLineDelimitedInitializeAndToolsList(t *testing.T) {
	server := mcpserver.New(nil)
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	var output bytes.Buffer

	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("line-delimited output line count = %d, output: %q", len(lines), output.String())
	}
	if strings.Contains(output.String(), "Content-Length:") {
		t.Fatalf("line-delimited response must not include Content-Length headers: %q", output.String())
	}
	if !strings.Contains(lines[0], `"id":1`) || !strings.Contains(lines[0], `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("initialize line missing: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"id":2`) || !strings.Contains(lines[1], `"list_folders"`) {
		t.Fatalf("tools/list line missing: %s", lines[1])
	}
}

func TestListFoldersOnEmptyLazyWorkspace(t *testing.T) {
	server := mcpserver.NewLazy(t.TempDir())
	result := callTool(t, server, "list_folders", map[string]any{})
	folders, ok := result.Content.([]store.Folder)
	if !ok {
		t.Fatalf("list_folders content = %#v", result.Content)
	}
	if len(folders) != 0 {
		t.Fatalf("folders = %#v, want empty", folders)
	}
}

func TestImportPSTViaMCPTool(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake readpst is only used on Unix-like systems")
	}
	workspace := t.TempDir()
	bin := filepath.Join(workspace, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(bin, "readpst")
	body := "#!/bin/sh\nout=\"\"\nwhile [ $# -gt 0 ]; do case \"$1\" in -o) out=\"$2\"; shift 2;; *) shift;; esac; done\nmkdir -p \"$out/Inbox\"\nprintf 'Date: Mon, 02 Jan 2006 15:04:05 +0000\\r\\nSubject: Imported\\r\\nFrom: a@example.com\\r\\nTo: b@example.com\\r\\n\\r\\nBody\\r\\n' > \"$out/Inbox/1.eml\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	pstPath := filepath.Join(workspace, "backup.pst")
	if err := os.WriteFile(pstPath, []byte("pst"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := mcpserver.NewLazy(workspace)
	imported := callTool(t, server, "import_pst", map[string]any{"pst_path": pstPath})
	stats, ok := imported.Content.(map[string]any)
	if !ok {
		t.Fatalf("import result = %#v", imported.Content)
	}
	if stats["message_count"] != 1 || stats["folder_count"] != 1 {
		t.Fatalf("import stats = %#v", stats)
	}

	listed := callTool(t, server, "list_messages", map[string]any{"limit": 10})
	if listed.Content.(map[string]any)["total"].(int) != 1 {
		t.Fatalf("list result = %#v", listed.Content)
	}
}

func TestServeRespondsToInitializeBeforeWorkspaceOpen(t *testing.T) {
	server := mcpserver.NewLazy(t.TempDir())
	input := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	var output bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
	if !strings.Contains(output.String(), `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize response missing: %s", output.String())
	}
}

func TestToolInputSchemasAreCursorCompatible(t *testing.T) {
	server := mcpserver.New(nil)
	input := frame(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	var output bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	payload := strings.SplitN(output.String(), "\r\n\r\n", 2)
	if len(payload) != 2 {
		t.Fatalf("unexpected frame output: %s", output.String())
	}
	if err := json.Unmarshal([]byte(payload[1]), &resp); err != nil {
		t.Fatalf("decode tools/list response: %v", err)
	}
	if len(resp.Result.Tools) != len(server.ToolNames()) {
		t.Fatalf("tool count = %d, want %d", len(resp.Result.Tools), len(server.ToolNames()))
	}
	for _, tool := range resp.Result.Tools {
		if tool.InputSchema["type"] != "object" {
			t.Fatalf("tool %q schema type = %#v", tool.Name, tool.InputSchema["type"])
		}
		if _, ok := tool.InputSchema["$schema"]; ok {
			t.Fatalf("tool %q schema must not include $schema", tool.Name)
		}
		if tool.InputSchema["additionalProperties"] == true {
			t.Fatalf("tool %q schema must not allow additionalProperties", tool.Name)
		}
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
