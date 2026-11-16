// Package compare produces report-safe comparison metadata.
package compare

import (
	"github.com/k-dawg23/envdiff/internal/parser"
	"sort"
)

type Duplicate struct {
	Key   string
	File  parser.Role
	Lines []int
}
type Summary struct{ Missing, Additional, Different, Duplicates int }
type Result struct {
	Mode, Status                   string
	Summary                        Summary
	Missing, Additional, Different []string
	Duplicates                     []Duplicate
}

func Compare(reference, target *parser.File, keysOnly bool) Result {
	r := Result{Mode: "keys_and_values", Status: "identical", Missing: []string{}, Additional: []string{}, Different: []string{}, Duplicates: []Duplicate{}}
	if keysOnly {
		r.Mode = "keys_only"
	}
	for key, occ := range reference.Keys {
		other, ok := target.Keys[key]
		if !ok {
			r.Missing = append(r.Missing, key)
		} else if !keysOnly && len(occ) == 1 && len(other) == 1 && occ[0].Value != other[0].Value {
			r.Different = append(r.Different, key)
		}
	}
	for key := range target.Keys {
		if _, ok := reference.Keys[key]; !ok {
			r.Additional = append(r.Additional, key)
		}
	}
	for _, entry := range []struct {
		role parser.Role
		file *parser.File
	}{{parser.Reference, reference}, {parser.Target, target}} {
		for key, occ := range entry.file.Keys {
			if len(occ) > 1 {
				lines := make([]int, len(occ))
				for i, o := range occ {
					lines[i] = o.Line
				}
				sort.Ints(lines)
				r.Duplicates = append(r.Duplicates, Duplicate{Key: key, File: entry.role, Lines: lines})
			}
		}
	}
	sort.Strings(r.Missing)
	sort.Strings(r.Additional)
	sort.Strings(r.Different)
	sort.Slice(r.Duplicates, func(i, j int) bool {
		if r.Duplicates[i].Key != r.Duplicates[j].Key {
			return r.Duplicates[i].Key < r.Duplicates[j].Key
		}
		return r.Duplicates[i].File == parser.Reference && r.Duplicates[j].File == parser.Target
	})
	r.Summary = Summary{len(r.Missing), len(r.Additional), len(r.Different), len(r.Duplicates)}
	if len(r.Duplicates) > 0 {
		r.Status = "duplicates"
	} else if len(r.Missing)+len(r.Additional)+len(r.Different) > 0 {
		r.Status = "differences"
	}
	return r
}
