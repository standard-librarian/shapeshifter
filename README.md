# ShapeShifter

ShapeShifter is a Go middleware library for adapting JSON request and response bodies between external HTTP contracts and a canonical internal controller shape.

Request direction:

```text
external client request -> ShapeShifter -> internal controller request
```

Response direction:

```text
internal controller response -> ShapeShifter -> external client response
```

MVP 1 includes the core engine, YAML/JSON spec loader, buffered JSON response transformation, and an Echo adapter. It intentionally does not include preview APIs, an embedded UI, Gin, Chi, or Fiber.

## Install

```sh
go get github.com/standard-librarian/shapeshifter
```

## Echo Usage

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
```

Controllers read and write only the internal JSON shape. The selected endpoint-scoped contract controls both request and response transformation.

## Additional Adapters

Gin uses `c.FullPath()`:

```go
r := gin.New()
r.Use(shapeshiftergin.Middleware(engine))
```

Chi is route-scoped in this MVP because Chi does not reliably expose the route pattern early enough for global request-body transformation:

```go
r.With(shapeshifterchi.Route(engine, "/users/{id}")).Post("/users/{id}", handler)
```

Fiber uses `c.Route().Path`. Mount it in the route handler chain so the matched route path is available before request transformation:

```go
app.Post("/users/:id", shapeshifterfiber.Middleware(engine), handler)
```

## Preview API

MVP 2 adds opt-in Echo preview endpoints. Mount them only behind your own authentication or local tooling.

```go
shapeshifterecho.MountPreviewAPI(e, engine)
```

Default routes:

- `GET /_shapeshifter/api/spec`
- `POST /_shapeshifter/api/process/request`
- `POST /_shapeshifter/api/process/response`

Process request body:

```json
{
  "route": { "method": "POST", "path": "/users" },
  "contract": "v2",
  "body": { "full_name": "Alice", "contact": { "email": "alice@example.com" } }
}
```

Preview processing uses `ModePreview`. Handlers registered without `PreviewSafe: true` are skipped and returned in `skipped_handlers`.

## Spec Notes

- `version` must be `"1"`.
- Endpoints are selected by method plus adapter-native route path.
- Contract IDs are unique within one endpoint.
- Missing contract headers use `default_contract` only when it is configured.
- JSON Schema `$ref` is rejected in MVP 1.
- Request passthrough requires root `additionalProperties: false`.
- Response passthrough is not supported; responses must use explicit `fields`.
- jq programs are trusted spec configuration and are compiled at load time.
- `gojq.RunWithContext` cancellation was verified during implementation, but default runtime limits rely on output-count and emitted-value-size bounds.
- Response transformation is buffered JSON only. Streaming, SSE, websockets, hijacking, multipart uploads, and non-Echo adapters are intentionally out of MVP 1.

## Tests

```sh
go test ./...
go test -race ./...
```

The test suite covers core request/response transforms, loader validation, response eligibility, request limiting, Echo real-server behavior, and concurrent engine use.
