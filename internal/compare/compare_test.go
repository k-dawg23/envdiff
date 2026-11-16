package compare

import (
	"fmt"
	"github.com/k-dawg23/envdiff/internal/parser"
	"reflect"
	"strings"
	"testing"
)

func parse(t *testing.T, s string) *parser.File {
	t.Helper()
	f, e := parser.Parse(strings.NewReader(s), parser.Reference)
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func TestComparison(t *testing.T) {
	cases := []struct {
		name, a, b                     string
		only                           bool
		status                         string
		missing, additional, different []string
		duplicates                     []Duplicate
	}{
		{name: "empty", status: "identical", missing: []string{}, additional: []string{}, different: []string{}, duplicates: []Duplicate{}},
		{name: "equivalent", a: "A=foo\nB='bar'", b: "A=\"foo\" # c\nB=bar", status: "identical", missing: []string{}, additional: []string{}, different: []string{}, duplicates: []Duplicate{}},
		{name: "mixed", a: "Z=SECRET_ONE\nA=1\nCase=x\nD=one", b: "B=2\nD=two\ncase=x", status: "differences", missing: []string{"A", "Case", "Z"}, additional: []string{"B", "case"}, different: []string{"D"}, duplicates: []Duplicate{}},
		{name: "case value", a: "A=x", b: "A=X", status: "differences", missing: []string{}, additional: []string{}, different: []string{"A"}, duplicates: []Duplicate{}},
		{name: "duplicates", a: "Z=SECRET_ONE\nZ=SECRET_TWO\nA=one\nA=two\nM=1\nM=2\nD=x", b: "Z=other\nZ=again\nN=1\nN=2\nD=y", status: "duplicates", missing: []string{"A", "M"}, additional: []string{"N"}, different: []string{"D"}, duplicates: []Duplicate{{"A", parser.Reference, []int{3, 4}}, {"M", parser.Reference, []int{5, 6}}, {"N", parser.Target, []int{3, 4}}, {"Z", parser.Reference, []int{1, 2}}, {"Z", parser.Target, []int{1, 2}}}},
		{name: "keys only", a: "A=SECRET_ONE\nX=1\nX=2", b: "A=SECRET_TWO\nY=3", only: true, status: "duplicates", missing: []string{"X"}, additional: []string{"Y"}, different: []string{}, duplicates: []Duplicate{{"X", parser.Reference, []int{2, 3}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := parse(t, tc.a), parse(t, tc.b)
			r := Compare(a, b, tc.only)
			if r.Status != tc.status || !reflect.DeepEqual(r.Missing, tc.missing) || !reflect.DeepEqual(r.Additional, tc.additional) || !reflect.DeepEqual(r.Different, tc.different) || !reflect.DeepEqual(r.Duplicates, tc.duplicates) {
				t.Fatal("incorrect comparison")
			}
			if r.Summary != (Summary{len(tc.missing), len(tc.additional), len(tc.different), len(tc.duplicates)}) {
				t.Fatal("incorrect counts")
			}
			if (r.Mode == "keys_only") != tc.only {
				t.Fatal("incorrect mode")
			}
			for i := 0; i < 20; i++ {
				if !reflect.DeepEqual(r, Compare(a, b, tc.only)) {
					t.Fatal("nondeterministic result")
				}
			}
			if strings.Contains(fmt.Sprintf("%+v", r), "SECRET_") {
				t.Fatal("result disclosed sentinel")
			}
		})
	}
}
func TestSelfComparisonWithDuplicates(t *testing.T) {
	f := parse(t, "A=x\nA=y")
	r := Compare(f, f, false)
	if r.Status != "duplicates" || len(r.Duplicates) != 2 || len(r.Different) != 0 {
		t.Fatal("incorrect self comparison")
	}
}
