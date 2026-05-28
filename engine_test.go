package shapeshifter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const sampleSpec = `
version: "1"
header: "X-ShapeShifter-Contract"
limits:
  request_body_bytes: 65536
  response_body_bytes: 1048576
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
          phone: { type: string }
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
    default_contract: v1
    contracts:
      - id: v1
        request:
          shape: CreateUserV1Request
          target_shape: CreateUserInternalRequest
          transform:
            passthrough: true
            validate:
              - field: ".email"
                rule: "test(\"^[^@]+@[^@]+$\")"
                error: "invalid email format"
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
        request:
          shape: CreateUserV2Request
          target_shape: CreateUserInternalRequest
          transform:
            fields:
              - from: ".full_name"
                to: ".name"
              - from: ".contact.email"
                to: ".email"
              - from: ".contact.phone"
                to: ".phone"
                required: false
            validate:
              - field: ".contact.email"
                rule: "test(\"^[^@]+@[^@]+$\")"
                error: "invalid email format"
        response:
          source_shape: UserInternalResponse
          shape: UserV2Response
          transform:
            fields:
              - from: ".internal_id"
                to: ".id"
              - from: ".name"
                to: ".full_name"
              - from: ".email"
                to: ".contact.email"
`

func newTestEngine(t *testing.T, specText string, registry *Registry) *Engine {
	t.Helper()
	if registry == nil {
		registry = NewRegistry()
	}
	spec, err := LoadSpecBytes([]byte(specText), registry.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestProcessRequestAndResponse(t *testing.T) {
	engine := newTestEngine(t, sampleSpec, nil)
	route := RouteKey{Method: "POST", Path: "/users"}

	v1, err := engine.ResolveContract(route, "")
	if err != nil {
		t.Fatal(err)
	}
	if v1.ContractID != "v1" || !v1.HasRequest || !v1.HasResponse {
		t.Fatalf("unexpected v1 selection: %+v", v1)
	}
	got, err := engine.ProcessRequest(context.Background(), v1, []byte(`{"name":"Alice","email":"alice@example.com"}`), ModeRuntime)
	if err != nil {
		t.Fatal(err)
	}
	assertJSON(t, got.Payload, `{"email":"alice@example.com","name":"Alice"}`)

	v2, err := engine.ResolveContract(route, "v2")
	if err != nil {
		t.Fatal(err)
	}
	got, err = engine.ProcessRequest(context.Background(), v2, []byte(`{"full_name":"Alice","contact":{"email":"alice@example.com"}}`), ModeRuntime)
	if err != nil {
		t.Fatal(err)
	}
	assertJSON(t, got.Payload, `{"email":"alice@example.com","name":"Alice"}`)

	got, err = engine.ProcessResponse(context.Background(), v2, []byte(`{"internal_id":"123","name":"Alice","email":"alice@example.com"}`), ModeRuntime)
	if err != nil {
		t.Fatal(err)
	}
	assertJSON(t, got.Payload, `{"contact":{"email":"alice@example.com"},"full_name":"Alice","id":"123"}`)
}

func TestProcessFailures(t *testing.T) {
	engine := newTestEngine(t, sampleSpec, nil)
	route := RouteKey{Method: "POST", Path: "/users"}
	v2, err := engine.ResolveContract(route, "v2")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input string
		code  ErrorCode
	}{
		{"malformed", `{"full_name":`, CodeMalformedJSON},
		{"source schema", `{"full_name":"Alice","contact":{}}`, CodeSourceSchemaFailed},
		{"validation rule", `{"full_name":"Alice","contact":{"email":"bad"}}`, CodeValidationRuleFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := engine.ProcessRequest(context.Background(), v2, []byte(tc.input), ModeRuntime)
			assertShapeErrorCode(t, err, tc.code)
		})
	}
}

func TestMultipleOutputsAndCoerceAndHandlers(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("suffix", func(input map[string]any) (map[string]any, error) {
		input["name"] = input["name"].(string) + "!"
		return input, nil
	}, HandlerOptions{PreviewSafe: true}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("unsafe", func(input map[string]any) (map[string]any, error) {
		input["name"] = "runtime"
		return input, nil
	}); err != nil {
		t.Fatal(err)
	}

	specText := `
version: "1"
shapes:
  Source:
    type: object
    additionalProperties: false
    required: [name, age]
    properties:
      name: { type: string }
      age: { type: string }
  Target:
    type: object
    additionalProperties: false
    required: [name, age]
    properties:
      name: { type: string }
      age: { type: integer }
endpoints:
  - path: /coerce
    method: POST
    default_contract: ok
    contracts:
      - id: ok
        request:
          shape: Source
          target_shape: Target
          transform:
            fields:
              - from: ".name"
                to: ".name"
              - from: ".age"
                to: ".age"
            coerce:
              - field: ".age"
                type: integer
            handler: suffix
      - id: unsafe
        request:
          shape: Source
          target_shape: Target
          transform:
            fields:
              - from: ".name"
                to: ".name"
              - from: ".age"
                to: ".age"
            coerce:
              - field: ".age"
                type: integer
            handler: unsafe
      - id: multi
        request:
          shape: Source
          target_shape: Target
          transform:
            fields:
              - from: ".[]"
                to: ".name"
              - from: ".age"
                to: ".age"
            coerce:
              - field: ".age"
                type: integer
`
	engine := newTestEngine(t, specText, registry)
	route := RouteKey{Method: "POST", Path: "/coerce"}

	ok, _ := engine.ResolveContract(route, "ok")
	got, err := engine.ProcessRequest(context.Background(), ok, []byte(`{"name":"Alice","age":"42"}`), ModeRuntime)
	if err != nil {
		t.Fatal(err)
	}
	assertJSON(t, got.Payload, `{"age":42,"name":"Alice!"}`)

	_, err = engine.ProcessRequest(context.Background(), ok, []byte(`{"name":"Alice","age":"nope"}`), ModeRuntime)
	assertShapeErrorCode(t, err, CodeCoercionFailed)

	multi, _ := engine.ResolveContract(route, "multi")
	_, err = engine.ProcessRequest(context.Background(), multi, []byte(`{"name":"Alice","age":"42"}`), ModeRuntime)
	assertShapeErrorCode(t, err, CodeMultipleJQOutputs)

	unsafe, _ := engine.ResolveContract(route, "unsafe")
	got, err = engine.ProcessRequest(context.Background(), unsafe, []byte(`{"name":"Alice","age":"42"}`), ModePreview)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SkippedHandlers) != 1 || got.SkippedHandlers[0] != "unsafe" {
		t.Fatalf("skipped handlers = %#v", got.SkippedHandlers)
	}
	assertJSON(t, got.Payload, `{"age":42,"name":"Alice"}`)
}

func TestHandlerValidationAndSystemFailure(t *testing.T) {
	registry := NewRegistry()
	_ = registry.Register("validate", func(input map[string]any) (map[string]any, error) {
		return nil, &HandlerValidationError{Errors: []ValidationError{{Field: ".name", Message: "bad", Code: CodeHandlerValidationFailed}}}
	})
	_ = registry.Register("fail", func(input map[string]any) (map[string]any, error) {
		return nil, errors.New("boom")
	})
	specText := handlerSpec("validate", "fail")
	engine := newTestEngine(t, specText, registry)
	route := RouteKey{Method: "POST", Path: "/h"}

	validate, _ := engine.ResolveContract(route, "validate")
	_, err := engine.ProcessRequest(context.Background(), validate, []byte(`{"name":"Alice"}`), ModeRuntime)
	assertShapeErrorCode(t, err, CodeHandlerValidationFailed)

	fail, _ := engine.ResolveContract(route, "fail")
	_, err = engine.ProcessRequest(context.Background(), fail, []byte(`{"name":"Alice"}`), ModeRuntime)
	assertShapeErrorCode(t, err, CodeHandlerFailed)
}

func TestLoadSpecConveniencesAndSelection(t *testing.T) {
	spec, err := LoadSpec(bytesReader(sampleSpec), NewRegistry().Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got := engine.Contracts(RouteKey{Method: "post", Path: "/users"}); len(got) != 2 || got[0] != "v1" || got[1] != "v2" {
		t.Fatalf("contracts = %#v", got)
	}
	_, err = engine.ResolveContract(RouteKey{Method: "POST", Path: "/users"}, "v9")
	var selectionErr *ContractSelectionError
	if !errors.As(err, &selectionErr) || selectionErr.Reason != ContractReasonUnknown {
		t.Fatalf("selection err = %#v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "shapeshifter.yaml")
	if err := os.WriteFile(path, []byte(sampleSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSpecFile(path, NewRegistry().Snapshot()); err != nil {
		t.Fatal(err)
	}
}

func TestSanitizedSpecMetadata(t *testing.T) {
	engine := newTestEngine(t, sampleSpec, nil)
	sanitized := engine.SanitizedSpec()
	if sanitized.Version != "1" || sanitized.Header != "X-ShapeShifter-Contract" {
		t.Fatalf("sanitized header/version = %+v", sanitized)
	}
	if len(sanitized.Shapes) == 0 {
		t.Fatal("expected shape names")
	}
	if sanitized.ShapeSchemas["CreateUserV2Request"] == nil {
		t.Fatal("expected sanitized shape schemas")
	}
	var v2 SanitizedContract
	for _, endpoint := range sanitized.Endpoints {
		if endpoint.Route == (RouteKey{Method: "POST", Path: "/users"}) {
			for _, contract := range endpoint.Contracts {
				if contract.ID == "v2" {
					v2 = contract
				}
			}
		}
	}
	if v2.ID == "" || v2.Request == nil || v2.Response == nil {
		t.Fatalf("missing v2 metadata: %+v", sanitized.Endpoints)
	}
	if v2.Request.Shape != "CreateUserV2Request" || v2.Request.SourceShape != "" || v2.Request.TargetShape != "CreateUserInternalRequest" {
		t.Fatalf("request side = %+v", v2.Request)
	}
	if v2.Response.Shape != "UserV2Response" || v2.Response.SourceShape != "UserInternalResponse" || v2.Response.TargetShape != "" {
		t.Fatalf("response side = %+v", v2.Response)
	}
}

func TestObserverFuncReceivesEvents(t *testing.T) {
	spec, err := LoadSpecBytes([]byte(sampleSpec), NewRegistry().Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var events []Event
	engine, err := NewEngine(spec, WithObserver(ObserverFunc(func(event Event) {
		events = append(events, event)
	})))
	if err != nil {
		t.Fatal(err)
	}
	sel, err := engine.ResolveContract(RouteKey{Method: "POST", Path: "/users"}, "v2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ProcessRequest(context.Background(), sel, []byte(`{"full_name":"Alice","contact":{"email":"alice@example.com"}}`), ModeRuntime); err != nil {
		t.Fatal(err)
	}
	var selected, transformed bool
	for _, event := range events {
		if event.Kind == EventContractSelected {
			selected = true
		}
		if event.Kind == EventRequestTransformed {
			transformed = true
		}
	}
	if !selected || !transformed {
		t.Fatalf("events = %#v", events)
	}
}

func TestEngineConcurrentUse(t *testing.T) {
	engine := newTestEngine(t, sampleSpec, nil)
	sel, err := engine.ResolveContract(RouteKey{Method: "POST", Path: "/users"}, "v2")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				req, err := engine.ProcessRequest(context.Background(), sel, []byte(`{"full_name":"Alice","contact":{"email":"alice@example.com"}}`), ModeRuntime)
				if err != nil {
					t.Errorf("request: %v", err)
					return
				}
				if len(req.Payload) == 0 {
					t.Errorf("empty request payload")
					return
				}
				resp, err := engine.ProcessResponse(context.Background(), sel, []byte(`{"internal_id":"123","name":"Alice","email":"alice@example.com"}`), ModeRuntime)
				if err != nil {
					t.Errorf("response: %v", err)
					return
				}
				if len(resp.Payload) == 0 {
					t.Errorf("empty response payload")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func assertShapeErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s", code)
	}
	var ssErr *ShapeShifterError
	if !errors.As(err, &ssErr) {
		t.Fatalf("expected ShapeShifterError, got %T %v", err, err)
	}
	for _, detail := range ssErr.Errors {
		if detail.Code == code {
			return
		}
	}
	t.Fatalf("details = %#v, want code %s", ssErr.Errors, code)
}

func handlerSpec(first, second string) string {
	return `
version: "1"
shapes:
  S:
    type: object
    additionalProperties: false
    required: [name]
    properties:
      name: { type: string }
endpoints:
  - path: /h
    method: POST
    contracts:
      - id: validate
        request:
          shape: S
          target_shape: S
          transform:
            fields:
              - from: ".name"
                to: ".name"
            handler: ` + first + `
      - id: fail
        request:
          shape: S
          target_shape: S
          transform:
            fields:
              - from: ".name"
                to: ".name"
            handler: ` + second + `
`
}
