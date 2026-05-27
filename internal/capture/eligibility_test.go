package capture

import (
	"net/http"
	"testing"

	"github.com/standard-librarian/shapeshifter"
)

func TestIsTransformableJSON(t *testing.T) {
	tests := []struct {
		name   string
		method string
		status int
		ct     string
		enc    string
		has    bool
		ok     bool
		reason shapeshifter.BypassReason
	}{
		{"json", "GET", 200, "application/json; charset=utf-8", "", true, true, shapeshifter.BypassNone},
		{"problem", "GET", 200, "application/problem+json", "", true, true, shapeshifter.BypassNone},
		{"vendor", "GET", 200, "application/vnd.api+json", "", true, true, shapeshifter.BypassNone},
		{"text", "GET", 200, "text/plain", "", true, false, shapeshifter.BypassContentType},
		{"head", "HEAD", 200, "application/json", "", true, false, shapeshifter.BypassHead},
		{"204", "GET", 204, "application/json", "", true, false, shapeshifter.BypassStatus},
		{"304", "GET", 304, "application/json", "", true, false, shapeshifter.BypassStatus},
		{"400", "GET", 400, "application/json", "", true, false, shapeshifter.BypassStatus},
		{"gzip", "GET", 200, "application/json", "gzip", true, false, shapeshifter.BypassContentEncoding},
		{"no side", "GET", 200, "application/json", "", false, false, shapeshifter.BypassNoResponseSide},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.ct != "" {
				h.Set("Content-Type", tc.ct)
			}
			if tc.enc != "" {
				h.Set("Content-Encoding", tc.enc)
			}
			ok, reason := IsTransformableJSON(tc.method, tc.status, h, 2, tc.has)
			if ok != tc.ok || reason != tc.reason {
				t.Fatalf("ok=%v reason=%q", ok, reason)
			}
		})
	}
}
