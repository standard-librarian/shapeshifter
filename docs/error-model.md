# ShapeShifter Error Model

ShapeShifter separates contract selection errors from processing errors. HTTP adapters should use the same envelopes so client and preview tooling can render errors consistently.

## Contract Selection

Missing contract header when no default exists:

```json
{
  "error": "contract selection failed",
  "code": "missing_contract_header",
  "route": { "method": "POST", "path": "/users" },
  "header": "X-ShapeShifter-Contract",
  "available_contracts": ["v1", "v2"]
}
```

Unknown contract:

```json
{
  "error": "contract selection failed",
  "code": "unknown_contract",
  "route": { "method": "POST", "path": "/users" },
  "header": "X-ShapeShifter-Contract",
  "contract": "v9",
  "available_contracts": ["v1", "v2"]
}
```

Selection reasons:

- `missing`
- `unknown`
- `no_endpoint`

HTTP status:

- configured endpoint with missing/unknown contract: `400`
- no endpoint: adapter bypass, not a client error

## Processing Errors

Envelope:

```json
{
  "error": "contract processing failed",
  "route": { "method": "POST", "path": "/users" },
  "contract": "v2",
  "phase": "request",
  "stage": "source_validate",
  "details": [
    {
      "field": ".contact.email",
      "message": "invalid email format",
      "code": "validation_rule_failed"
    }
  ]
}
```

`details` is omitted when there are no field-level errors.

## Phases

- `request`
- `response`

## Stages

- `decode`
- `source_validate`
- `number_normalize`
- `transform`
- `handler`
- `target_validate`
- `marshal`

## Error Codes

- `malformed_json`
- `empty_request_body`
- `unsupported_content_type`
- `request_too_large`
- `source_schema_failed`
- `validation_rule_failed`
- `missing_required_field`
- `multiple_jq_outputs`
- `number_normalization_failed`
- `coercion_failed`
- `handler_validation_failed`
- `handler_failed`
- `target_schema_failed`
- `marshal_failed`
- `response_too_large`
- `missing_contract_header`
- `unknown_contract`

## Status Mapping

Request side:

- invalid content type: `400`
- malformed JSON, empty body, non-object root: `400`
- request body too large: `413`
- source schema, validation, mapping, coercion, target schema, or handler validation failure: `422`
- ordinary handler system failure: `500`

Response side:

- source schema, validation, mapping, coercion, handler, target schema, marshal, or response-size failure: `500`

## Sensitive Data

`ShapeShifterError.Error()` is stable and non-sensitive. It does not include raw causes. Adapters must not include raw internal error strings in `500` HTTP bodies by default. Use the observer hook for internal logging and metrics.

## Response Transform Failure Cleanup

If a response transform fails after the controller writes into the capture buffer, adapters must discard the captured body and return a clean JSON error response.

Required cleanup:

- clear `Content-Encoding`
- clear `Content-Length`
- clear `Transfer-Encoding`
- set `Content-Type: application/json`
- set `Cache-Control: no-store`
- preserve only safe correlation headers such as `Request-ID`, `X-Request-ID`, and `Traceparent`
