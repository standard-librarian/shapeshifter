# ShapeShifter

ShapeShifter is a Go middleware library that adapts JSON HTTP contracts at the edge of your application. Controllers keep reading and writing one canonical internal JSON shape, while ShapeShifter maps external request and response contracts around them.

```text
external request -> ShapeShifter -> internal controller request
internal response -> ShapeShifter -> external response
```

Current capabilities:

- Core transform engine with request and response pipelines
- YAML/JSON spec loader with load-time schema, jq, path, handler, and contract validation
- JSON Schema 2020-12 validation through `github.com/santhosh-tekuri/jsonschema/v5`
- jq source expressions through `github.com/itchyny/gojq`
- Echo adapter, preview API, and example app
- Gin adapter
- Route-scoped Chi adapter
- Fiber adapter
- Embedded static UI for previewing request and response transforms
- No-op-by-default observer hook with a simple `ObserverFunc` helper

## Install

```sh
go get github.com/standard-librarian/shapeshifter
```

## Quick Start

```go
registry := shapeshifter.NewRegistry()

spec, err := shapeshifter.LoadSpecFile("shapeshifter.yaml", registry.Snapshot())
if err != nil {
    return err
}

engine, err := shapeshifter.NewEngine(spec)
if err != nil {
    return err
}

e := echo.New()
e.Use(shapeshifterecho.Middleware(engine))
e.POST("/users", createUser)
```

The selected contract is endpoint-scoped. A `v2` contract on `POST /users` is independent from a `v2` contract on another route.

## Example

Run the Echo example:

```sh
cd examples/echo-example
go run .
```

Default `v1` request:

```sh
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

Explicit `v2` request:

```sh
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -H 'X-ShapeShifter-Contract: v2' \
  -d '{"full_name":"Alice","contact":{"email":"alice@example.com"}}'
```

The controller sees this internal request in both cases:

```json
{"name":"Alice","email":"alice@example.com"}
```

For `v2`, the client receives:

```json
{"id":"123","full_name":"Alice","contact":{"email":"alice@example.com"}}
```

## Preview API

Echo can mount preview endpoints for local tooling or an authenticated internal console:

```go
shapeshifterecho.MountPreviewAPI(e, engine)
```

Default routes:

- `GET /_shapeshifter/api/spec`
- `POST /_shapeshifter/api/process/request`
- `POST /_shapeshifter/api/process/response`

Preview request transform:

```sh
curl -X POST http://localhost:8080/_shapeshifter/api/process/request \
  -H 'Content-Type: application/json' \
  -d '{"route":{"method":"POST","path":"/users"},"contract":"v2","body":{"full_name":"Alice","contact":{"email":"alice@example.com"}}}'
```

Preview response transform:

```sh
curl -X POST http://localhost:8080/_shapeshifter/api/process/response \
  -H 'Content-Type: application/json' \
  -d '{"route":{"method":"POST","path":"/users"},"contract":"v2","body":{"internal_id":"123","name":"Alice","email":"alice@example.com"}}'
```

Preview processing uses `ModePreview`. Handlers registered without `PreviewSafe: true` are skipped and returned in `skipped_handlers`.

## Embedded UI

The embedded UI is static and opt-in. It uses the preview API, so mount both behind your own auth for non-local environments.

```go
shapeshifterecho.MountPreviewAPI(e, engine)

uiHandler := http.StripPrefix("/_shapeshifter/ui", ui.Handler())
e.GET("/_shapeshifter/ui", func(c echo.Context) error {
    return c.Redirect(http.StatusFound, "/_shapeshifter/ui/")
})
e.GET("/_shapeshifter/ui/*", echo.WrapHandler(uiHandler))
```

Open:

```text
http://localhost:8080/_shapeshifter/ui/
```

The UI reads `GET /_shapeshifter/api/spec`, renders request and response input forms from JSON Schemas, runs preview transforms, and surfaces skipped unsafe handlers.

## Handlers

Handlers run after field mapping and coercion. They may mutate and return the target map.

```go
registry := shapeshifter.NewRegistry()

err := registry.Register("normalizeUserInput", func(input map[string]any) (map[string]any, error) {
    if name, ok := input["name"].(string); ok {
        input["name"] = strings.TrimSpace(name)
    }
    return input, nil
}, shapeshifter.HandlerOptions{PreviewSafe: true})
```

`LoadSpec` takes `registry.Snapshot()`, so handlers registered after loading do not affect an already compiled spec.

## Observer

ShapeShifter does not log directly. Use an observer to connect it to your logger, metrics, or tracing backend.

```go
observer := shapeshifter.ObserverFunc(func(event shapeshifter.Event) {
    slog.Info("shapeshifter",
        "kind", event.Kind,
        "route", event.Route,
        "contract", event.ContractID,
        "phase", event.Phase,
        "stage", event.Stage,
        "duration", event.Duration,
        "in_bytes", event.InBytes,
        "out_bytes", event.OutBytes,
        "reason", event.Reason,
    )
})

engine, err := shapeshifter.NewEngine(spec, shapeshifter.WithObserver(observer))
```

The Echo example includes a concrete observer that logs selected contracts, bypasses, transform success, validation failures, and handler failures.

## Adapters

Echo uses `c.Path()`:

```go
e := echo.New()
e.Use(shapeshifterecho.Middleware(engine))
```

Gin uses `c.FullPath()`:

```go
r := gin.New()
r.Use(shapeshiftergin.Middleware(engine))
```

Chi is route-scoped because Chi does not reliably expose the route pattern early enough for global request-body transformation:

```go
r.With(shapeshifterchi.Route(engine, "/users/{id}")).Post("/users/{id}", handler)
```

Fiber uses `c.Route().Path`. Mount it in the route handler chain so the matched route path is available before request transformation:

```go
app.Post("/users/:id", shapeshifterfiber.Middleware(engine), handler)
```

## Spec Rules

- `version` is required and must be `"1"`.
- `method` is normalized to uppercase.
- `path` is the adapter-native route pattern.
- Duplicate `method + path` endpoints are rejected.
- Duplicate contract IDs within one endpoint are rejected.
- Missing contract headers use `default_contract` only when it is explicitly configured.
- Unknown contract headers return `400`.
- JSON Schema `$ref` is rejected in this version.
- Request passthrough requires root `additionalProperties: false`.
- Response passthrough is not supported; responses must use explicit `fields`.
- Target paths support object paths like `.contact.email`.
- jq programs are trusted spec configuration and are compiled at load time.
- `gojq.RunWithContext` cancellation is available; default runtime guardrails enforce output-count and emitted-value-size limits.
- The sanitized preview spec includes JSON Schemas and handler metadata, but not handler functions or compiled internals.

## HTTP Behavior

Request side:

- Missing or unknown contract: `400`
- Invalid request `Content-Type`: `400`
- Malformed JSON, empty body, root array, or scalar root: `400`
- Request body above limit: `413`
- Valid JSON that fails schema, validation, mapping, coercion, or handler validation: `422`
- Ordinary request handler system failure: `500`

Response side:

- Only buffered JSON responses are transformed.
- `HEAD`, `204`, `304`, `>=400`, non-JSON content types, and non-identity content encodings bypass transformation.
- Response validation, transform, handler, target validation, marshal, or response-size failures return a controlled `500` JSON envelope.
- On response transform failure, stale content headers are cleared and only safe correlation headers are preserved.

## Limits

Defaults:

```go
const (
    DefaultRequestBodyBytes  int64 = 65536
    DefaultResponseBodyBytes int64 = 1048576
)
```

Endpoint limits override top-level limits. Zero and negative limits are invalid.

## Tests

```sh
go test ./...
go test -race ./...
go vet ./...
```

The suite covers loader validation, transform semantics, JSON eligibility, request and response limits, real HTTP server behavior, preview processing, observer delivery, adapter behavior, concurrency, and fuzz seeds for paths and number normalization.

## Current Non-Goals

- Streaming responses, SSE, websockets, and hijacked connections
- Multipart/file upload transformation
- Root arrays or scalar JSON roots
- Cross-framework route pattern normalization
- Independent request-contract and response-contract selectors
- Arbitrary jq mutation/update programs
- Response passthrough
