package release_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowPublishesBinaries(t *testing.T) {
	root := filepath.Join("..", "..")
	workflow := readFile(t, filepath.Join(root, ".github", "workflows", "release-binaries.yaml"))

	requiredSnippets := []string{
		"name: Release binaries",
		"workflow_call:",
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

func TestPullRequestWorkflowRunsProjectVerification(t *testing.T) {
	root := filepath.Join("..", "..")
	workflow := readFile(t, filepath.Join(root, ".github", "workflows", "tests-on-pr.yaml"))

	requiredSnippets := []string{
		"name: Tests on PR",
		"pull_request:",
		"actions/setup-go@v5",
		"go-version-file: go.mod",
		"make check",
		"make test",
		"make build",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(workflow, snippet) {
			t.Fatalf("pull request workflow is missing %q", snippet)
		}
	}
}

func TestTagOnMergeWorkflowBumpsPatchAndCallsReleaseBinaries(t *testing.T) {
	root := filepath.Join("..", "..")
	workflow := readFile(t, filepath.Join(root, ".github", "workflows", "tag-on-merge.yaml"))

	requiredSnippets := []string{
		"name: Tag release on merge",
		"branches:",
		"main",
		"workflow_dispatch:",
		"release_tag",
		"git tag -a",
		"git push origin",
		"awk -F.",
		"uses: ./.github/workflows/release-binaries.yaml",
		"tag: ${{ needs.release.outputs.tag }}",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(workflow, snippet) {
			t.Fatalf("tag on merge workflow is missing %q", snippet)
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

func TestGitignoreDoesNotHideInternalWorkspacePackage(t *testing.T) {
	root := filepath.Join("..", "..")
	path := filepath.Join("internal", "workspace", "path.go")
	cmd := exec.Command("git", "check-ignore", "-q", path)
	cmd.Dir = root

	if err := cmd.Run(); err == nil {
		t.Fatalf("%s is ignored by git, but it is source code required by cmd/outlook-pst-mcp", path)
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("git check-ignore %s failed unexpectedly: %v", path, err)
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
