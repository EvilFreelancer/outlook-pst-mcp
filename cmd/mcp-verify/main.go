package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	pstPath := filepath.Join(root, "data", "backup.pst")
	if len(os.Args) > 1 {
		pstPath = os.Args[1]
	}

	workspace, err := os.MkdirTemp("", "outlook-pst-mcp-verify-*")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(workspace)

	bin := filepath.Join(root, "bin", "outlook-pst-mcp")

	importStart := time.Now()
	importCmd := exec.Command(bin, "import", "-workspace", workspace, "-pst", pstPath)
	importCmd.Env = append(os.Environ(), bundleEnv(root)...)
	importOutput, err := importCmd.CombinedOutput()
	if err != nil {
		fatal(fmt.Errorf("import failed: %w: %s", err, strings.TrimSpace(string(importOutput))))
	}

	cmd := exec.Command(bin, "-workspace", workspace)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), bundleEnv(root)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatal(err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	ctx := context.Background()
	reader := bufio.NewReader(stdout)

	if err := writeFrame(stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "mcp-verify", "version": "0.1.0"},
		},
	}); err != nil {
		fatal(err)
	}
	if _, err := readFrame(reader); err != nil {
		fatal(err)
	}

	fmt.Println("=== import (CLI) ===")
	fmt.Printf("import done in %s: %s\n", time.Since(importStart).Round(time.Second), strings.TrimSpace(string(importOutput)))

	fmt.Println("\n=== list_folders (first 20) ===")
	foldersRaw, err := callTool(stdin, reader, 2, "list_folders", map[string]any{})
	if err != nil {
		fatal(err)
	}
	var folders []map[string]any
	if err := json.Unmarshal(foldersRaw, &folders); err != nil {
		fatal(err)
	}
	fmt.Printf("folder_count=%d\n", len(folders))
	for i, folder := range folders {
		if i >= 20 {
			fmt.Printf("... and %d more folders\n", len(folders)-20)
			break
		}
		fmt.Printf("  - %s (%s)\n", folder["Path"], folder["ID"])
	}

	fmt.Println("\n=== list_messages (no filter, limit 5) ===")
	pageRaw, err := callTool(stdin, reader, 3, "list_messages", map[string]any{"limit": 5})
	if err != nil {
		fatal(err)
	}
	var page struct {
		Messages []map[string]any `json:"messages"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal(pageRaw, &page); err != nil {
		fatal(err)
	}
	fmt.Printf("total_messages=%d\n", page.Total)
	if page.Total == 0 {
		fatal(fmt.Errorf("no messages indexed"))
	}
	for _, msg := range page.Messages {
		fmt.Printf("  - %s | %s\n", msg["ID"], msg["Subject"])
	}

	sampleID, _ := page.Messages[0]["ID"].(string)
	fmt.Println("\n=== get_message (body) ===")
	bodyRaw, err := callTool(stdin, reader, 4, "get_message", map[string]any{
		"message_id":   sampleID,
		"include_body": true,
	})
	if err != nil {
		fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(bodyRaw, &body); err != nil {
		fatal(err)
	}
	subject, _ := body["subject"].(string)
	from, _ := body["from"].(string)
	bodyText, _ := body["body_text"].(string)
	if len(bodyText) > 120 {
		bodyText = bodyText[:120] + "..."
	}
	fmt.Printf("id=%s subject=%q from=%q body_preview=%q\n", sampleID, subject, from, bodyText)

	queries := []string{"@", "re:", "invoice", "привет", "test"}
	fmt.Println("\n=== search (list_messages query) ===")
	fmt.Println("Note: search matches subject, from, and to only (not body).")
	for i, query := range queries {
		raw, err := callTool(stdin, reader, 10+i, "list_messages", map[string]any{
			"query": query,
			"limit": 3,
		})
		if err != nil {
			fmt.Printf("  query %q: ERROR %v\n", query, err)
			continue
		}
		var result struct {
			Messages []map[string]any `json:"messages"`
			Total    int              `json:"total"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			fmt.Printf("  query %q: parse error %v\n", query, err)
			continue
		}
		fmt.Printf("  query %q: total=%d", query, result.Total)
		if len(result.Messages) > 0 {
			subj, _ := result.Messages[0]["subject"].(string)
			fmt.Printf(" first=%q", subj)
		}
		fmt.Println()
	}

	_ = ctx
}

func bundleEnv(root string) []string {
	bundle := filepath.Join(root, "tools", "readpst-bundle")
	lib := filepath.Join(bundle, "usr", "lib", "x86_64-linux-gnu")
	bin := filepath.Join(bundle, "usr", "bin")

	ld := "LD_LIBRARY_PATH=" + lib
	if current := os.Getenv("LD_LIBRARY_PATH"); current != "" {
		ld = ld + ":" + current
	}
	path := "PATH=" + bin + ":" + os.Getenv("PATH")
	return []string{ld, path}
}

func callTool(stdin io.Writer, stdout *bufio.Reader, id int, name string, args map[string]any) (json.RawMessage, error) {
	if err := writeFrame(stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}); err != nil {
		return nil, err
	}
	resp, err := readFrame(stdout)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("%s", envelope.Error.Message)
	}
	if len(envelope.Result.Content) == 0 {
		return nil, fmt.Errorf("empty tool result")
	}
	return json.RawMessage(envelope.Result.Content[0].Text), nil
}

func writeFrame(w io.Writer, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		return err
	}
	return nil
}

func readFrame(r *bufio.Reader) (json.RawMessage, error) {
	var length int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			value := strings.TrimSpace(line[len("content-length:"):])
			length, err = strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
		}
	}
	if length <= 0 {
		return nil, fmt.Errorf("invalid content length %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return json.RawMessage(buf), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
