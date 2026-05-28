# Echo Adapter

Import:

```go
import shapeshifterecho "github.com/standard-librarian/shapeshifter/adapters/echo"
```

Mount:

```go
e := echo.New()
e.Use(shapeshifterecho.Middleware(engine))
```

Route key:

- method: `c.Request().Method`
- path: `c.Path()`

Use Echo route patterns in `shapeshifter.yaml`, for example `/users/:id`.

Preview API:

```go
shapeshifterecho.MountPreviewAPI(e, engine)
```

The Echo example in `examples/echo-example` is the reference implementation for middleware, preview API, portal mounting, and observer logging.
