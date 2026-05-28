package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/standard-librarian/shapeshifter"
	shapeshifterecho "github.com/standard-librarian/shapeshifter/adapters/echo"
)

func TestExamplePortalAndRuntime(t *testing.T) {
	registry := shapeshifter.NewRegistry()
	spec, err := shapeshifter.LoadSpecFile("shapeshifter.yaml", registry.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(shapeshifterecho.Middleware(engine))
	shapeshifterecho.MountPreviewAPI(e, engine)
	mountShapeShifterUI(e)
	e.POST("/users", createUser)

	server := httptest.NewServer(e)
	defer server.Close()
	client := server.Client()

	assertGETContains(t, client, server.URL+"/_shapeshifter/ui/", "ShapeShifter")
	assertGETContains(t, client, server.URL+"/_shapeshifter/ui/config.json", `"try_it_out_enabled":true`)
	assertGETContains(t, client, server.URL+"/_shapeshifter/api/spec", `"title":"User API Contracts"`)
	assertGETContains(t, client, server.URL+"/_shapeshifter/api/spec", `"examples"`)

	resp := postJSON(t, client, server.URL+"/_shapeshifter/api/process/request", `{
		"route":{"method":"POST","path":"/users"},
		"contract":"v2",
		"body":{"full_name":"Alice","contact":{"email":"alice@example.com"}}
	}`)
	assertBodyContains(t, resp, `"name":"Alice"`)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/users", strings.NewReader(`{"full_name":"Alice","contact":{"email":"alice@example.com"}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ShapeShifter-Contract", "v2")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	assertBodyContains(t, resp, `"full_name":"Alice"`)
}

func assertGETContains(t *testing.T, client *http.Client, url, want string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	assertBodyContains(t, resp, want)
}

func postJSON(t *testing.T, client *http.Client, url, body string) *http.Response {
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

func assertBodyContains(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), want) {
		t.Fatalf("body missing %q: %s", want, body)
	}
}
