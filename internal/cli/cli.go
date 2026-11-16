// Package cli handles command line validation and orchestration.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/k-dawg23/envdiff/internal/compare"
	"github.com/k-dawg23/envdiff/internal/parser"
	"github.com/k-dawg23/envdiff/internal/report"
)

const help = `EnvDiff compares two dotenv files without displaying values.

Usage:
  envdiff compare [options] <reference> <target>
  envdiff --help
  envdiff --version
  envdiff compare --help

Options:
  --format text|json    Output format (default: text; --format=json also works)
  --ignore-values       Compare keys only; both files are still validated
  --                    Treat subsequent arguments as paths

Compare options can precede, follow, or appear between paths. The first file
is the reference. Keys are sorted in ASCII order; duplicates are sorted by key
and then file role. Exit codes: 0 identical, 1 differences, 2 error, 3 duplicates.

Input is UTF-8 dotenv with optional initial BOM, LF or CRLF, assignments,
comments, optional export, and single or double quoted values. No interpolation,
shell execution, or physical multiline values are supported. Files must be
regular and at most 10 MiB. Key names, line numbers and outcomes can reveal
information; values are held in process memory and are not securely erased.

Examples:
  envdiff compare example.env local.env
  envdiff compare --format=json example.env local.env --ignore-values
`

type options struct {
	format   string
	keysOnly bool
	paths    []string
}

func parse(args []string) (options, bool) {
	o := options{format: "text"}
	formatSeen, keysSeen, end := false, false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !end && a == "--" {
			end = true
			continue
		}
		if !end && strings.HasPrefix(a, "-") {
			switch {
			case a == "--ignore-values":
				if keysSeen {
					return o, false
				}
				keysSeen = true
				o.keysOnly = true
			case a == "--format" || strings.HasPrefix(a, "--format="):
				if formatSeen {
					return o, false
				}
				formatSeen = true
				if a == "--format" {
					i++
					if i >= len(args) {
						return o, false
					}
					o.format = args[i]
				} else {
					o.format = strings.TrimPrefix(a, "--format=")
				}
				if o.format != "text" && o.format != "json" {
					return o, false
				}
			default:
				return o, false
			}
			continue
		}
		o.paths = append(o.paths, a)
	}
	return o, len(o.paths) == 2
}
func write(w io.Writer, data []byte) bool {
	n, err := w.Write(data)
	return err == nil && n == len(data)
}
func Run(args []string, stdout, stderr io.Writer, version string) int {
	fail := func(msg string) int { _, _ = fmt.Fprintln(stderr, "Error: "+msg+"."); return 2 }
	if len(args) == 1 && (args[0] == "--help" || args[0] == "--version") {
		out := help
		if args[0] == "--version" {
			out = "envdiff " + version + "\n"
		}
		if !write(stdout, []byte(out)) {
			return fail("Cannot write output")
		}
		return 0
	}
	if len(args) == 2 && args[0] == "compare" && args[1] == "--help" {
		if !write(stdout, []byte(help)) {
			return fail("Cannot write output")
		}
		return 0
	}
	if len(args) == 0 || args[0] != "compare" {
		return fail("Invalid command or arguments")
	}
	o, ok := parse(args[1:])
	if !ok {
		return fail("Invalid command or arguments")
	}
	ref, err := parser.ReadFile(o.paths[0], parser.Reference)
	if err != nil {
		return fail(err.Error())
	}
	target, err := parser.ReadFile(o.paths[1], parser.Target)
	if err != nil {
		return fail(err.Error())
	}
	result := compare.Compare(ref, target, o.keysOnly)
	var data []byte
	if o.format == "json" {
		data, err = report.JSON(result)
		if err != nil {
			return fail("Cannot render report")
		}
	} else {
		data = report.Text(result, version)
	}
	if !write(stdout, data) {
		return fail("Cannot write output")
	}
	switch result.Status {
	case "duplicates":
		return 3
	case "differences":
		return 1
	}
	return 0
}
