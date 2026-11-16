# EnvDiff

EnvDiff compares two local dotenv files and reports key names and structural differences without displaying configuration values. The first file is the reference; keys found only there are missing, and keys found only in the target are additional.

Requires Go 1.26. Build locally with `go build -o envdiff ./cmd/envdiff`. Release builds can embed a version with `go build -ldflags '-X main.version=1.0.0' -o envdiff ./cmd/envdiff`; otherwise the version is `dev`.

```sh
envdiff compare example.env local.env
envdiff compare --format json example.env local.env --ignore-values
envdiff compare -- --reference.env target.env
envdiff --help
envdiff compare --help
envdiff --version
```

`--format text|json` (or `--format=json`) selects output; text is the default. `--ignore-values` compares key sets without comparing values, but still fully validates both files. Compare options can appear before, between or after the two paths. `--` ends option parsing. Options cannot be repeated. Help and version are standalone commands.

Reports use file roles rather than paths, sorted key names, category counts, duplicate occurrence lines and a final status. JSON uses schema version `1.0` and empty arrays for empty categories. Duplicates are reported for each file and take precedence over ordinary differences; duplicated values are not compared. Reports go to stdout; errors go to stderr. If an output write fails, partial stdout may already have been written.

| Exit | Meaning |
| --- | --- |
| 0 | Identical in the selected mode |
| 1 | Differences |
| 2 | Usage, input, parsing or output error |
| 3 | Duplicate keys (complete report is still produced) |

Input must be a readable regular UTF-8 file of at most 10 MiB. Symlinks to regular files are accepted. LF and CRLF, an optional initial UTF-8 BOM, blank lines, comments, `export KEY=value`, and single- or double-quoted values are supported. Keys use ASCII letters, digits and underscores, with a letter or underscore first. Double quotes decode `\"`, `\\`, `\n`, `\r` and `\t`; unknown escapes stay literal. Unquoted inline comments start at a `#` preceded by space or tab. No shell evaluation, interpolation or physical multiline values are supported.

For example, `PORT=3000` and `PORT="3000"` compare equally. All examples are synthetic.

EnvDiff reads files without modifying them and does not output values, paths, hashes or raw input lines. Key names, duplicate line numbers and comparison outcomes can themselves reveal information. Values are held in process memory; this does not protect against OS inspection, swap, crash dumps or a hostile host, and memory is not securely erased. Files should be in trusted directories: a hostile actor replacing a file between the file-type check and open can cause unexpected behaviour, including blocking on a FIFO.

Run `go test ./...` to test. Linux is the currently verified platform; native Windows and macOS verification remains outstanding. MIT licence, copyright (c) 2026 Kenneth Rough.
