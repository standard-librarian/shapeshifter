# ShapeShifter Spec v1

This document describes the stable ShapeShifter YAML contract format for `version: "1"`.

## Top Level

```yaml
version: "1"
title: User API Contracts
description: Contract mappings for the user service.
header: "X-ShapeShifter-Contract"
limits:
  request_body_bytes: 65536
  response_body_bytes: 1048576
shapes: {}
endpoints: []
```

Rules:

- `version` is required and must equal `"1"`.
- `title` and `description` are optional documentation metadata.
- `header` defaults to `X-ShapeShifter-Contract`.
- `limits` are optional. Defaults are `65536` request bytes and `1048576` response bytes.
- Zero or negative limits are invalid.
- `shapes` must contain JSON object schemas.
- JSON Schema `$ref` is rejected in v1.

## Endpoints

```yaml
endpoints:
  - path: /users
    method: POST
    summary: Create user
    description: Creates a user.
    tags: [users]
    default_contract: v1
    limits:
      request_body_bytes: 65536
    contracts: []
```

Rules:

- `method` is normalized to uppercase.
- `path` is the adapter-native route pattern.
- Duplicate `method + path` endpoints are invalid.
- Endpoint limits override top-level limits.
- `summary`, `description`, and `tags` are optional documentation metadata.
- `default_contract` is optional. If present, it must match a contract ID on the endpoint.

## Contracts

```yaml
contracts:
  - id: v2
    summary: Nested contact request
    description: Web-app public contract.
    deprecated: false
    request: {}
    response: {}
```

Rules:

- Contract IDs are unique within one endpoint only.
- Each contract must define `request`, `response`, or both.
- `summary`, `description`, and `deprecated` are optional metadata.

## Request Side

```yaml
request:
  description: External client request body.
  shape: CreateUserV2Request
  target_shape: CreateUserInternalRequest
  examples:
    - name: Basic user
      body:
        full_name: Alice
        contact:
          email: alice@example.com
  transform:
    fields:
      - from: ".full_name"
        to: ".name"
    validate:
      - field: ".contact.email"
        rule: "test(\"^[^@]+@[^@]+$\")"
        error: "invalid email format"
```

Rules:

- `shape` is required and validates the external client request.
- `target_shape` is optional and validates the internal controller request after transform.
- Request transforms must define explicit `fields` unless `passthrough: true` is set.
- Request passthrough requires root `additionalProperties: false` on the request shape.
- Request examples are optional and validate against `shape` at load time.
- Example names must be non-empty and unique within the request side.

## Response Side

```yaml
response:
  description: External client response body.
  source_shape: UserInternalResponse
  shape: UserV2Response
  examples:
    - name: Created user
      body:
        id: "123"
        full_name: Alice
  transform:
    fields:
      - from: ".internal_id"
        to: ".id"
```

Rules:

- `shape` is required and validates the external client response after transform.
- `source_shape` is optional and validates the internal controller response before transform.
- Response transforms must define explicit `fields`.
- Response passthrough is unsupported in v1.
- Response examples are optional and validate against external response `shape`.

## Transform Semantics

Order:

1. Decode JSON object with `json.Decoder.UseNumber`.
2. Validate source JSON Schema.
3. Normalize numbers before jq and handlers.
4. Run source validation rules.
5. Build a new target object from `fields` or request passthrough.
6. Apply target coercions.
7. Run optional handler.
8. Validate target JSON Schema when configured.
9. Marshal target JSON.

Field mapping:

- `from` is a trusted jq source expression compiled at load time.
- `from` must emit exactly one value.
- `to` is an object path parsed by ShapeShifter, not jq.
- `to` grammar is `.segment(.segment)*`, with identifier segments only.
- `required` defaults to `true`.
- `nil` values are omitted only when `required: false`.

Coercion:

- `field` uses the same object path grammar as `to`.
- Supported types are `integer`, `number`, `string`, and `boolean`.

Handlers:

- Handlers run after mapping/coercion.
- `LoadSpec` accepts `HandlerSnapshot`; registrations after snapshot do not affect loaded specs.
- Preview mode calls only handlers registered with `PreviewSafe: true`.

## Stable JSON Surfaces

The following are treated as public compatibility surfaces for v1:

- `shapeshifter.yaml`
- error envelope JSON
- preview API request/response JSON
- sanitized spec JSON
- UI `config.json`
- conformance fixtures in `conformance/`
