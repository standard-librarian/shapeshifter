# Public API Surface

This document records the public API reviewed for the first pre-1.0 release. ShapeShifter is still pre-1.0, but these names are intended to be coherent enough for `v0.1.0`.

## Root Package

Import:

```go
import "github.com/standard-librarian/shapeshifter"
```

Primary startup APIs:

- `NewRegistry() *Registry`
- `(*Registry).Register(name string, fn HandlerFunc, opts ...HandlerOptions) error`
- `(*Registry).Snapshot() HandlerSnapshot`
- `LoadSpec(r io.Reader, handlers HandlerSnapshot, opts ...Option) (*Spec, error)`
- `LoadSpecBytes(data []byte, handlers HandlerSnapshot, opts ...Option) (*Spec, error)`
- `LoadSpecFile(path string, handlers HandlerSnapshot, opts ...Option) (*Spec, error)`
- `NewEngine(spec *Spec, opts ...Option) (*Engine, error)`
- `WithObserver(observer Observer) Option`

Primary runtime APIs:

- `(*Engine).HeaderName() string`
- `(*Engine).HasEndpoint(route RouteKey) bool`
- `(*Engine).Contracts(route RouteKey) []string`
- `(*Engine).ResolveContract(route RouteKey, headerValue string) (ContractSelection, error)`
- `(*Engine).ProcessRequest(ctx context.Context, sel ContractSelection, input []byte, mode ProcessMode) (ProcessResult, error)`
- `(*Engine).ProcessResponse(ctx context.Context, sel ContractSelection, input []byte, mode ProcessMode) (ProcessResult, error)`
- `(*Engine).SanitizedSpec() SanitizedSpec`
- `(*Engine).Emit(event Event)`

Core public types:

- `RouteKey`
- `Limits`
- `ContractSelection`
- `ProcessResult`
- `ProcessMode`
- `HandlerFunc`
- `HandlerOptions`
- `RegisteredHandler`
- `HandlerSnapshot`
- `Registry`
- `Observer`
- `ObserverFunc`
- `Event`
- `EventKind`
- `BypassReason`
- `ValidationError`
- `ErrorCode`
- `Phase`
- `Stage`
- `ShapeShifterError`
- `HandlerValidationError`
- `ContractSelectionError`
- `ContractSelectionReason`
- `SanitizedSpec` and nested sanitized spec structs

Stable constants:

- default limits
- process modes
- phases and stages
- error codes
- contract selection reasons
- observer event kinds
- bypass reasons

Known pre-1.0 caveat:

- `Option` is shared by spec loading and engine construction today, but only `WithObserver` affects `NewEngine`. Keep accepting it for loader compatibility; avoid adding loader-only behavior without documenting it.

## Echo Adapter

Import:

```go
import shapeshifterecho "github.com/standard-librarian/shapeshifter/adapters/echo"
```

Public APIs:

- `Middleware(engine *shapeshifter.Engine) echo.MiddlewareFunc`
- `MountPreviewAPI(router PreviewRouter, engine *shapeshifter.Engine, opts ...PreviewOption)`
- `WithPreviewPrefix(prefix string) PreviewOption`
- `DefaultPreviewPrefix`
- `PreviewRouter`

## Gin Adapter

Import:

```go
import shapeshiftergin "github.com/standard-librarian/shapeshifter/adapters/gin"
```

Public API:

- `Middleware(engine *shapeshifter.Engine) gin.HandlerFunc`

## Chi Adapter

Import:

```go
import shapeshifterchi "github.com/standard-librarian/shapeshifter/adapters/chi"
```

Public API:

- `Route(engine *shapeshifter.Engine, pattern string) func(http.Handler) http.Handler`

Global Chi middleware is intentionally not exposed.

## Fiber Adapter

Import:

```go
import shapeshifterfiber "github.com/standard-librarian/shapeshifter/adapters/fiber"
```

Public API:

- `Middleware(engine *shapeshifter.Engine) fiber.Handler`

## UI Package

Import:

```go
import "github.com/standard-librarian/shapeshifter/ui"
```

Public APIs:

- `Handler(opts ...Option) http.Handler`
- `WithPreviewAPIBase(path string) Option`
- `WithTryItOut(enabled bool) Option`
- `WithTryItOutBase(path string) Option`

The UI package serves static assets and `config.json`; the host application owns auth.
