# EnvDiff

EnvDiff is a local Go utility in development for comparing two dotenv files without reporting their values. Stage 1 implements the parser and comparison engine; a command-line interface and releases are not yet available.

The parser uses a restricted dotenv grammar and does not evaluate shell expressions or interpolate variables. Comparison results contain key names, duplicate line numbers and outcomes, which can themselves disclose information. Values remain in process memory; this is not protection from a hostile host, swap, crash dumps or OS inspection. Input files are read only.

Development requires Go 1.26. Run `go test ./...` to test the packages. Synthetic fixtures are under `testdata`.

MIT licence. Copyright (c) 2026 Kenneth Rough.
