package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDir is the workspace directory name under the process cwd when -workspace is omitted.
const DefaultDir = ".outlook-pst-mcp_data"

// Resolve returns dir when non-empty; otherwise <cwd>/DefaultDir.
func Resolve(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir != "" {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	return filepath.Join(cwd, DefaultDir), nil
}
