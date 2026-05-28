# ShapeShifter Go Integration Reference

## Install

```sh
go get github.com/standard-librarian/shapeshifter
```

Import the root package plus the framework adapter:

```go
import (
    "github.com/standard-librarian/shapeshifter"
    shapeshifterecho "github.com/standard-librarian/shapeshifter/adapters/echo"
)
```

## Startup Pattern

```go
registry := shapeshifter.NewRegistry()

// Optional handlers must be registered before Snapshot/LoadSpec.
_ = registry.Register("normalizeUserInput", func(input map[string]any) (map[string]any, error) {
    return input, nil
}, shapeshifter.HandlerOptions{PreviewSafe: true})

spec, err := shapeshifter.LoadSpecFile("shapeshifter.yaml", registry.Snapshot())
if err != nil {
    return err
}

engine, err := shapeshifter.NewEngine(spec, shapeshifter.WithObserver(shapeshifter.ObserverFunc(func(event shapeshifter.Event) {
    // Connect to slog/metrics/tracing if the app has them.
})))
if err != nil {
    return err
}
```

## Adapter Mounting

Echo:

```go
e := echo.New()
e.Use(shapeshifterecho.Middleware(engine))
```

Gin:

```go
r := gin.New()
r.Use(shapeshiftergin.Middleware(engine))
```

Chi is route-scoped:

```go
r.With(shapeshifterchi.Route(engine, "/users/{id}")).Post("/users/{id}", createUser)
```

Fiber:

```go
app.Post("/users/:id", shapeshifterfiber.Middleware(engine), createUser)
```

## Minimal Spec

```yaml
version: "1"
title: User API Contracts
description: Contract mappings for the user service.
header: "X-ShapeShifter-Contract"

shapes:
  CreateUserV1Request:
    type: object
    additionalProperties: false
    required: [name, email]
    properties:
      name: { type: string }
      email: { type: string }

  CreateUserV2Request:
    type: object
    additionalProperties: false
    required: [full_name, contact]
    properties:
      full_name: { type: string }
      contact:
        type: object
        additionalProperties: false
        required: [email]
        properties:
          email: { type: string }

  CreateUserInternalRequest:
    type: object
    additionalProperties: false
    required: [name, email]
    properties:
      name: { type: string }
      email: { type: string }

  UserInternalResponse:
    type: object
    additionalProperties: false
    required: [internal_id, name, email]
    properties:
      internal_id: { type: string }
      name: { type: string }
      email: { type: string }

  UserV1Response:
    type: object
    additionalProperties: false
    required: [id, name, email]
    properties:
      id: { type: string }
      name: { type: string }
      email: { type: string }

  UserV2Response:
    type: object
    additionalProperties: false
    required: [id, full_name, contact]
    properties:
      id: { type: string }
      full_name: { type: string }
      contact:
        type: object
        additionalProperties: false
        required: [email]
        properties:
          email: { type: string }

endpoints:
  - path: /users
    method: POST
    summary: Create user
    tags: [users]
    default_contract: v1
    contracts:
      - id: v1
        summary: Internal-compatible request
        request:
          shape: CreateUserV1Request
          target_shape: CreateUserInternalRequest
          examples:
            - name: Basic v1 user
              body: { name: Alice, email: alice@example.com }
          transform:
            passthrough: true
        response:
          source_shape: UserInternalResponse
          shape: UserV1Response
          transform:
            fields:
              - from: ".internal_id"
                to: ".id"
              - from: ".name"
                to: ".name"
              - from: ".email"
                to: ".email"

      - id: v2
        summary: Nested contact request
        request:
          shape: CreateUserV2Request
          target_shape: CreateUserInternalRequest
          examples:
            - name: Basic v2 user
              body:
                full_name: Alice
                contact: { email: alice@example.com }
          transform:
            fields:
              - from: ".full_name"
                to: ".name"
              - from: ".contact.email"
                to: ".email"
        response:
          source_shape: UserInternalResponse
          shape: UserV2Response
          examples:
            - name: Created v2 user
              body:
                id: "123"
                full_name: Alice
                contact: { email: alice@example.com }
          transform:
            fields:
              - from: ".internal_id"
                to: ".id"
              - from: ".name"
                to: ".full_name"
              - from: ".email"
                to: ".contact.email"
```

## Preview API And Portal

Echo preview API:

```go
shapeshifterecho.MountPreviewAPI(e, engine)
```

Embedded portal:

```go
uiHandler := http.StripPrefix(
    "/_shapeshifter/ui",
    ui.Handler(
        ui.WithPreviewAPIBase("/_shapeshifter/api"),
        ui.WithTryItOut(true),
        ui.WithTryItOutBase("/"),
    ),
)
e.GET("/_shapeshifter/ui", func(c echo.Context) error {
    return c.Redirect(http.StatusFound, "/_shapeshifter/ui/")
})
e.GET("/_shapeshifter/ui/*", echo.WrapHandler(uiHandler))
```

Production apps should protect preview/UI routes with their own auth middleware. Try-it-out calls real application handlers and can mutate data.

## Smoke Tests

Use real HTTP tests where possible:

```sh
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -H 'X-ShapeShifter-Contract: v2' \
  -d '{"full_name":"Alice","contact":{"email":"alice@example.com"}}'
```

Expected v2 response shape:

```json
{"id":"123","full_name":"Alice","contact":{"email":"alice@example.com"}}
```

Preview request transform:

```sh
curl -X POST http://localhost:8080/_shapeshifter/api/process/request \
  -H 'Content-Type: application/json' \
  -d '{"route":{"method":"POST","path":"/users"},"contract":"v2","body":{"full_name":"Alice","contact":{"email":"alice@example.com"}}}'
```
