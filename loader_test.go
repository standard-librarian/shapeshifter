package shapeshifter

import (
	"strings"
	"testing"
)

func TestLoadSpecValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want string
	}{
		{"unknown version", `version: "2"`, "unsupported spec version"},
		{"ref rejected", `
version: "1"
shapes:
  S:
    type: object
    properties:
      x: { "$ref": "#/defs/X" }
endpoints: []
`, "$ref"},
		{"duplicate endpoint", minimalSpec(`
  - path: /x
    method: post
    contracts: [ { id: v1, request: { shape: S, transform: { passthrough: true } } } ]
  - path: /x
    method: POST
    contracts: [ { id: v1, request: { shape: S, transform: { passthrough: true } } } ]
`), "duplicate endpoint"},
		{"duplicate contract", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request: { shape: S, transform: { passthrough: true } }
      - id: v1
        request: { shape: S, transform: { passthrough: true } }
`), "duplicate contract"},
		{"bad default", minimalSpec(`
  - path: /x
    method: POST
    default_contract: v2
    contracts:
      - id: v1
        request: { shape: S, transform: { passthrough: true } }
`), "default_contract"},
		{"missing shape", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request: { shape: Missing, transform: { passthrough: true } }
`), "not defined"},
		{"response without fields", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        response:
          shape: S
          transform: {}
`), "explicit fields"},
		{"response passthrough", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        response:
          shape: S
          transform: { passthrough: true }
`), "response passthrough"},
		{"invalid target path", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request:
          shape: S
          transform:
            fields:
              - from: ".name"
                to: ".bad-key"
`), "invalid target path"},
		{"zero limit", `
version: "1"
limits:
  request_body_bytes: 0
shapes:
  S:
    type: object
    additionalProperties: false
endpoints: []
`, "positive"},
		{"passthrough not strict", `
version: "1"
shapes:
  S:
    type: object
endpoints:
  - path: /x
    method: POST
    contracts:
      - id: v1
        request: { shape: S, transform: { passthrough: true } }
`, "additionalProperties"},
		{"duplicate example names", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request:
          shape: S
          examples:
            - name: Same
              body: { name: Alice }
            - name: Same
              body: { name: Bob }
          transform: { passthrough: true }
`), "duplicate example"},
		{"empty example name", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request:
          shape: S
          examples:
            - body: { name: Alice }
          transform: { passthrough: true }
`), "example name"},
		{"missing example body", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request:
          shape: S
          examples:
            - name: Empty
          transform: { passthrough: true }
`), "body is required"},
		{"invalid request example", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request:
          shape: S
          examples:
            - name: Bad
              body: { extra: Alice }
          transform: { passthrough: true }
`), "does not match shape"},
		{"invalid response example", minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        response:
          shape: S
          examples:
            - name: Bad
              body: { extra: Alice }
          transform:
            fields:
              - from: ".name"
                to: ".name"
`), "does not match shape"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadSpecBytes([]byte(tc.spec), NewRegistry().Snapshot())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestSpecMetadataAndExamplesAreOptional(t *testing.T) {
	engine := newTestEngine(t, minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request: { shape: S, transform: { passthrough: true } }
`), nil)
	sanitized := engine.SanitizedSpec()
	if sanitized.Title != "" || sanitized.Description != "" {
		t.Fatalf("unexpected metadata = %+v", sanitized)
	}
	if len(sanitized.Endpoints) != 1 || len(sanitized.Endpoints[0].Tags) != 0 {
		t.Fatalf("endpoint metadata = %+v", sanitized.Endpoints)
	}
}

func TestSanitizedSpecIncludesMetadataAndExamples(t *testing.T) {
	spec := `
version: "1"
title: User API Contracts
description: Contract docs.
shapes:
  S:
    type: object
    additionalProperties: false
    required: [name]
    properties:
      name: { type: string }
endpoints:
  - path: /x
    method: post
    summary: Create x
    description: Endpoint docs.
    tags: [users, write]
    default_contract: v1
    contracts:
      - id: v1
        summary: Contract summary
        description: Contract docs.
        deprecated: true
        request:
          description: Request docs.
          shape: S
          examples:
            - name: Basic
              description: Minimal request.
              body: { name: Alice }
          transform: { passthrough: true }
        response:
          description: Response docs.
          shape: S
          examples:
            - name: Created
              body: { name: Alice }
          transform:
            fields:
              - from: ".name"
                to: ".name"
`
	engine := newTestEngine(t, spec, nil)
	sanitized := engine.SanitizedSpec()
	if sanitized.Title != "User API Contracts" || sanitized.Description != "Contract docs." {
		t.Fatalf("top metadata = %+v", sanitized)
	}
	ep := sanitized.Endpoints[0]
	if ep.Route.Method != "POST" || ep.Summary != "Create x" || ep.Description != "Endpoint docs." || len(ep.Tags) != 2 {
		t.Fatalf("endpoint = %+v", ep)
	}
	contract := ep.Contracts[0]
	if contract.Summary != "Contract summary" || contract.Description != "Contract docs." || !contract.Deprecated {
		t.Fatalf("contract = %+v", contract)
	}
	if contract.Request == nil || contract.Request.Description != "Request docs." || len(contract.Request.Examples) != 1 {
		t.Fatalf("request = %+v", contract.Request)
	}
	if contract.Request.Examples[0].Name != "Basic" || contract.Request.Examples[0].Body.(map[string]any)["name"] != "Alice" {
		t.Fatalf("request examples = %+v", contract.Request.Examples)
	}
	if contract.Response == nil || contract.Response.Description != "Response docs." || len(contract.Response.Examples) != 1 {
		t.Fatalf("response = %+v", contract.Response)
	}
}

func TestMissingHandlerReference(t *testing.T) {
	spec := minimalSpec(`
  - path: /x
    method: POST
    contracts:
      - id: v1
        request:
          shape: S
          transform:
            fields:
              - from: ".name"
                to: ".name"
            handler: missing
`)
	_, err := LoadSpecBytes([]byte(spec), NewRegistry().Snapshot())
	if err == nil || !strings.Contains(err.Error(), "handler") {
		t.Fatalf("err = %v", err)
	}
}

func TestEndpointLimitsOverrideGlobal(t *testing.T) {
	spec := `
version: "1"
limits:
  request_body_bytes: 10
  response_body_bytes: 20
shapes:
  S:
    type: object
    additionalProperties: false
    properties:
      name: { type: string }
endpoints:
  - path: /x
    method: POST
    limits:
      request_body_bytes: 30
    contracts:
      - id: v1
        request:
          shape: S
          transform:
            passthrough: true
`
	engine := newTestEngine(t, spec, nil)
	sel, err := engine.ResolveContract(RouteKey{Method: "POST", Path: "/x"}, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if sel.Limits.RequestBodyBytes != 30 || sel.Limits.ResponseBodyBytes != 20 {
		t.Fatalf("limits = %+v", sel.Limits)
	}
}

func minimalSpec(endpoints string) string {
	return `
version: "1"
shapes:
  S:
    type: object
    additionalProperties: false
    properties:
      name: { type: string }
endpoints:
` + endpoints
}
