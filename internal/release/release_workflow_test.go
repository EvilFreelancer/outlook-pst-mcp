package release_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowPublishesBinaries(t *testing.T) {
	root := filepath.Join("..", "..")
	workflow := readFile(t, filepath.Join(root, ".github", "workflows", "release-binaries.yaml"))

	requiredSnippets := []string{
		"name: Release binaries",
		"workflow_dispatch:",
		"push:",
		"tags:",
		"[0-9]+.[0-9]+.[0-9]+",
		"actions/setup-go@v5",
		"go-version-file: go.mod",
		"CGO_ENABLED=1",
		"outlook-pst-mcp_${VERSION}_${GOOS}_${GOARCH}",
		"./cmd/outlook-pst-mcp",
		"SHA256SUMS",
		"softprops/action-gh-release@v2",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(workflow, snippet) {
			t.Fatalf("release workflow is missing %q", snippet)
		}
	}
}

func TestReadmeDocumentsReleaseBinaryInstall(t *testing.T) {
	root := filepath.Join("..", "..")
	readme := readFile(t, filepath.Join(root, "README.md"))

	requiredSnippets := []string{
		"GitHub Releases",
		"outlook-pst-mcp_<version>_<os>_<arch>",
		"SHA256SUMS",
		"~/.local/bin/outlook-pst-mcp",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README release install documentation is missing %q", snippet)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
