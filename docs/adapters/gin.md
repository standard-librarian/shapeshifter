# Gin Adapter

Import:

```go
import shapeshiftergin "github.com/standard-librarian/shapeshifter/adapters/gin"
```

Mount:

```go
r := gin.New()
r.Use(shapeshiftergin.Middleware(engine))
```

Route key:

- method: `c.Request.Method`
- path: `c.FullPath()`

Use Gin route patterns in `shapeshifter.yaml`, for example `/users/:id`.

Notes:

- The adapter wraps Gin's response writer to buffer eligible JSON responses.
- It preserves Gin response-writer status, size, and written-state behavior expected by handlers.
- Non-JSON, encoded, `HEAD`, `204`, `304`, and `>=400` responses bypass transformation.
