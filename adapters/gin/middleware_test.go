package gin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	ginframework "github.com/gin-gonic/gin"
	"github.com/standard-librarian/shapeshifter"
)

const ginSpec = `
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
  - path: /users/:id
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

func TestGinMiddlewareTransforms(t *testing.T) {
	server := newGinServer(t)
	defer server.Close()

	resp := ginPost(t, server.URL+"/users/123", `{"full_name":"Alice","contact":{"email":"alice@example.com"}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Gin-Status"); got != "201" {
		t.Fatalf("gin status header = %q", got)
	}
	assertGinJSON(t, resp, `{"id":"123","full_name":"Alice","contact":{"email":"alice@example.com"}}`)
}

func TestGinMiddlewareErrorsAndBypass(t *testing.T) {
	server := newGinServer(t)
	defer server.Close()

	resp := ginPost(t, server.URL+"/users/123", `{"full_name":"Alice"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertGinBodyContains(t, resp, `"source_schema_failed"`)

	resp = ginPost(t, server.URL+"/users/123?text=1", `{"full_name":"Alice","contact":{"email":"alice@example.com"}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "plain" {
		t.Fatalf("body = %q", body)
	}
}

func newGinServer(t *testing.T) *httptest.Server {
	t.Helper()
	ginframework.SetMode(ginframework.TestMode)
	spec, err := shapeshifter.LoadSpecBytes([]byte(ginSpec), shapeshifter.NewRegistry().Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	r := ginframework.New()
	r.Use(Middleware(engine))
	r.POST("/users/:id", func(c *ginframework.Context) {
		if c.Query("text") == "1" {
			c.String(http.StatusAccepted, "plain")
			return
		}
		var internal map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&internal); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		c.Header("X-Gin-Status", "201")
		c.JSON(http.StatusCreated, map[string]any{"internal_id": c.Param("id"), "name": internal["name"], "email": internal["email"]})
	})
	return httptest.NewServer(r)
}

func ginPost(t *testing.T, url, body, contract string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if contract != "" {
		req.Header.Set("X-ShapeShifter-Contract", contract)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertGinJSON(t *testing.T, resp *http.Response, expected string) {
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

func assertGinBodyContains(t *testing.T, resp *http.Response, sub string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), sub) {
		t.Fatalf("body = %s, want containing %s", body, sub)
	}
}
