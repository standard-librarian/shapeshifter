package ui

import (
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
		{"/app.js", "process/${phase}"},
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
