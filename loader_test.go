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
