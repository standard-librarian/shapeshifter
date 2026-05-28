# Release And Stability

ShapeShifter is currently pre-1.0. The project should not tag a public release until CI is green from a clean clone and the conformance suite covers the behavior expected from downstream users.

## Stability Levels

Stable for v1 compatibility:

- `shapeshifter.yaml` `version: "1"` semantics documented in `docs/spec-v1.md`
- core request/response transform order
- contract selection behavior
- processing error phases, stages, and error codes
- preview API JSON shapes documented in `docs/http-api.md`
- sanitized spec JSON fields documented in `docs/http-api.md`
- UI `config.json`
- conformance fixtures under `conformance/`

Pre-1.0 and still allowed to evolve:

- portal layout and visual design
- exact wording of documentation metadata
- observer event payload additions
- adapter implementation internals
- future adapter examples

Breaking changes require one of:

- a new spec `version`
- a clearly documented compatibility exception
- a conformance fixture update explaining the changed expected behavior

## Release Checklist

Before tagging:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Also verify:

- Echo example starts from a clean checkout.
- `GET /_shapeshifter/api/spec` returns sanitized metadata and examples.
- `POST /_shapeshifter/api/process/request` works for the v2 example.
- `POST /_shapeshifter/api/process/response` works for the v2 example.
- `/_shapeshifter/ui/` loads without external CDN or React runtime dependencies.

Recommended first tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Do not tag automatically from automation until the release checklist has been run intentionally.
