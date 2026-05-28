---
name: shapeshifter-go
description: Install and integrate the ShapeShifter Go middleware library in existing Go HTTP APIs. Use when adding contract-version JSON request/response transformation, authoring shapeshifter.yaml specs, mounting Echo/Gin/Chi/Fiber middleware, enabling the preview API or embedded contract portal, registering handlers, or debugging ShapeShifter integration in a Go repo.
---

# ShapeShifter Go

## Workflow

1. Inspect the target repo first: identify the router framework, route patterns, controller JSON shapes, module path, existing middleware layout, and test style.
2. Add the library with `go get github.com/standard-librarian/shapeshifter` unless the repo already uses a local replace or workspace.
3. Create or update `shapeshifter.yaml` near application startup. Keep controllers canonical: request transforms map external JSON into internal controller JSON; response transforms map internal controller JSON back to external JSON.
4. Register deterministic transform handlers before loading the spec. Pass `registry.Snapshot()` to `LoadSpecFile`; do not pass a mutable registry to the loader.
5. Load the spec at startup, create an engine, and mount the correct adapter middleware for the framework.
6. Add focused tests: one successful default contract request, one explicit newer contract request, one validation failure, and one response transform assertion.
7. If preview or the portal is requested, mount the preview API and UI behind application-owned auth. Enable UI try-it-out only for local/dev or protected internal environments.

## Framework Choice

Use the adapter that matches the repo:

- Echo: `github.com/standard-librarian/shapeshifter/adapters/echo`
- Gin: `github.com/standard-librarian/shapeshifter/adapters/gin`
- Chi: `github.com/standard-librarian/shapeshifter/adapters/chi`
- Fiber: `github.com/standard-librarian/shapeshifter/adapters/fiber`

For detailed snippets, spec examples, and portal setup, read `references/integration.md`.

## Integration Rules

- Use adapter-native route patterns in the spec: Echo `c.Path()`, Gin `c.FullPath()`, route-scoped Chi pattern, or Fiber `c.Route().Path`.
- Keep contract IDs route-scoped. `v2` on `POST /users` is independent from `v2` on another endpoint.
- Define `default_contract` only when missing headers should be accepted. Without it, a missing contract header is a `400`.
- Keep response transforms explicit with `fields`; do not rely on passthrough for responses.
- Use `passthrough: true` for requests only when the external request shape exactly matches the controller request shape and has root `additionalProperties: false`.
- Register preview-safe handlers with `shapeshifter.HandlerOptions{PreviewSafe: true}` only when they are deterministic and safe to run in preview mode.
- Do not expose preview API or UI publicly without the host app's auth.

## Validation

Run the repo's normal checks plus at least:

```sh
go test ./...
go test -race ./...
go vet ./...
```

When adding the portal, manually verify `GET /_shapeshifter/api/spec`, `POST /_shapeshifter/api/process/request`, `POST /_shapeshifter/api/process/response`, and the UI path mounted by the app.
