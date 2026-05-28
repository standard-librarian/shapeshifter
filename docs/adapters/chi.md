# Chi Adapter

Import:

```go
import shapeshifterchi "github.com/standard-librarian/shapeshifter/adapters/chi"
```

Mount route-scoped middleware:

```go
r.With(shapeshifterchi.Route(engine, "/users/{id}")).Post("/users/{id}", createUser)
```

Route key:

- method: `r.Method`
- path: the pattern passed to `shapeshifterchi.Route`

Use Chi route patterns in `shapeshifter.yaml`, for example `/users/{id}`.

Important limitation:

Do not mount ShapeShifter as ordinary global Chi middleware. Chi's route pattern is not reliably available early enough for request-body transformation in global middleware. Use the route-scoped API so ShapeShifter knows the route pattern before reading and replacing the request body.
