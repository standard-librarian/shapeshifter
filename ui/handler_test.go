package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesEmbeddedUI(t *testing.T) {
	handler := Handler()

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/", "ShapeShifter"},
		{"/app.js", "runPreview"},
		{"/styles.css", "grid-template-columns"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("body missing %q", tc.want)
			}
		})
	}
}

func TestHandlerConfig(t *testing.T) {
	tests := []struct {
		name string
		h    http.Handler
		want config
	}{
		{
			name: "defaults",
			h:    Handler(),
			want: config{PreviewAPIBase: "/_shapeshifter/api", TryItOutBase: "/"},
		},
		{
			name: "custom",
			h: Handler(
				WithPreviewAPIBase("/tools/shapeshifter/api/"),
				WithTryItOut(true),
				WithTryItOutBase("/api"),
			),
			want: config{PreviewAPIBase: "/tools/shapeshifter/api", TryItOutEnabled: true, TryItOutBase: "/api/"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/config.json", nil)
			rec := httptest.NewRecorder()
			tc.h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			var got config
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("config = %+v, want %+v", got, tc.want)
			}
		})
	}
}
