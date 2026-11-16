//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRemediationLinuxFileErrors(t *testing.T) {
	exe := remediationBinary(t)
	dir := t.TempDir()
	reference := filepath.Join(dir, "reference")
	remediationPut(t, reference, "A=x\n")
	run := func(target, diagnostic string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "compare", "--format=json", reference, target)
		var out, errout bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errout
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatal("CLI timed out")
		}
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 2 || out.Len() != 0 || errout.String() != diagnostic {
			t.Fatal("file error contract mismatch")
		}
	}
	t.Run("FIFO", func(t *testing.T) {
		path := filepath.Join(dir, "sensitive_fifo_name")
		if err := syscall.Mkfifo(path, 0600); err != nil {
			t.Fatalf("create FIFO: %v", err)
		}
		run(path, "Error: not a regular file in target file.\n")
	})
	t.Run("permission denied", func(t *testing.T) {
		path := filepath.Join(dir, "sensitive_unreadable_name")
		remediationPut(t, path, "A=x\n")
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(path, 0600)
		f, err := os.Open(path)
		if err == nil {
			f.Close()
			t.Skip("current privileges or filesystem allow reading mode 0000")
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("could not establish permission denial: %v", err)
		}
		run(path, "Error: cannot read file in target file.\n")
	})
}
