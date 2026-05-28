# ShapeShifter HTTP APIs

ShapeShifter exposes optional HTTP APIs for preview tooling and the embedded contract portal. Applications own authentication and authorization for these routes.

## Preview API

Echo mounts the preview API with:

```go
shapeshifterecho.MountPreviewAPI(e, engine)
```

Default prefix: `/_shapeshifter/api`.

Routes:

- `GET /_shapeshifter/api/spec`
- `POST /_shapeshifter/api/process/request`
- `POST /_shapeshifter/api/process/response`

### Sanitized Spec

`GET /spec` returns the loaded spec without handler functions or compiled internals.

Top-level shape:

```json
{
  "version": "1",
  "title": "User API Contracts",
  "description": "Contract mappings for the user service.",
  "header": "X-ShapeShifter-Contract",
  "shapes": ["CreateUserV2Request"],
  "shape_schemas": {},
  "endpoints": []
}
```

The sanitized spec includes:

- endpoint route, summary, description, tags, defaults, and limits
- contract metadata and deprecation flag
- request/response shape names
- curated examples
- transform fields, validation rules, coercions, and handler metadata

It does not include:

- handler functions
- compiled jq programs
- compiled JSON Schema internals
- private Go implementation details

### Preview Processing

Request:

```json
{
  "route": { "method": "POST", "path": "/users" },
  "contract": "v2",
  "body": {
    "full_name": "Alice",
    "contact": { "email": "alice@example.com" }
  }
}
```

`payload` is accepted as an alias for `body`.

Success response:

```json
{
  "route": { "method": "POST", "path": "/users" },
  "contract": "v2",
  "phase": "request",
  "payload": {
    "name": "Alice",
    "email": "alice@example.com"
  },
  "skipped_handlers": ["unsafeHandler"]
}
```

Preview processing always uses `ModePreview`. Unsafe handlers are skipped and reported in `skipped_handlers`.

## Embedded Portal

The portal is served by:

```go
uiHandler := http.StripPrefix(
    "/_shapeshifter/ui",
    ui.Handler(
        ui.WithPreviewAPIBase("/_shapeshifter/api"),
        ui.WithTryItOut(false),
        ui.WithTryItOutBase("/"),
    ),
)
```

Routes under the UI mount:

- `GET /`
- `GET /app.js`
- `GET /styles.css`
- `GET /config.json`

`config.json` response:

```json
{
  "preview_api_base": "/_shapeshifter/api",
  "try_it_out_enabled": false,
  "try_it_out_base": "/"
}
```

Try-it-out:

- Disabled by default.
- Enabled only with `ui.WithTryItOut(true)`.
- Sends real same-origin requests with `credentials: "same-origin"`.
- Can mutate application data for non-idempotent methods.

## Runtime Middleware Behavior

Request transformation is eligible only when the selected contract has a request side:

- request body must be present and within the configured limit
- `Content-Type` must be `application/json` or a `+json` media type
- malformed JSON and non-object roots return `400`
- valid JSON contract failures return `422`

Response transformation is eligible only when the selected contract has a response side:

- method is not `HEAD`
- status is `>= 200` and `< 400`
- status is not `204` or `304`
- content type is `application/json` or a `+json` media type
- content encoding is absent or `identity`
- captured body is within the configured limit

Non-eligible responses bypass transformation unless the response body exceeds the configured response limit, which returns a controlled transform error.
