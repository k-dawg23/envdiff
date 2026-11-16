package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func remediationBinary(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "envdiff")
	if b, e := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); e != nil {
		t.Fatalf("build failed: %v %s", e, b)
	}
	return exe
}
func remediationRun(t *testing.T, exe, dir string, args ...string) (int, string, string) {
	t.Helper()
	c := exec.Command(exe, args...)
	c.Dir = dir
	var out, errout bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errout
	e := c.Run()
	if e == nil {
		return 0, out.String(), errout.String()
	}
	if x, ok := e.(*exec.ExitError); ok {
		return x.ExitCode(), out.String(), errout.String()
	}
	t.Fatalf("process failed: %v", e)
	return 0, "", ""
}
func remediationPut(t *testing.T, path, content string) {
	t.Helper()
	if e := os.WriteFile(path, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}

func TestRemediationReports(t *testing.T) {
	exe := remediationBinary(t)
	dir := t.TempDir()
	ref := filepath.Join(dir, "ref")
	target := filepath.Join(dir, "target")
	cases := []struct {
		name, ref, target, text, json string
		code                          int
	}{
		{"identical", "A=x\n", "A=x\n", "EnvDiff dev\nMode: keys_and_values\nReference: reference\nTarget: target\n\nMissing keys: 0\nAdditional keys: 0\nDifferent values: 0\nDuplicate keys: 0\n\nMissing:\n  (none)\nAdditional:\n  (none)\nDifferent:\n  (none)\nDuplicates:\n  (none)\n\nResult: No differences\n", `{"schema_version":"1.0","status":"identical","mode":"keys_and_values","summary":{"missing":0,"additional":0,"different":0,"duplicates":0},"missing":[],"additional":[],"different":[],"duplicates":[]}`, 0},
		{"differences", "Z=x\nB=x\nA=x\n", "Z=y\nD=x\nC=x\n", "EnvDiff dev\nMode: keys_and_values\nReference: reference\nTarget: target\n\nMissing keys: 2\nAdditional keys: 2\nDifferent values: 1\nDuplicate keys: 0\n\nMissing:\n  A\n  B\nAdditional:\n  C\n  D\nDifferent:\n  Z\nDuplicates:\n  (none)\n\nResult: Differences detected\n", `{"schema_version":"1.0","status":"differences","mode":"keys_and_values","summary":{"missing":2,"additional":2,"different":1,"duplicates":0},"missing":["A","B"],"additional":["C","D"],"different":["Z"],"duplicates":[]}`, 1},
		{"duplicates", "Z=x\nB=x\nB=y\nA=x\nA=y\n", "Z=y\nA=x\nA=y\nB=x\nB=y\n", "EnvDiff dev\nMode: keys_and_values\nReference: reference\nTarget: target\n\nMissing keys: 0\nAdditional keys: 0\nDifferent values: 1\nDuplicate keys: 4\n\nMissing:\n  (none)\nAdditional:\n  (none)\nDifferent:\n  Z\nDuplicates:\n  A (reference: lines 4, 5)\n  A (target: lines 2, 3)\n  B (reference: lines 2, 3)\n  B (target: lines 4, 5)\n\nResult: Duplicates detected\n", `{"schema_version":"1.0","status":"duplicates","mode":"keys_and_values","summary":{"missing":0,"additional":0,"different":1,"duplicates":4},"missing":[],"additional":[],"different":["Z"],"duplicates":[{"key":"A","file":"reference","lines":[4,5]},{"key":"A","file":"target","lines":[2,3]},{"key":"B","file":"reference","lines":[2,3]},{"key":"B","file":"target","lines":[4,5]}]}`, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remediationPut(t, ref, tc.ref)
			remediationPut(t, target, tc.target)
			code, out, errout := remediationRun(t, exe, dir, "compare", ref, target)
			if code != tc.code || out != tc.text || errout != "" {
				t.Fatal("text report or exit mismatch")
			}
			code, out, errout = remediationRun(t, exe, dir, "compare", "--format=json", ref, target)
			if code != tc.code || errout != "" || !strings.HasSuffix(out, "\n") {
				t.Fatal("JSON output or exit mismatch")
			}
			var actual, want any
			if json.Unmarshal([]byte(out), &actual) != nil || json.Unmarshal([]byte(tc.json), &want) != nil || !reflect.DeepEqual(actual, want) {
				t.Fatal("JSON schema or contents mismatch")
			}
			for _, p := range []struct{ path, want string }{{ref, tc.ref}, {target, tc.target}} {
				b, e := os.ReadFile(p.path)
				if e != nil || string(b) != p.want {
					t.Fatal("input bytes changed")
				}
			}
		})
	}
	// The key-only schema has the same fields, with different suppressed.
	remediationPut(t, ref, "A=x\n")
	remediationPut(t, target, "A=y\n")
	code, out, errout := remediationRun(t, exe, dir, "compare", "--ignore-values", "--format=json", ref, target)
	var actual, want any
	expected := `{"schema_version":"1.0","status":"identical","mode":"keys_only","summary":{"missing":0,"additional":0,"different":0,"duplicates":0},"missing":[],"additional":[],"different":[],"duplicates":[]}`
	if code != 0 || errout != "" || json.Unmarshal([]byte(out), &actual) != nil || json.Unmarshal([]byte(expected), &want) != nil || !reflect.DeepEqual(actual, want) {
		t.Fatal("keys-only schema mismatch")
	}
}

func TestRemediationArgumentsAndPrecedence(t *testing.T) {
	exe := remediationBinary(t)
	dir := t.TempDir()
	remediationPut(t, filepath.Join(dir, "-ref"), "A=x\n")
	remediationPut(t, filepath.Join(dir, "-target"), "A=x\n")
	for _, args := range [][]string{{"compare", "--format=json", "--", "-ref", "-target"}, {"compare", "--format", "json", "--", "-ref", "-target"}} {
		code, _, errout := remediationRun(t, exe, dir, args...)
		if code != 0 || errout != "" {
			t.Fatal("end-of-options failed")
		}
	}
	ref := filepath.Join(dir, "ref_sensitive_filename")
	target := filepath.Join(dir, "target_sensitive_filename")
	remediationPut(t, ref, "A=x\n")
	remediationPut(t, target, "A=x\n")
	for _, args := range [][]string{{"compare", "--format=json", ref, target}, {"compare", ref, "--format=json", target}, {"compare", ref, target, "--format", "json"}, {"compare", "--ignore-values", ref, target}, {"compare", ref, "--ignore-values", target}, {"compare", ref, target, "--ignore-values"}} {
		code, _, errout := remediationRun(t, exe, dir, args...)
		if code != 0 || errout != "" {
			t.Fatal("option position failed")
		}
	}
	for _, args := range [][]string{{"compare", ref, target, "--format"}, {"compare", "--format", ref, target}, {"compare", ref, target, "--format=json", "--format=text"}, {"compare", ref, target, "--ignore-values", "--ignore-values"}, {"compare", "sensitive_argument_value"}} {
		code, out, errout := remediationRun(t, exe, dir, args...)
		if code != 2 || out != "" || errout != "Error: Invalid command or arguments.\n" {
			t.Fatal("unsafe argument diagnostic")
		}
	}
	remediationPut(t, ref, "A=\"broken\n")
	remediationPut(t, target, "A=\"broken\n")
	code, out, errout := remediationRun(t, exe, dir, "compare", ref, target)
	if code != 2 || out != "" || errout != "Error: unterminated quoted value in reference file at line 1.\n" {
		t.Fatal("reference-first diagnostic mismatch")
	}
	for _, reference := range []string{"A=x\nA=y\n", "ONLY_REFERENCE=x\n"} {
		remediationPut(t, ref, reference)
		for _, opts := range [][]string{{"--format=json"}, {"--format=json", "--ignore-values"}} {
			args := append([]string{"compare"}, opts...)
			args = append(args, ref, target)
			code, out, errout = remediationRun(t, exe, dir, args...)
			if code != 2 || out != "" || errout != "Error: unterminated quoted value in target file at line 1.\n" {
				t.Fatal("operational error precedence mismatch")
			}
		}
	}
}

func TestRemediationSecretMatrix(t *testing.T) {
	exe := remediationBinary(t)
	dir := t.TempDir()
	ref := filepath.Join(dir, "synthetic_sensitive_path_ref")
	target := filepath.Join(dir, "target")
	secrets := []string{"secret_alpha_7426", "secret_beta_5813", "secret_gamma_9302", "secret_delta_4175"}
	remediationPut(t, ref, "D="+secrets[0]+"\nD="+secrets[1]+"\nA="+secrets[2]+"\n")
	remediationPut(t, target, "A="+secrets[3]+"\n")
	assertSafe := func(out, errout string) {
		t.Helper()
		combined := out + errout
		for _, s := range secrets {
			hash := sha256.Sum256([]byte(s))
			for _, v := range []string{s, s[7:12], hex.EncodeToString(hash[:])} {
				if strings.Contains(combined, v) {
					t.Fatal("synthetic value, excerpt or hash leaked")
				}
			}
		}
		if strings.Contains(combined, "synthetic_sensitive_path_ref") {
			t.Fatal("path leaked")
		}
	}
	for _, format := range []string{"text", "json"} {
		for _, keys := range []bool{false, true} {
			args := []string{"compare", "--format", format}
			if keys {
				args = append(args, "--ignore-values")
			}
			args = append(args, ref, target)
			code, out, errout := remediationRun(t, exe, dir, args...)
			if code != 3 || errout != "" {
				t.Fatal("duplicate report failed")
			}
			assertSafe(out, errout)
			remediationPut(t, target, "A=\""+secrets[3]+"\n")
			code, out, errout = remediationRun(t, exe, dir, args...)
			if code != 2 || out != "" || errout != "Error: unterminated quoted value in target file at line 1.\n" {
				t.Fatal("malformed input diagnostic mismatch")
			}
			assertSafe(out, errout)
			remediationPut(t, target, "A="+secrets[3]+"\n")
		}
	}
}
