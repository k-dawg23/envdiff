package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type broken struct{}

func (broken) Write(p []byte) (int, error) {
	if len(p) > 0 {
		return 1, errors.New("synthetic private error")
	}
	return 0, io.ErrShortWrite
}
func TestArguments(t *testing.T) {
	cases := []struct {
		args []string
		ok   bool
	}{
		{[]string{"a", "b"}, true}, {[]string{"--format=json", "a", "--ignore-values", "b"}, true}, {[]string{"a", "--format", "text", "b"}, true}, {[]string{"--", "-a", "-b"}, true},
		{nil, false}, {[]string{"a"}, false}, {[]string{"a", "b", "c"}, false}, {[]string{"--format", "a", "b"}, false}, {[]string{"--format=xml", "a", "b"}, false}, {[]string{"--format=text", "--format=json", "a", "b"}, false}, {[]string{"--ignore-values", "--ignore-values", "a", "b"}, false}, {[]string{"--ignore-values=true", "a", "b"}, false}, {[]string{"--help", "a", "b"}, false},
	}
	for _, c := range cases {
		_, ok := parse(c.args)
		if ok != c.ok {
			t.Errorf("argument acceptance mismatch for %q", c.args)
		}
	}
}
func TestWriteFailureAndSafeArguments(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "input")
	if err := os.WriteFile(p, []byte("A=synthetic_secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := Run([]string{"compare", p, p}, broken{}, &stderr, "dev"); code != 2 || stderr.String() != "Error: Cannot write output.\n" {
		t.Fatalf("write failure: code %d diagnostic %q", code, stderr.String())
	}
	var out bytes.Buffer
	stderr.Reset()
	if code := Run([]string{"synthetic_secret"}, &out, &stderr, "dev"); code != 2 || out.Len() != 0 || strings.Contains(stderr.String(), "synthetic_secret") {
		t.Fatal("unsafe argument error")
	}
}

type shortWriter struct {
	captured  bytes.Buffer
	withError bool
}

func (w *shortWriter) Write(p []byte) (int, error) {
	n := 0
	if len(p) > 0 {
		n = 1
		w.captured.Write(p[:n])
	}
	if w.withError {
		return n, errors.New("sensitive underlying error")
	}
	return n, nil
}
func TestRemediationOutputFailures(t *testing.T) {
	dir := t.TempDir()
	ref := filepath.Join(dir, "ref")
	target := filepath.Join(dir, "target")
	cases := []struct {
		name              string
		args              []string
		reference, target string
	}{
		{name: "help", args: []string{"--help"}}, {name: "compare help", args: []string{"compare", "--help"}}, {name: "version", args: []string{"--version"}},
		{name: "identical", args: []string{"compare", ref, target}, reference: "A=x\n", target: "A=x\n"},
		{name: "differences", args: []string{"compare", "--format=json", ref, target}, reference: "A=x\n", target: "A=y\n"},
		{name: "duplicates", args: []string{"compare", "--format=json", ref, target}, reference: "A=x\nA=y\n", target: "A=x\n"},
	}
	for _, tc := range cases {
		for _, withError := range []bool{false, true} {
			name := tc.name + "/short"
			if withError {
				name = tc.name + "/error"
			}
			t.Run(name, func(t *testing.T) {
				if tc.reference != "" {
					if e := os.WriteFile(ref, []byte(tc.reference), 0600); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(target, []byte(tc.target), 0600); e != nil {
						t.Fatal(e)
					}
				}
				w := &shortWriter{withError: withError}
				var stderr bytes.Buffer
				code := Run(tc.args, w, &stderr, "dev")
				if code != 2 || stderr.String() != "Error: Cannot write output.\n" || w.captured.Len() != 1 {
					t.Fatal("output failure contract mismatch")
				}
			})
		}
	}
}
