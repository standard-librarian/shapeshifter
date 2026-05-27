package capture

import (
	"mime"
	"net/http"
	"strings"

	"github.com/standard-librarian/shapeshifter"
)

func IsJSONMediaType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	if mediaType == "application/json" {
		return true
	}
	parts := strings.SplitN(mediaType, "/", 2)
	if len(parts) != 2 {
		return false
	}
	return strings.HasSuffix(parts[1], "+json")
}

func IsJSONRequest(header http.Header) bool {
	return IsJSONMediaType(header.Get("Content-Type"))
}

func IsTransformableJSON(method string, status int, header http.Header, bodyLen int, hasResponseSide bool) (bool, shapeshifter.BypassReason) {
	if !hasResponseSide {
		return false, shapeshifter.BypassNoResponseSide
	}
	if strings.EqualFold(method, http.MethodHead) {
		return false, shapeshifter.BypassHead
	}
	if status == 0 {
		status = http.StatusOK
	}
	if status < 200 || status >= 400 || status == http.StatusNoContent || status == http.StatusNotModified {
		return false, shapeshifter.BypassStatus
	}
	if bodyLen < 0 {
		return false, shapeshifter.BypassTooLarge
	}
	if !IsJSONMediaType(header.Get("Content-Type")) {
		return false, shapeshifter.BypassContentType
	}
	encoding := strings.TrimSpace(header.Get("Content-Encoding"))
	if encoding != "" && !strings.EqualFold(encoding, "identity") {
		return false, shapeshifter.BypassContentEncoding
	}
	return true, shapeshifter.BypassNone
}
