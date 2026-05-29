package pst

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Options struct {
	PSTPath   string
	OutputDir string
}

type Result struct {
	Messages []ExtractedMessage
}

type ExtractedMessage struct {
	Path       string
	FolderPath string
}

func Import(options Options) (Result, error) {
	readpst, err := exec.LookPath("readpst")
	if err != nil {
		return Result{}, fmt.Errorf("readpst not found on PATH; install libpst/readpst before importing PST files")
	}
	info, err := os.Stat(options.PSTPath)
	if err != nil {
		return Result{}, fmt.Errorf("invalid PST path: %w", err)
	}
	if info.IsDir() {
		return Result{}, fmt.Errorf("invalid PST path %q: directory is not a PST file", options.PSTPath)
	}
	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return Result{}, err
	}
	cmd := exec.Command(readpst, "-o", options.OutputDir, options.PSTPath)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/usr/bin:/bin")
	if output, err := cmd.CombinedOutput(); err != nil {
		tail := strings.TrimSpace(string(output))
		if len(tail) > 800 {
			tail = tail[len(tail)-800:]
		}
		return Result{}, fmt.Errorf("readpst failed: %w: %s", err, tail)
	}
	var result Result
	err = filepath.WalkDir(options.OutputDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".eml" {
			return nil
		}
		rel, err := filepath.Rel(options.OutputDir, filepath.Dir(path))
		if err != nil {
			return err
		}
		folder := filepath.ToSlash(rel)
		if folder == "." {
			folder = "Root"
		}
		result.Messages = append(result.Messages, ExtractedMessage{Path: path, FolderPath: folder})
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
