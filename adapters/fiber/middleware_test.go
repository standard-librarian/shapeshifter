package fiber

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	fiberframework "github.com/gofiber/fiber/v2"
	"github.com/standard-librarian/shapeshifter"
)

const fiberSpec = `
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

func TestFiberMiddlewareTransforms(t *testing.T) {
	app := newFiberApp(t)

	resp := fiberPost(t, app, "/users/123", `{"full_name":"Alice","contact":{"email":"alice@example.com"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFiberJSON(t, resp, `{"id":"123","full_name":"Alice","contact":{"email":"alice@example.com"}}`)
}

func TestFiberMiddlewareErrorsAndBypass(t *testing.T) {
	app := newFiberApp(t)

	resp := fiberPost(t, app, "/users/123", `{"full_name":"Alice"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFiberBodyContains(t, resp, `"source_schema_failed"`)

	resp = fiberPost(t, app, "/users/123?text=1", `{"full_name":"Alice","contact":{"email":"alice@example.com"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "plain" {
		t.Fatalf("body = %q", body)
	}
}

func newFiberApp(t *testing.T) *fiberframework.App {
	t.Helper()
	spec, err := shapeshifter.LoadSpecBytes([]byte(fiberSpec), shapeshifter.NewRegistry().Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	app := fiberframework.New(fiberframework.Config{DisableStartupMessage: true})
	app.Post("/users/:id", Middleware(engine), func(c *fiberframework.Ctx) error {
		if c.Query("text") == "1" {
			return c.Status(http.StatusAccepted).Type("txt").SendString("plain")
		}
		var internal map[string]any
		if err := json.Unmarshal(c.BodyRaw(), &internal); err != nil {
			return err
		}
		return c.JSON(map[string]any{
			"internal_id": c.Params("id"),
			"name":        internal["name"],
			"email":       internal["email"],
		})
	})
	return app
}

func fiberPost(t *testing.T, app *fiberframework.App, target, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, target, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertFiberJSON(t *testing.T, resp *http.Response, expected string) {
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

func assertFiberBodyContains(t *testing.T, resp *http.Response, sub string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), sub) {
		t.Fatalf("body = %s, want containing %s", body, sub)
	}
}
