# Contributing

Thanks for the interest. Drive-by PRs welcome; open an issue for anything larger.

## Dev loop

```
make test     # go test -race -cover
make vet      # go vet ./...
make lint     # golangci-lint run (install: brew install golangci-lint)
make build    # -> dist/tsk
```

Run the TUI against a throwaway file:

```
mkdir -p /tmp/tsk-play && cd /tmp/tsk-play
go run ./cmd/tsk init
go run ./cmd/tsk
```

## Commit style

Conventional commits: `feat(scope): ...`, `fix(scope): ...`, `docs: ...`, `ci: ...`, `chore: ...`.

Sign-off required:

```
git commit -s -m "feat(cmd): nice thing"
```

## Code expectations

- `go vet ./...` clean.
- `golangci-lint run` clean.
- Parser / store changes land with tests (`go test -race -cover ./...`).
- Public APIs get a doc comment.
- Keep changes bisect-friendly: each commit should build and test cleanly.

## Releasing

Maintainers tag `vX.Y.Z` (annotated). Push the tag; the `release` workflow builds binaries and publishes to GitHub Releases via goreleaser.
