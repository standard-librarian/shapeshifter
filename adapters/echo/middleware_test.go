package echo

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	labstackecho "github.com/labstack/echo/v4"
	"github.com/standard-librarian/shapeshifter"
)

const echoSpec = `
version: "1"
header: "X-ShapeShifter-Contract"
shapes:
  V1Request:
    type: object
    additionalProperties: false
    required: [name, email]
    properties:
      name: { type: string }
      email: { type: string }
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
  V1Response:
    type: object
    additionalProperties: false
    required: [id, name, email]
    properties:
      id: { type: string }
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
  - path: /users
    method: POST
    default_contract: v1
    contracts:
      - id: v1
        request:
          shape: V1Request
          target_shape: InternalRequest
          transform:
            passthrough: true
        response:
          source_shape: InternalResponse
          shape: V1Response
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
  - path: /small
    method: POST
    limits:
      request_body_bytes: 8
      response_body_bytes: 8
    contracts:
      - id: v1
        request:
          shape: V1Request
          target_shape: InternalRequest
          transform:
            passthrough: true
        response:
          source_shape: InternalResponse
          shape: V1Response
          transform:
            fields:
              - from: ".internal_id"
                to: ".id"
              - from: ".name"
                to: ".name"
              - from: ".email"
                to: ".email"
  - path: /nodefault
    method: POST
    contracts:
      - id: v1
        request:
          shape: V1Request
          transform:
            passthrough: true
  - path: /small-response
    method: POST
    default_contract: v1
    limits:
      response_body_bytes: 8
    contracts:
      - id: v1
        response:
          source_shape: InternalResponse
          shape: V1Response
          transform:
            fields:
              - from: ".internal_id"
                to: ".id"
              - from: ".name"
                to: ".name"
              - from: ".email"
                to: ".email"
`

func TestEchoMiddlewareRealServer(t *testing.T) {
	server := newEchoTestServer(t)
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}

	t.Run("default contract", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users", "", `{"name":"Alice","email":"alice@example.com"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertResponseJSON(t, resp, `{"id":"123","name":"Alice","email":"alice@example.com"}`)
	})

	t.Run("v2 contract", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users", "v2", `{"full_name":"Alice","contact":{"email":"alice@example.com"}}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertResponseJSON(t, resp, `{"id":"123","full_name":"Alice","contact":{"email":"alice@example.com"}}`)
	})

	t.Run("malformed json", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users", "", `{"name":`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertBodyContains(t, resp, `"malformed_json"`)
	})

	t.Run("unknown contract", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users", "v9", `{"name":"Alice","email":"alice@example.com"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertBodyContains(t, resp, `"unknown_contract"`)
	})

	t.Run("missing contract without default", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/nodefault", "", `{"name":"Alice","email":"alice@example.com"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertBodyContains(t, resp, `"missing_contract_header"`)
	})

	t.Run("request too large", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/small", "v1", `{"name":"Alice","email":"alice@example.com"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertBodyContains(t, resp, `"request_too_large"`)
	})

	t.Run("unsupported request content type", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/users", strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertBodyContains(t, resp, `"unsupported_content_type"`)
	})

	t.Run("valid json schema failure", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users", "", `{"name":"Alice"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertBodyContains(t, resp, `"source_schema_failed"`)
	})

	t.Run("response too large", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/small-response", "", `{}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		assertBodyContains(t, resp, `"response_too_large"`)
	})

	t.Run("non json bypass", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users?text=1", "", `{"name":"Alice","email":"alice@example.com"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "plain" {
			t.Fatalf("body = %q", body)
		}
	})

	t.Run("gzip bypass", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users?gzip=1", "", `{"name":"Alice","email":"alice@example.com"}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("content-encoding = %q", got)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != `{"internal_id":"123","name":"Alice","email":"alice@example.com"}` {
			t.Fatalf("body = %q", body)
		}
	})

	t.Run("response transform failure cleans headers", func(t *testing.T) {
		resp := doPost(t, client, server.URL+"/users?bad_response=1", "v2", `{"full_name":"Alice","contact":{"email":"alice@example.com"}}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Encoding"); got != "" {
			t.Fatalf("content-encoding = %q", got)
		}
		if got := resp.Header.Get("X-App-Secret"); got != "" {
			t.Fatalf("secret header leaked: %q", got)
		}
		if got := resp.Header.Get("X-Request-ID"); got != "req-1" {
			t.Fatalf("x-request-id = %q", got)
		}
		assertBodyContains(t, resp, `"contract processing failed"`)
	})
}

func TestReadLimited(t *testing.T) {
	got, err := readLimited(strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
	if _, err := readLimited(strings.NewReader("hello!"), 5); !errors.Is(err, shapeshifter.ErrRequestTooLarge) {
		t.Fatalf("err = %v", err)
	}
}

func newEchoTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	registry := shapeshifter.NewRegistry()
	spec, err := shapeshifter.LoadSpecBytes([]byte(echoSpec), registry.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}

	e := labstackecho.New()
	e.Use(Middleware(engine))
	e.POST("/users", func(c labstackecho.Context) error {
		if c.QueryParam("text") == "1" {
			return c.String(http.StatusOK, "plain")
		}
		var internal map[string]any
		if err := json.NewDecoder(c.Request().Body).Decode(&internal); err != nil {
			return err
		}
		if c.QueryParam("gzip") == "1" {
			c.Response().Header().Set("Content-Type", "application/json")
			c.Response().Header().Set("Content-Encoding", "gzip")
			return c.String(http.StatusOK, `{"internal_id":"123","name":"Alice","email":"alice@example.com"}`)
		}
		if c.QueryParam("bad_response") == "1" {
			c.Response().Header().Set("Content-Encoding", "identity")
			c.Response().Header().Set("X-App-Secret", "secret")
			c.Response().Header().Set("X-Request-ID", "req-1")
			return c.JSON(http.StatusOK, map[string]any{"name": internal["name"], "email": internal["email"]})
		}
		return c.JSON(http.StatusOK, map[string]any{"internal_id": "123", "name": internal["name"], "email": internal["email"]})
	})
	e.POST("/small", func(c labstackecho.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"internal_id": "123", "name": "Alice", "email": "alice@example.com"})
	})
	e.POST("/nodefault", func(c labstackecho.Context) error {
		return c.NoContent(http.StatusOK)
	})
	e.POST("/small-response", func(c labstackecho.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"internal_id": "123", "name": "Alice", "email": "alice@example.com"})
	})
	return httptest.NewServer(e)
}

func doPost(t *testing.T, client *http.Client, url, contract, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if contract != "" {
		req.Header.Set("X-ShapeShifter-Contract", contract)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertResponseJSON(t *testing.T, resp *http.Response, expected string) {
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

func assertBodyContains(t *testing.T, resp *http.Response, sub string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), sub) {
		t.Fatalf("body = %s, want containing %s", body, sub)
	}
}
