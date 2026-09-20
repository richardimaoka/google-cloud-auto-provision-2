# Sample responses

`googlecloud/samples/data/` holds recorded API responses. They must never contain real identifiers (project numbers, organizations, project IDs, email addresses, and so on).

- Record a response with a script in `googlecloud/samples/scripts/`. It saves the raw response to `googlecloud/samples/scripts/out/`. That directory is git-ignored because the responses hold real identifiers. Never commit anything from it.
- Before a sample goes into `googlecloud/samples/data/`, replace the identifiers: `go run ./cmd/anonymize googlecloud/samples/scripts/out/<name>.json`. It writes the result to `googlecloud/samples/data/`. Do not copy a raw response there.
- Check the result with `go test ./cmd/anonymize` before committing. The pre-commit hook `.githooks/pre-commit` runs it when a sample is staged. Enable the hook once per clone with `git config core.hooksPath .githooks`, and do not bypass it with `--no-verify`.
- The anonymize tool does not handle the groups samples. Replace their identifiers by hand.
- Name samples and list them in `googlecloud/samples/data/README.md` as described there.
