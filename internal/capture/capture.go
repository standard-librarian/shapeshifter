package capture

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
)

type ResponseCapture struct {
	http.ResponseWriter
	body        *bytes.Buffer
	statusCode  int
	wroteHeader bool
	maxBytes    int64
	tooLarge    bool
}

func NewResponseCapture(w http.ResponseWriter, maxBytes int64) *ResponseCapture {
	return &ResponseCapture{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
		maxBytes:       maxBytes,
	}
}

func (rc *ResponseCapture) WriteHeader(code int) {
	if rc.wroteHeader {
		return
	}
	rc.statusCode = code
	rc.wroteHeader = true
}

func (rc *ResponseCapture) Write(p []byte) (int, error) {
	if !rc.wroteHeader {
		rc.WriteHeader(http.StatusOK)
	}
	if rc.maxBytes > 0 && int64(rc.body.Len()+len(p)) > rc.maxBytes {
		rc.tooLarge = true
		return len(p), nil
	}
	if !rc.tooLarge {
		_, _ = rc.body.Write(p)
	}
	return len(p), nil
}

func (rc *ResponseCapture) StatusCode() int {
	if rc.statusCode == 0 {
		return http.StatusOK
	}
	return rc.statusCode
}

func (rc *ResponseCapture) BodyBytes() []byte {
	return rc.body.Bytes()
}

func (rc *ResponseCapture) BodyLen() int {
	if rc.tooLarge {
		return -1
	}
	return rc.body.Len()
}

func (rc *ResponseCapture) TooLarge() bool {
	return rc.tooLarge
}

func (rc *ResponseCapture) FlushOriginal(method string) {
	payload := rc.body.Bytes()
	if strings.EqualFold(method, http.MethodHead) || rc.StatusCode() == http.StatusNoContent || rc.StatusCode() == http.StatusNotModified {
		payload = nil
	}
	rc.flush(rc.StatusCode(), payload, false)
}

func (rc *ResponseCapture) FlushTransformed(status int, payload []byte) {
	rc.flush(status, payload, true)
}

func (rc *ResponseCapture) flush(status int, payload []byte, transformed bool) {
	h := rc.ResponseWriter.Header()
	if transformed {
		h.Del("Content-Encoding")
	}
	h.Del("Transfer-Encoding")
	h.Set("Content-Length", strconv.Itoa(len(payload)))
	rc.ResponseWriter.WriteHeader(status)
	if len(payload) > 0 {
		_, _ = rc.ResponseWriter.Write(payload)
	}
}
