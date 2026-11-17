# Changelog

## v1.0.0 — 20 November 2026 (planned, unreleased)

- Initial local comparison of two dotenv files, reporting missing and additional keys and differing parsed values without displaying values.
- Text and JSON reports, keys-only mode, help and version commands, predictable exit codes and safe diagnostics. Duplicate definitions are reported with every occurrence line; their values are not compared. Errors take precedence over duplicates, then differences.
- Restricted UTF-8 dotenv grammar supports comments, optional `export`, unquoted and quoted values, selected double-quote escapes, LF and CRLF, and files up to 10 MiB.
- Reports omit values, paths and hashes; input files are not modified. Key names, line numbers and comparison outcomes may still be sensitive. Values remain in process memory and are not securely erased.
- Native hosted CI tests and builds passed on Linux amd64, Windows amd64 and macOS arm64; release-candidate archives were verified on hosted runners. These are the planned initial release targets; release artifacts have not yet been published. macOS amd64 is omitted because native verification is unavailable.
- No shell evaluation, interpolation or physical multiline values; only two regular files can be compared. Hostile concurrent file replacement is outside the supported threat model. See the [README](README.md) for syntax and security limitations.
