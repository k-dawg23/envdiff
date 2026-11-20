# EnvDiff

EnvDiff compares two local dotenv files without displaying their values. Use it to check whether a local configuration has the keys and values expected by an example configuration, or to detect configuration drift in CI. The first file is the **reference**; keys found only there are missing, and keys found only in the second, **target** file are additional.

## Installation

EnvDiff v1.0.0 was released on 20 November 2026. Download `SHA256SUMS` and the archive for your platform from the [v1.0.0 GitHub Release](https://github.com/k-dawg23/envdiff/releases/tag/v1.0.0):

- Linux amd64: `envdiff-v1.0.0-linux-amd64.tar.gz`
- macOS arm64: `envdiff-v1.0.0-darwin-arm64.tar.gz`
- Windows amd64: `envdiff-v1.0.0-windows-amd64.zip`

Verify the archive before extracting it. In a terminal in the directory containing both downloads, use the command for your platform:

```sh
# Linux
sha256sum --ignore-missing --check SHA256SUMS

# macOS (compare the output with the matching line in SHA256SUMS)
shasum -a 256 envdiff-v1.0.0-darwin-arm64.tar.gz
```

```powershell
# Windows PowerShell (compare Hash with the matching line in SHA256SUMS)
Get-FileHash .\envdiff-v1.0.0-windows-amd64.zip -Algorithm SHA256
Get-Content .\SHA256SUMS
```

On Linux, the selected archive should report `OK`; on macOS and Windows, compare the complete hash with its archive's entry in `SHA256SUMS`. A checksum confirms consistency with the manifest, not an independent signature or provenance attestation.

Extract the archive; its top level contains `LICENSE` and `envdiff` (Linux and macOS) or `envdiff.exe` (Windows). Run `./envdiff` on Linux or macOS, or `.\envdiff.exe` in PowerShell, from the directory containing the executable. Alternatively, put the executable in a directory on your `PATH` and invoke it as `envdiff`. Running a compiled executable does not require Go.

Native hosted CI tests and builds passed on Linux amd64, Windows amd64 and macOS arm64, and [release-candidate archives were verified](https://github.com/k-dawg23/envdiff/actions/runs/44064596233). These are automated checks on hosted runners, not human manual smoke tests. macOS amd64 is omitted because native verification is unavailable.

### Build from source

Building requires Go 1.26. From the repository root, run:

```sh
# Linux and macOS
go build -o envdiff ./cmd/envdiff
./envdiff --version
```

```powershell
# Windows PowerShell
go build -o envdiff.exe ./cmd/envdiff
.\envdiff.exe --version
```

A normal source build reports `envdiff dev`. To embed the release version, add `-ldflags '-X main.version=1.0.0'` to the build command. Place the resulting executable on `PATH` if desired.

## Quick start

Create these two synthetic files in your current directory:

`reference.env`:

```dotenv
PORT=3000
API_KEY=example
SHARED=one
```

`target.env`:

```dotenv
PORT="3000"
DEBUG=true
SHARED=two
```

With a locally built executable, run `./envdiff compare reference.env target.env` (PowerShell: `.\envdiff.exe compare reference.env target.env`). The following exact output is from a development build; a version-injected release build displays its version in the first line:

```text
EnvDiff dev
Mode: keys_and_values
Reference: reference
Target: target

Missing keys: 1
Additional keys: 1
Different values: 1
Duplicate keys: 0

Missing:
  API_KEY
Additional:
  DEBUG
Different:
  SHARED
Duplicates:
  (none)

Result: Differences detected
```

Run `./envdiff compare --format=json reference.env target.env` (use `.\envdiff.exe` in PowerShell) for this complete output:

```json
{"schema_version":"1.0","status":"differences","mode":"keys_and_values","summary":{"missing":1,"additional":1,"different":1,"duplicates":0},"missing":["API_KEY"],"additional":["DEBUG"],"different":["SHARED"],"duplicates":[]}
```

Both commands exit 1 because differences were found. `PORT` is equivalent after parsing. All values above are fictitious.

## CLI

The examples below assume `envdiff` is on `PATH`; otherwise use the current-directory invocation above.

```text
envdiff compare [options] <reference> <target>
envdiff --help
envdiff compare --help
envdiff --version
```

- `--format text` or `--format json` selects the report; `--format=json` also works. Text is the default.
- `--ignore-values` compares keys only; both files are still parsed and validated, and duplicates are still reported. The different category is empty.
- Options can appear before, between or after paths. `--` ends option parsing, so subsequent arguments are literal paths, including names beginning with `-`: `envdiff compare -- --reference.env target.env`.
- Exactly two paths are required. Unknown or repeated options, missing option values, unsupported formats and ambiguous help/version combinations are errors. Help and version are standalone invocations.

Key lists are sorted in ASCII order; duplicate entries are sorted by key, then reference before target. Shared values are compared exactly after parsing, case-sensitively and without Unicode normalization. For each duplicated key in each file, the report lists all physical occurrence lines. Duplicated values are not compared; unrelated keys still are. A duplicated key present in only one file is also missing or additional. Counts are numbers of report entries.

| Exit | Meaning |
| --- | --- |
| 0 | No differences in the selected mode |
| 1 | Differences |
| 2 | Usage, input, parsing, execution or output error |
| 3 | Duplicates detected; complete report produced |

Precedence is 2 > 3 > 1 > 0. Reports for 0, 1 and 3 go to stdout with empty stderr; errors go to stderr with empty stdout before output begins. If an output write fails, partial stdout may already have been written. JSON reports contain one object and a newline, including empty arrays for empty categories. Diagnostics use file roles and line numbers where applicable, not paths or raw input.

## Supported dotenv syntax

- Inputs must be readable regular UTF-8 files, at most 10 MiB (10,485,760 bytes) each, including line endings and any BOM. Symlinks to regular files are accepted. Empty files, an optional initial UTF-8 BOM, LF or CRLF and a missing final newline are supported. Invalid UTF-8, NUL bytes and standalone carriage returns are rejected. Directories, devices and pipes are rejected.
- Blank lines and lines whose first non-space/tab character is `#` are ignored. Physical line numbers start at 1 and include blank and comment lines. Syntax whitespace means ASCII space or tab.
- Assignments optionally start with lowercase `export` followed by one or more spaces/tabs. Split at the first `=`; spaces/tabs around the key are trimmed. Keys match `[A-Za-z_][A-Za-z0-9_]*` and are case-sensitive. Bare keys are invalid.
- After `=`, leading spaces/tabs are skipped to determine whether a value is quoted. Unquoted values are trimmed of surrounding spaces/tabs. An unquoted `#` starts a comment only when immediately preceded by a space/tab in the original value portion: `KEY=#abc` contains `#abc`, while `KEY= #abc` is empty. Internal whitespace, backslashes, extra `=` and later quote characters remain literal.
- Single quotes preserve their contents literally; the first subsequent single quote closes the value. Double quotes decode `\"`, `\\`, `\n`, `\r` and `\t`; unknown escapes stay literal. After a closing quote, only spaces/tabs and an optional `#` comment preceded by at least one space/tab are allowed. Adjacent text or `#`, concatenated quoted segments and unterminated quotes are errors.
- Physical multiline values, shell evaluation, variable interpolation, command substitution and environment lookup are unsupported. An escaped newline in a double-quoted value is supported. Empty values are valid.

## Security and limitations

EnvDiff does not modify its input files, access the network or persist configuration values. Reports and diagnostics do not emit values, paths, hashes or raw input lines. A value that is also a valid key name can appear in its role as a key. Key names, duplicate line numbers and comparison outcomes can themselves reveal sensitive information.

Values are held in process memory and are not securely erased. This application-level guarantee does not protect against OS inspection, swap, crash dumps or a hostile host. Use trusted directories: hostile concurrent replacement between the file-type check and opening a file can cause unexpected behaviour, including blocking on a FIFO.

## Development and contributions

With Go 1.26 installed, run `go test ./...` from the repository root. Before submitting changes, format Go code with `gofmt`, run `go vet ./...` and the tests, and use only synthetic fixtures. Contributions can be proposed through GitHub issues and pull requests; describe the change and its verification. The project uses only the Go standard library.

## Licence

[MIT](LICENSE), copyright © 2026 Kenneth Rough.
