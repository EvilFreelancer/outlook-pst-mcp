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
		"https://github.com/EvilFreelancer/outlook-pst-mcp/releases/tag/0.1.1",
		"curl -fsSL https://raw.githubusercontent.com/EvilFreelancer/outlook-pst-mcp/main/install.sh | bash",
		`irm https://raw.githubusercontent.com/EvilFreelancer/outlook-pst-mcp/main/install.ps1 | iex`,
		"./install.sh --version 0.1.1",
		"outlook-pst-mcp_<version>_<os>_<arch>",
		"~/.local/bin/outlook-pst-mcp",
		`"args": ["-workspace",`,
		"import_pst",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README release install documentation is missing %q", snippet)
		}
	}
}

func TestInstallScriptsDownloadReleaseAssets(t *testing.T) {
	root := filepath.Join("..", "..")
	shell := readFile(t, filepath.Join(root, "install.sh"))
	powershell := readFile(t, filepath.Join(root, "install.ps1"))

	shellSnippets := []string{
		"OUTLOOK_PST_MCP_REPO",
		"EvilFreelancer/outlook-pst-mcp",
		"--version X.Y.Z",
		"--install-dir D",
		"--workspace D",
		"releases/latest",
		"outlook-pst-mcp_${TAG}_${GOOS}_${GOARCH}.tar.gz",
		"SHA256SUMS",
		"sha256sum -c",
		"outlook-pst-mcp --help",
	}
	for _, snippet := range shellSnippets {
		if !strings.Contains(shell, snippet) {
			t.Fatalf("install.sh is missing %q", snippet)
		}
	}

	powershellSnippets := []string{
		"OUTLOOK_PST_MCP_REPO",
		"EvilFreelancer/outlook-pst-mcp",
		"outlook-pst-mcp_${tag}_windows_${arch}.zip",
		"Invoke-RestMethod",
		"Invoke-WebRequest",
		"Expand-Archive",
		"outlook-pst-mcp.exe",
		"Get-Command readpst",
		"readpst.exe",
		"mcpServers",
	}
	for _, snippet := range powershellSnippets {
		if !strings.Contains(powershell, snippet) {
			t.Fatalf("install.ps1 is missing %q", snippet)
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
