package echo

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	labstackecho "github.com/labstack/echo/v4"
	"github.com/standard-librarian/shapeshifter"
)

const previewSpec = `
version: "1"
header: "X-ShapeShifter-Contract"
shapes:
  ExternalRequest:
    type: object
    additionalProperties: false
    required: [full_name]
    properties:
      full_name: { type: string }
  InternalRequest:
    type: object
    additionalProperties: false
    required: [name]
    properties:
      name: { type: string }
  InternalResponse:
    type: object
    additionalProperties: false
    required: [internal_id, name]
    properties:
      internal_id: { type: string }
      name: { type: string }
  ExternalResponse:
    type: object
    additionalProperties: false
    required: [id, full_name]
    properties:
      id: { type: string }
      full_name: { type: string }
endpoints:
  - path: /preview
    method: POST
    default_contract: v1
    contracts:
      - id: v1
        request:
          shape: ExternalRequest
          target_shape: InternalRequest
          transform:
            fields:
              - from: ".full_name"
                to: ".name"
            handler: unsafeRequest
        response:
          source_shape: InternalResponse
          shape: ExternalResponse
          transform:
            fields:
              - from: ".internal_id"
                to: ".id"
              - from: ".name"
                to: ".full_name"
            handler: safeResponse
`

func TestPreviewAPI(t *testing.T) {
	server := newPreviewServer(t)
	defer server.Close()
	client := server.Client()

	t.Run("sanitized spec", func(t *testing.T) {
		resp, err := client.Get(server.URL + DefaultPreviewPrefix + "/spec")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if bytes.Contains(body, []byte("additionalProperties")) {
			t.Fatalf("sanitized spec leaked schema internals: %s", body)
		}
		if !bytes.Contains(body, []byte(`"name":"unsafeRequest"`)) || !bytes.Contains(body, []byte(`"preview_safe":false`)) {
			t.Fatalf("sanitized spec missing handler metadata: %s", body)
		}
	})

	t.Run("request preview skips unsafe handler", func(t *testing.T) {
		resp := postPreview(t, client, server.URL+DefaultPreviewPrefix+"/process/request", `{
			"route": {"method":"POST","path":"/preview"},
			"contract": "v1",
			"body": {"full_name":"Alice"}
		}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		var got struct {
			Payload         map[string]any `json:"payload"`
			SkippedHandlers []string       `json:"skipped_handlers"`
			Phase           string         `json:"phase"`
		}
		decodePreview(t, resp, &got)
		if got.Phase != "request" {
			t.Fatalf("phase = %q", got.Phase)
		}
		if !reflect.DeepEqual(got.Payload, map[string]any{"name": "Alice"}) {
			t.Fatalf("payload = %#v", got.Payload)
		}
		if !reflect.DeepEqual(got.SkippedHandlers, []string{"unsafeRequest"}) {
			t.Fatalf("skipped = %#v", got.SkippedHandlers)
		}
	})

	t.Run("response preview calls safe handler", func(t *testing.T) {
		resp := postPreview(t, client, server.URL+DefaultPreviewPrefix+"/process/response", `{
			"route": {"method":"POST","path":"/preview"},
			"contract": "v1",
			"body": {"internal_id":"123","name":"Alice"}
		}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		var got struct {
			Payload map[string]any `json:"payload"`
			Phase   string         `json:"phase"`
		}
		decodePreview(t, resp, &got)
		if got.Phase != "response" {
			t.Fatalf("phase = %q", got.Phase)
		}
		if !reflect.DeepEqual(got.Payload, map[string]any{"id": "123", "full_name": "Alice!"}) {
			t.Fatalf("payload = %#v", got.Payload)
		}
	})

	t.Run("unknown contract uses shared envelope", func(t *testing.T) {
		resp := postPreview(t, client, server.URL+DefaultPreviewPrefix+"/process/request", `{
			"route": {"method":"POST","path":"/preview"},
			"contract": "v9",
			"body": {"full_name":"Alice"}
		}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"unknown_contract"`) {
			t.Fatalf("body = %s", body)
		}
	})

	t.Run("processing error uses shared envelope", func(t *testing.T) {
		resp := postPreview(t, client, server.URL+DefaultPreviewPrefix+"/process/request", `{
			"route": {"method":"POST","path":"/preview"},
			"body": {"wrong":"Alice"}
		}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"source_schema_failed"`) {
			t.Fatalf("body = %s", body)
		}
	})
}

func newPreviewServer(t *testing.T) *httptest.Server {
	t.Helper()
	registry := shapeshifter.NewRegistry()
	if err := registry.Register("unsafeRequest", func(input map[string]any) (map[string]any, error) {
		input["name"] = "runtime"
		return input, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("safeResponse", func(input map[string]any) (map[string]any, error) {
		input["full_name"] = input["full_name"].(string) + "!"
		return input, nil
	}, shapeshifter.HandlerOptions{PreviewSafe: true}); err != nil {
		t.Fatal(err)
	}
	spec, err := shapeshifter.LoadSpecBytes([]byte(previewSpec), registry.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	e := labstackecho.New()
	MountPreviewAPI(e, engine)
	return httptest.NewServer(e)
}

func postPreview(t *testing.T, client *http.Client, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodePreview(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}
