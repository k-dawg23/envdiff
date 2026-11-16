package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testExecutablePath(dir string) string {
	name := "envdiff"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

func TestCompiledCLI(t *testing.T) {
	dir := t.TempDir()
	exe := testExecutablePath(dir)
	build := exec.Command("go", "build", "-o", exe, ".")
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	ref := filepath.Join(dir, "reference synthetic_path_secret.env")
	target := filepath.Join(dir, "target.env")
	put := func(p, s string) {
		t.Helper()
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	run := func(args ...string) (int, string, string) {
		t.Helper()
		cmd := exec.Command(exe, args...)
		var out, errout bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errout
		e := cmd.Run()
		code := 0
		if e != nil {
			if x, ok := e.(*exec.ExitError); ok {
				code = x.ExitCode()
			} else {
				t.Fatal(e)
			}
		}
		for _, s := range []string{"synthetic_value_secret", "synthetic_path_secret", "synthetic_argument_secret"} {
			if strings.Contains(out.String(), s) || strings.Contains(errout.String(), s) {
				t.Fatal("sensitive sentinel disclosed")
			}
		}
		return code, out.String(), errout.String()
	}
	original := "Z=synthetic_value_secret\nA=one\nM=x\nD=first\nD=second\n"
	put(ref, original)
	put(target, "Z=other\nB=x\nD=x\nD=y\n")
	code, out, errout := run("compare", "--format=json", ref, "--ignore-values", target)
	if code != 3 || errout != "" {
		t.Fatalf("duplicate result %d %q", code, errout)
	}
	var doc struct {
		Schema                         string         `json:"schema_version"`
		Status                         string         `json:"status"`
		Mode                           string         `json:"mode"`
		Summary                        map[string]int `json:"summary"`
		Missing, Additional, Different []string
		Duplicates                     []struct {
			Key, File string
			Lines     []int
		}
	}
	if e := json.Unmarshal([]byte(out), &doc); e != nil {
		t.Fatal(e)
	}
	if doc.Schema != "1.0" || doc.Status != "duplicates" || doc.Mode != "keys_only" || doc.Summary["missing"] != 2 || doc.Summary["additional"] != 1 || doc.Summary["different"] != 0 || doc.Summary["duplicates"] != 2 || doc.Different == nil || len(doc.Duplicates) != 2 || doc.Duplicates[0].File != "reference" || doc.Duplicates[1].File != "target" {
		t.Fatal("incorrect JSON contract")
	}
	_, out2, _ := run("compare", "--format=json", ref, "--ignore-values", target)
	if out != out2 {
		t.Fatal("non-deterministic output")
	}
	code, out, errout = run("compare", ref, target)
	if code != 3 || errout != "" || !strings.Contains(out, "Result: Duplicates detected\n") || !strings.Contains(out, "Different values: 1") {
		t.Fatal("incomplete text report")
	}
	b, e := os.ReadFile(ref)
	if e != nil || string(b) != original {
		t.Fatal("input changed")
	}
	put(ref, "A=x\n")
	put(target, "A=x\n")
	code, out, errout = run("compare", ref, target, "--format", "json")
	if code != 0 || errout != "" || !strings.Contains(out, `"missing":[]`) || !strings.Contains(out, `"duplicates":[]`) {
		t.Fatal("identical JSON contract")
	}
	put(target, "A=y\n")
	code, _, errout = run("compare", ref, target)
	if code != 1 || errout != "" {
		t.Fatal("difference exit")
	}
	code, _, _ = run("compare", ref, target, "--ignore-values")
	if code != 0 {
		t.Fatal("key-only exit")
	}
	put(target, "A=\"synthetic_value_secret\n")
	code, out, errout = run("compare", ref, target, "--ignore-values")
	if code != 2 || out != "" || !strings.Contains(errout, "target file at line 1") {
		t.Fatal("key-only validation")
	}
	code, out, errout = run("compare", filepath.Join(dir, "missing"), target)
	if code != 2 || out != "" || !strings.Contains(errout, "reference file") {
		t.Fatal("reference first")
	}
	for _, args := range [][]string{{"--help"}, {"compare", "--help"}, {"--version"}} {
		code, out, errout = run(args...)
		if code != 0 || out == "" || errout != "" {
			t.Fatal("help/version")
		}
	}
	for _, args := range [][]string{{}, {"synthetic_argument_secret"}, {"compare", "--help", ref, target}, {"compare", "--format=bad", ref, target}, {"--version", "--help"}} {
		code, out, errout = run(args...)
		if code != 2 || out != "" || errout == "" {
			t.Fatal("invalid invocation")
		}
	}
	code, out, errout = run("compare", dir, target)
	if code != 2 || out != "" || !strings.Contains(errout, "not a regular file") {
		t.Fatal("directory handling")
	}
}

func TestCompiledInputBoundaries(t *testing.T) {
	dir := t.TempDir()
	exe := testExecutablePath(dir)
	if b, e := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	ref := filepath.Join(dir, "ref")
	target := filepath.Join(dir, "target")
	if e := os.WriteFile(ref, []byte("A=x\n"), 0600); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name       string
		data       []byte
		diagnostic string
	}{{"invalid UTF8", []byte{0xff}, "invalid UTF-8"}, {"NUL", []byte("A=\x00"), "NUL byte"}, {"too large", bytes.Repeat([]byte{'x'}, 10*1024*1024+1), "size limit"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if e := os.WriteFile(target, c.data, 0600); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command(exe, "compare", "--format=json", ref, target)
			var out, errout bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errout
			e := cmd.Run()
			x, ok := e.(*exec.ExitError)
			if !ok || x.ExitCode() != 2 || out.Len() != 0 || !strings.Contains(errout.String(), c.diagnostic) {
				t.Fatal("incorrect input error")
			}
		})
	}
}
