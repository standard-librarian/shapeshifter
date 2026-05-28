# ShapeShifter Conformance

This directory defines language-neutral fixtures for ShapeShifter behavior. The Go test runner in this directory is the first consumer; future implementations should consume the same `specs/` and `cases/` files.

## Case Format

Each file in `cases/` is a JSON array:

```json
{
  "name": "v2 mapped request",
  "spec": "users.yaml",
  "route": { "method": "POST", "path": "/users" },
  "contract": "v2",
  "phase": "request",
  "mode": "runtime",
  "input": {
    "full_name": "Alice",
    "contact": { "email": "alice@example.com" }
  },
  "expect": {
    "status": "ok",
    "payload": {
      "name": "Alice",
      "email": "alice@example.com"
    }
  }
}
```

Fields:

- `spec`: file under `specs/`.
- `route`: endpoint route key.
- `contract`: selected contract ID. Empty string means missing header/default selection.
- `phase`: `request`, `response`, or `selection`.
- `mode`: `runtime` or `preview`.
- `input`: JSON object to process. Omitted for `selection` cases.
- `expect.status`: `ok`, `process_error`, or `selection_error`.
- `expect.payload`: expected transformed JSON for successful process cases.
- `expect.skipped_handlers`: expected skipped handlers in preview mode.
- `expect.error_code`: expected ShapeShifter error code for processing errors.
- `expect.stage`: expected processing stage for processing errors.
- `expect.selection_reason`: expected contract selection failure reason.

## Running

```sh
go test ./conformance
```

The root CI should run `go test ./...`, which includes these fixtures.
