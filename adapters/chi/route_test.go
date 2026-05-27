package chi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	chiframework "github.com/go-chi/chi/v5"
	"github.com/standard-librarian/shapeshifter"
)

const chiSpec = `
version: "1"
header: "X-ShapeShifter-Contract"
shapes:
  V2Request:
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
  InternalRequest:
    type: object
    additionalProperties: false
    required: [name, email]
    properties:
      name: { type: string }
      email: { type: string }
  InternalResponse:
    type: object
    additionalProperties: false
    required: [internal_id, name, email]
    properties:
      internal_id: { type: string }
      name: { type: string }
      email: { type: string }
  V2Response:
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
  - path: /users/{id}
    method: POST
    default_contract: v2
    contracts:
      - id: v2
        request:
          shape: V2Request
          target_shape: InternalRequest
          transform:
            fields:
              - from: ".full_name"
                to: ".name"
              - from: ".contact.email"
                to: ".email"
        response:
          source_shape: InternalResponse
          shape: V2Response
          transform:
            fields:
              - from: ".internal_id"
                to: ".id"
              - from: ".name"
                to: ".full_name"
              - from: ".email"
                to: ".contact.email"
`

func TestChiRouteTransforms(t *testing.T) {
	server := newChiServer(t)
	defer server.Close()

	resp := chiPost(t, server.URL+"/users/123", `{"full_name":"Alice","contact":{"email":"alice@example.com"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertChiJSON(t, resp, `{"id":"123","full_name":"Alice","contact":{"email":"alice@example.com"}}`)
}

func TestChiRouteErrorsAndBypass(t *testing.T) {
	server := newChiServer(t)
	defer server.Close()

	resp := chiPost(t, server.URL+"/users/123", `{"full_name":"Alice"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertChiBodyContains(t, resp, `"source_schema_failed"`)

	resp = chiPost(t, server.URL+"/plain", `{"anything":true}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "plain" {
		t.Fatalf("body = %q", body)
	}
}

func newChiServer(t *testing.T) *httptest.Server {
	t.Helper()
	spec, err := shapeshifter.LoadSpecBytes([]byte(chiSpec), shapeshifter.NewRegistry().Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	r := chiframework.NewRouter()
	r.With(Route(engine, "/users/{id}")).Post("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		var internal map[string]any
		if err := json.NewDecoder(r.Body).Decode(&internal); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"internal_id": chiframework.URLParam(r, "id"),
			"name":        internal["name"],
			"email":       internal["email"],
		})
	})
	r.Post("/plain", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain"))
	})
	return httptest.NewServer(r)
}

func chiPost(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertChiJSON(t *testing.T, resp *http.Response, expected string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var got any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("body is not JSON %q: %v", body, err)
	}
	var want any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("json = %s, want %s", body, expected)
	}
}

func assertChiBodyContains(t *testing.T, resp *http.Response, sub string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), sub) {
		t.Fatalf("body = %s, want containing %s", body, sub)
	}
}
