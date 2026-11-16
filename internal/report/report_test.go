package report

import (
	"encoding/json"
	"github.com/k-dawg23/envdiff/internal/compare"
	"github.com/k-dawg23/envdiff/internal/parser"
	"strings"
	"testing"
)

func TestReports(t *testing.T) {
	r := compare.Compare(&parser.File{Keys: map[string][]parser.Occurrence{"B": {{Line: 2}, {Line: 5}}, "A": {{Line: 1}}}}, &parser.File{Keys: map[string][]parser.Occurrence{}}, false)
	text := string(Text(r, "dev"))
	for _, want := range []string{"EnvDiff dev\n", "Missing keys: 2\n", "  A\n  B\n", "B (reference: lines 2, 5)", "Result: Duplicates detected\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing expected output %q", want)
		}
	}
	data, err := JSON(r)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err = json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	if len(d) != 8 || d["schema_version"] != "1.0" || d["status"] != "duplicates" || !strings.HasSuffix(string(data), "\n") {
		t.Fatal("invalid JSON report")
	}
	if d["different"] == nil || d["additional"] == nil {
		t.Fatal("null collections")
	}
}
