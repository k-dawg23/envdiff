package parser

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntax(t *testing.T) {
	cases := []struct{ name, input, key, want string }{
		{"empty", "K=", "K", ""}, {"spaces", " K \t= \t hello world \t", "K", "hello world"}, {"single", "K='  a\\n # b  '", "K", "  a\\n # b  "},
		{"double", `K="a\"b\\c\n\r\t\q"`, "K", "a\"b\\c\n\r\t\\q"}, {"hash", "K=abc#123", "K", "abc#123"}, {"comment", "K=abc \t#comment", "K", "abc"}, {"leading comment", "K= #comment", "K", ""}, {"leading hash", "K=#abc", "K", "#abc"},
		{"equals", "K=a=b=c", "K", "a=b=c"}, {"export", "export \tK = 'v' # x", "K", "v"}, {"literal", "K=${OTHER_KEY}", "K", "${OTHER_KEY}"}, {"quote literal", `K=abc"def`, "K", `abc"def`}, {"unicode", "K=é", "K", "é"}, {"bom crlf", "\ufeff# x\r\n \t\r\nK=v\r\n", "K", "v"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Parse(strings.NewReader(tc.input), Reference)
			if err != nil {
				t.Fatal(err)
			}
			got := f.Keys[tc.key]
			if len(got) != 1 || got[0].Value != tc.want {
				t.Fatal("incorrect parse result")
			}
		})
	}
	f, err := Parse(strings.NewReader("\n# comment\nA=one\na=two\nA=three"), Target)
	if err != nil || len(f.Keys) != 2 || len(f.Keys["A"]) != 2 || f.Keys["A"][0].Line != 3 || f.Keys["A"][1].Line != 5 {
		t.Fatal("incorrect key occurrences")
	}
	f, err = Parse(strings.NewReader(" \t\n"), Reference)
	if err != nil || len(f.Keys) != 0 {
		t.Fatal("empty file")
	}
}
func TestErrors(t *testing.T) {
	cases := []struct {
		name, input string
		kind        Kind
		line        int
	}{
		{"bare", "SECRET_SENTINEL", ErrAssignment, 1}, {"digit", "1X=SECRET_SENTINEL", ErrKey, 1}, {"dash", "A-B=SECRET_SENTINEL", ErrKey, 1}, {"unicode key", "é=x", ErrKey, 1}, {"export bare", "export X", ErrAssignment, 1},
		{"single", "K='SECRET_SENTINEL", ErrQuote, 1}, {"double", "K=\"SECRET_SENTINEL", ErrQuote, 1}, {"physical multiline", "K=\"SECRET_SENTINEL\nnext\"", ErrQuote, 1},
		{"adjacent", "K='SECRET_SENTINEL'#x", ErrTrailing, 1}, {"text", "K=\"SECRET_SENTINEL\"junk", ErrTrailing, 1}, {"concat", "K='a' 'SECRET_SENTINEL'", ErrTrailing, 1}, {"cr", "K=x\r", ErrCarriageReturn, 1}, {"nul", "K=SECRET_SENTINEL\x00", ErrNUL, 1}, {"utf8", "K=SECRET_SENTINEL\xff", ErrEncoding, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input), Target)
			var pe *Error
			if !errors.As(err, &pe) || pe.Kind != tc.kind || pe.Line != tc.line || pe.Role != Target {
				t.Fatal("incorrect structured error")
			}
			if strings.Contains(err.Error(), "SECRET_SENTINEL") {
				t.Fatal("error disclosed sentinel")
			}
		})
	}
}
func TestLimits(t *testing.T) {
	for _, n := range []int{MaxSize, MaxSize + 1} {
		t.Run(string(rune('0'+n-MaxSize)), func(t *testing.T) {
			input := append([]byte("K="), bytes.Repeat([]byte{'x'}, n-2)...)
			f, err := Parse(bytes.NewReader(input), Reference)
			if n == MaxSize {
				if err != nil || len(f.Keys["K"][0].Value) != n-2 {
					t.Fatal("limit rejected")
				}
			} else if !IsKind(err, ErrTooLarge) {
				t.Fatal("oversized input accepted")
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("SECRET_SENTINEL") }
func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input")
	data := []byte("K=SECRET_SENTINEL\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := ReadFile(path, Reference)
	if err != nil || len(f.Keys) != 1 {
		t.Fatal("regular file failed")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("file changed")
	}
	if _, err = ReadFile(dir, Target); !IsKind(err, ErrFileType) {
		t.Fatal("directory accepted")
	}
	if _, err = ReadFile(filepath.Join(dir, "SECRET_SENTINEL"), Target); !IsKind(err, ErrRead) || strings.Contains(err.Error(), "SECRET_SENTINEL") {
		t.Fatal("unsafe read error")
	}
	_, err = Parse(failingReader{}, Reference)
	if !IsKind(err, ErrRead) || strings.Contains(err.Error(), "SECRET_SENTINEL") {
		t.Fatal("unsafe reader error")
	}
	link := filepath.Join(dir, "link")
	if err = os.Symlink(path, link); err == nil {
		if _, err = ReadFile(link, Reference); err != nil {
			t.Fatal("symlink to regular file rejected")
		}
	}
}

func TestSyntheticFixtures(t *testing.T) {
	for _, tc := range []struct {
		path string
		kind Kind
	}{
		{"../../testdata/basic/reference.env", ""},
		{"../../testdata/basic/target.env", ""},
		{"../../testdata/duplicates/example.env", ""},
		{"../../testdata/malformed/example.env", ErrQuote},
	} {
		f, err := ReadFile(tc.path, Reference)
		if tc.kind != "" {
			if !IsKind(err, tc.kind) {
				t.Fatal("fixture error mismatch")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(tc.path, "duplicates") && len(f.Keys["PORT"]) != 2 {
			t.Fatal("fixture duplicates missing")
		}
	}
}

func TestStage2RegressionEdges(t *testing.T) {
	cases := []struct {
		name, input string
		kind        Kind
	}{
		{"repeated BOM", "\xef\xbb\xbf\xef\xbb\xbfA=x", ErrKey},
		{"even backslashes", "A=\"x\\\\\"", ""},
		{"odd backslashes", "A=\"x\\\\\\\"", ErrQuote},
		{"ASCII valid boundaries", "A=1\nZ=1\na=1\nz=1\n_=1\nA0=1\nA9=1", ""},
		{"digit first", "0A=1", ErrKey}, {"punctuation", "A-1=1", ErrKey}, {"non ASCII", "é=1", ErrKey},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(c.input), Reference)
			if c.kind == "" && err != nil {
				t.Fatal("unexpected error")
			}
			if c.kind != "" && !IsKind(err, c.kind) {
				t.Fatal("unexpected error kind")
			}
		})
	}
	_, err := Parse(io.MultiReader(strings.NewReader("A=x\n"), stage2ErrorReader{}), Reference)
	if !IsKind(err, ErrRead) {
		t.Fatal("partial read error not propagated safely")
	}
}

type stage2ErrorReader struct{}

func (stage2ErrorReader) Read([]byte) (int, error) { return 0, errors.New("synthetic private error") }

func TestLineIterationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		kind        Kind
		line        int
	}{
		{"empty", "", "", 0},
		{"trailing newline", "A=x\r\n\n", "", 0},
		{"final comment", "A=x\n# comment", "", 0},
		{"final CR", "A=x\r\n\n# comment\r", ErrCarriageReturn, 3},
		{"embedded CR", "\nA=x\r\r\n", ErrCarriageReturn, 2},
		{"NUL after blank lines", "\n\r\n\x00", ErrNUL, 3},
		{"unterminated quote after CRLF", "# comment\r\n\r\nA=\"x", ErrQuote, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input), Target)
			if tc.kind == "" {
				if err != nil {
					t.Fatal("unexpected parse error")
				}
				return
			}
			var pe *Error
			if !errors.As(err, &pe) || pe.Kind != tc.kind || pe.Line != tc.line || pe.Role != Target {
				t.Fatal("incorrect structured error")
			}
		})
	}
	f, err := Parse(strings.NewReader("\r\nA=one\n# comment\r\nA=two"), Reference)
	if err != nil || len(f.Keys["A"]) != 2 || f.Keys["A"][0].Line != 2 || f.Keys["A"][1].Line != 4 {
		t.Fatal("incorrect duplicate line numbers")
	}
}
