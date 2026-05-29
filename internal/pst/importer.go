package pst

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

	baseArgs := []string{"-e", "-b", "-w", "-o", options.OutputDir, options.PSTPath}
	if output, err := runReadpst(readpst, baseArgs); err != nil {
		if !isSegfault(err) {
			return Result{}, formatReadpstError(err, output)
		}
		safeArgs := []string{"-e", "-b", "-w", "-q", "-j", "1", "-t", "e", "-8", "-o", options.OutputDir, options.PSTPath}
		if output2, err2 := runReadpst(readpst, safeArgs); err2 != nil {
			return Result{}, formatReadpstError(err2, output2)
		}
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
	sortExtractedMessages(result.Messages)
	return result, nil
}

func runReadpst(readpst string, args []string) ([]byte, error) {
	cmd := exec.Command(readpst, args...)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/usr/bin:/bin")
	return cmd.CombinedOutput()
}

func isSegfault(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	if status.Signaled() && status.Signal() == syscall.SIGSEGV {
		return true
	}
	if status.Exited() && status.ExitStatus() == 128+int(syscall.SIGSEGV) {
		return true
	}
	if status.Exited() && status.ExitStatus() == 139 {
		return true
	}
	return false
}

func formatReadpstError(err error, output []byte) error {
	tail := strings.TrimSpace(string(output))
	if len(tail) > 800 {
		tail = tail[len(tail)-800:]
	}
	return fmt.Errorf("readpst failed: %w: %s", err, tail)
}
