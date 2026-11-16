// Package parser implements EnvDiff's restricted dotenv grammar.
package parser

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const MaxSize = 10 * 1024 * 1024

type Role string

const (
	Reference Role = "reference"
	Target    Role = "target"
)

type Kind string

const (
	ErrRead           Kind = "cannot read file"
	ErrFileType       Kind = "not a regular file"
	ErrTooLarge       Kind = "file exceeds size limit"
	ErrEncoding       Kind = "invalid UTF-8"
	ErrNUL            Kind = "NUL byte"
	ErrCarriageReturn Kind = "standalone carriage return"
	ErrAssignment     Kind = "missing assignment"
	ErrKey            Kind = "invalid key"
	ErrQuote          Kind = "unterminated quoted value"
	ErrTrailing       Kind = "unexpected content after quoted value"
)

type Error struct {
	Kind Kind
	Role Role
	Line int
}

func (e *Error) Error() string {
	role := "input"
	if e.Role == Reference || e.Role == Target {
		role = string(e.Role)
	}
	if e.Line > 0 {
		return string(e.Kind) + " in " + role + " file at line " + itoa(e.Line)
	}
	return string(e.Kind) + " in " + role + " file"
}
func itoa(n int) string { // line numbers are positive
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type Occurrence struct {
	Value string
	Line  int
}
type File struct{ Keys map[string][]Occurrence }

func failure(k Kind, role Role, line int) error { return &Error{Kind: k, Role: role, Line: line} }

// ReadFile follows symlinks to regular files and bounds bytes actually read.
func ReadFile(path string, role Role) (*File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, failure(ErrRead, role, 0)
	}
	if !info.Mode().IsRegular() {
		return nil, failure(ErrFileType, role, 0)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, failure(ErrRead, role, 0)
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, failure(ErrRead, role, 0)
	}
	if !info.Mode().IsRegular() {
		return nil, failure(ErrFileType, role, 0)
	}
	return Parse(f, role)
}

// Parse reads at most MaxSize+1 bytes and never embeds input in errors.
func Parse(r io.Reader, role Role) (*File, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return nil, failure(ErrRead, role, 0)
	}
	if len(data) > MaxSize {
		return nil, failure(ErrTooLarge, role, 0)
	}
	if !utf8.Valid(data) {
		return nil, failure(ErrEncoding, role, 0)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		data = data[3:]
	}
	result := &File{Keys: make(map[string][]Occurrence)}
	lines := bytes.Split(data, []byte{'\n'})
	for i, line := range lines {
		number := i + 1
		if bytes.HasSuffix(line, []byte{'\r'}) && i < len(lines)-1 {
			line = line[:len(line)-1]
		}
		if bytes.IndexByte(line, 0) >= 0 {
			return nil, failure(ErrNUL, role, number)
		}
		if bytes.IndexByte(line, '\r') >= 0 {
			return nil, failure(ErrCarriageReturn, role, number)
		}
		s := strings.TrimLeft(string(line), " \t")
		if s == "" || s[0] == '#' {
			continue
		}
		if strings.HasPrefix(s, "export") && len(s) > 6 && (s[6] == ' ' || s[6] == '\t') {
			s = strings.TrimLeft(s[6:], " \t")
		}
		eq := strings.IndexByte(s, '=')
		if eq < 0 {
			return nil, failure(ErrAssignment, role, number)
		}
		key := strings.Trim(s[:eq], " \t")
		if !validKey(key) {
			return nil, failure(ErrKey, role, number)
		}
		value, k := parseValue(s[eq+1:])
		if k != "" {
			return nil, failure(k, role, number)
		}
		result.Keys[key] = append(result.Keys[key], Occurrence{Value: value, Line: number})
	}
	return result, nil
}
func validKey(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range []byte(s) {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func parseValue(raw string) (string, Kind) {
	s := strings.TrimLeft(raw, " \t")
	if len(s) == 0 {
		return "", ""
	}
	if s[0] != '\'' && s[0] != '"' {
		for i := 0; i < len(raw); i++ {
			if raw[i] == '#' && i > 0 && (raw[i-1] == ' ' || raw[i-1] == '\t') {
				raw = raw[:i]
				break
			}
		}
		return strings.Trim(raw, " \t"), ""
	}
	quote := s[0]
	var out strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == quote {
			rest := s[i+1:]
			if rest == "" {
				return out.String(), ""
			}
			if rest[0] != ' ' && rest[0] != '\t' {
				return "", ErrTrailing
			}
			rest = strings.TrimLeft(rest, " \t")
			if rest != "" && rest[0] != '#' {
				return "", ErrTrailing
			}
			return out.String(), ""
		}
		if quote == '"' && c == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case '"':
				out.WriteByte('"')
			case '\\':
				out.WriteByte('\\')
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			default:
				out.WriteByte('\\')
				out.WriteByte(s[i])
			}
			continue
		}
		out.WriteByte(c)
	}
	return "", ErrQuote
}

// IsKind permits callers to inspect wrapped parser errors without exposing input.
func IsKind(err error, kind Kind) bool { var e *Error; return errors.As(err, &e) && e.Kind == kind }
