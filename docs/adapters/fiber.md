# Fiber Adapter

Import:

```go
import shapeshifterfiber "github.com/standard-librarian/shapeshifter/adapters/fiber"
```

Mount in the route chain:

```go
app.Post("/users/:id", shapeshifterfiber.Middleware(engine), createUser)
```

Route key:

- method: `c.Method()`
- path: `c.Route().Path`

Use Fiber route patterns in `shapeshifter.yaml`, for example `/users/:id`.

Notes:

- Mount the middleware where the matched route is available.
- The adapter replaces the request body with the transformed internal JSON before calling the next handler.
- After `c.Next()`, it inspects and replaces the buffered Fiber response body when the response is eligible JSON.
- Non-JSON, encoded, `HEAD`, `204`, `304`, and `>=400` responses bypass transformation.
