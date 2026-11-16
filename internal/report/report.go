// Package report renders comparison metadata without access to parsed values.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/k-dawg23/envdiff/internal/compare"
)

type summary struct {
	Missing    int `json:"missing"`
	Additional int `json:"additional"`
	Different  int `json:"different"`
	Duplicates int `json:"duplicates"`
}
type duplicate struct {
	Key   string `json:"key"`
	File  string `json:"file"`
	Lines []int  `json:"lines"`
}
type document struct {
	SchemaVersion string      `json:"schema_version"`
	Status        string      `json:"status"`
	Mode          string      `json:"mode"`
	Summary       summary     `json:"summary"`
	Missing       []string    `json:"missing"`
	Additional    []string    `json:"additional"`
	Different     []string    `json:"different"`
	Duplicates    []duplicate `json:"duplicates"`
}

func JSON(r compare.Result) ([]byte, error) {
	d := document{SchemaVersion: "1.0", Status: r.Status, Mode: r.Mode, Summary: summary{r.Summary.Missing, r.Summary.Additional, r.Summary.Different, r.Summary.Duplicates}, Missing: append([]string{}, r.Missing...), Additional: append([]string{}, r.Additional...), Different: append([]string{}, r.Different...), Duplicates: []duplicate{}}
	for _, v := range r.Duplicates {
		d.Duplicates = append(d.Duplicates, duplicate{v.Key, string(v.File), v.Lines})
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(d)
	return b.Bytes(), err
}
func Text(r compare.Result, version string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "EnvDiff %s\nMode: %s\nReference: reference\nTarget: target\n\nMissing keys: %d\nAdditional keys: %d\nDifferent values: %d\nDuplicate keys: %d\n\n", version, r.Mode, r.Summary.Missing, r.Summary.Additional, r.Summary.Different, r.Summary.Duplicates)
	for _, c := range []struct {
		name string
		keys []string
	}{{"Missing", r.Missing}, {"Additional", r.Additional}, {"Different", r.Different}} {
		fmt.Fprintf(&b, "%s:\n", c.name)
		if len(c.keys) == 0 {
			b.WriteString("  (none)\n")
		}
		for _, k := range c.keys {
			fmt.Fprintf(&b, "  %s\n", k)
		}
	}
	b.WriteString("Duplicates:\n")
	if len(r.Duplicates) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, d := range r.Duplicates {
		fmt.Fprintf(&b, "  %s (%s: lines ", d.Key, d.File)
		for i, n := range d.Lines {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprint(&b, n)
		}
		b.WriteString(")\n")
	}
	status := map[string]string{"identical": "No differences", "differences": "Differences detected", "duplicates": "Duplicates detected"}[r.Status]
	fmt.Fprintf(&b, "\nResult: %s\n", status)
	return []byte(b.String())
}
