//go:build linux

package parser

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A subprocess ensures a regression that opens a FIFO cannot hang the suite.
func TestLinuxSpecialFiles(t *testing.T) {
	if path := os.Getenv("ENVDIFF_SPECIAL_TEST_PATH"); path != "" {
		_, err := ReadFile(path, Target)
		if !IsKind(err, ErrFileType) || strings.Contains(err.Error(), "SENTINEL_PATH") {
			t.Fatal("special file did not produce a safe file-type error")
		}
		return
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "SENTINEL_PATH_fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatalf("create FIFO: %v", err)
	}
	fifoLink := filepath.Join(dir, "SENTINEL_PATH_fifo_link")
	dirLink := filepath.Join(dir, "SENTINEL_PATH_dir_link")
	if err := os.Symlink(fifo, fifoLink); err != nil {
		t.Fatalf("create FIFO symlink: %v", err)
	}
	if err := os.Symlink(dir, dirLink); err != nil {
		t.Fatalf("create directory symlink: %v", err)
	}
	for _, tc := range []struct{ name, path string }{
		{"fifo", fifo}, {"fifo_symlink", fifoLink}, {"character_device", "/dev/null"},
		{"directory", dir}, {"directory_symlink", dirLink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLinuxSpecialFiles$")
			cmd.Env = append(os.Environ(), "ENVDIFF_SPECIAL_TEST_PATH="+tc.path)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("special file handling timed out")
			}
			if err != nil {
				t.Fatalf("special file subprocess failed: %v\n%s", err, output)
			}
		})
	}
}

func TestLinuxUnreadableRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SENTINEL_PATH")
	if err := os.WriteFile(path, []byte("K=SYNTHETIC_VALUE\n"), 0000); err != nil {
		t.Fatal("could not create fixture")
	}
	f, err := os.Open(path)
	if err == nil {
		f.Close()
		t.Skip("current identity can read mode 0000 files; permission denial cannot be exercised")
	}
	if !os.IsPermission(err) {
		t.Fatalf("fixture open failed for a reason other than permission denial: %v", err)
	}
	_, err = ReadFile(path, Reference)
	if !IsKind(err, ErrRead) || strings.Contains(err.Error(), "SENTINEL_PATH") || strings.Contains(err.Error(), "SYNTHETIC_VALUE") {
		t.Fatal("permission denial did not produce a safe read error")
	}
}
