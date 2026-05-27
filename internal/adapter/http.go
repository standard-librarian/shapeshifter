package adapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/standard-librarian/shapeshifter"
)

func ReadLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, shapeshifter.ErrRequestTooLarge
	}
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, shapeshifter.ErrRequestTooLarge
	}
	return data, nil
}

func UnsupportedContentTypeError(sel shapeshifter.ContractSelection) *shapeshifter.ShapeShifterError {
	return &shapeshifter.ShapeShifterError{
		Route:      sel.Route,
		ContractID: sel.ContractID,
		Phase:      shapeshifter.PhaseRequest,
		Stage:      shapeshifter.StageDecode,
		Errors: []shapeshifter.ValidationError{{
			Field:   ".",
			Message: "unsupported content type",
			Code:    shapeshifter.CodeUnsupportedContentType,
		}},
		Cause: shapeshifter.ErrUnsupportedContentType,
	}
}

func RequestTooLargeError(sel shapeshifter.ContractSelection) *shapeshifter.ShapeShifterError {
	return &shapeshifter.ShapeShifterError{
		Route:      sel.Route,
		ContractID: sel.ContractID,
		Phase:      shapeshifter.PhaseRequest,
		Stage:      shapeshifter.StageDecode,
		Errors: []shapeshifter.ValidationError{{
			Field:   ".",
			Message: "request body too large",
			Code:    shapeshifter.CodeRequestTooLarge,
		}},
		Cause: shapeshifter.ErrRequestTooLarge,
	}
}

func ResponseTooLargeError(sel shapeshifter.ContractSelection) *shapeshifter.ShapeShifterError {
	return &shapeshifter.ShapeShifterError{
		Route:      sel.Route,
		ContractID: sel.ContractID,
		Phase:      shapeshifter.PhaseResponse,
		Stage:      shapeshifter.StageTransform,
		Errors: []shapeshifter.ValidationError{{
			Field:   ".",
			Message: "response body too large",
			Code:    shapeshifter.CodeResponseTooLarge,
		}},
		Cause: errors.New("shapeshifter: response body too large"),
	}
}

func StatusForError(err error) int {
	if errors.Is(err, shapeshifter.ErrRequestTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	if errors.Is(err, shapeshifter.ErrUnsupportedContentType) {
		return http.StatusBadRequest
	}
	var ssErr *shapeshifter.ShapeShifterError
	if errors.As(err, &ssErr) {
		if ssErr.Phase == shapeshifter.PhaseResponse {
			return http.StatusInternalServerError
		}
		for _, detail := range ssErr.Errors {
			switch detail.Code {
			case shapeshifter.CodeMalformedJSON, shapeshifter.CodeEmptyRequestBody, shapeshifter.CodeUnsupportedContentType:
				return http.StatusBadRequest
			case shapeshifter.CodeRequestTooLarge:
				return http.StatusRequestEntityTooLarge
			}
		}
		if ssErr.Stage == shapeshifter.StageHandler {
			for _, detail := range ssErr.Errors {
				if detail.Code == shapeshifter.CodeHandlerValidationFailed {
					return http.StatusUnprocessableEntity
				}
			}
			return http.StatusInternalServerError
		}
		return http.StatusUnprocessableEntity
	}
	var selectionErr *shapeshifter.ContractSelectionError
	if errors.As(err, &selectionErr) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func FormatContractError(err error) map[string]any {
	out := map[string]any{
		"error": "contract selection failed",
	}
	var selectionErr *shapeshifter.ContractSelectionError
	if !errors.As(err, &selectionErr) {
		out["code"] = shapeshifter.CodeUnknownContract
		return out
	}
	code := shapeshifter.CodeUnknownContract
	if selectionErr.Reason == shapeshifter.ContractReasonMissing {
		code = shapeshifter.CodeMissingContractHeader
	}
	out["code"] = code
	out["route"] = selectionErr.Route
	out["header"] = selectionErr.HeaderName
	if selectionErr.HeaderValue != "" {
		out["contract"] = selectionErr.HeaderValue
	}
	out["available_contracts"] = selectionErr.Available
	return out
}

func FormatProcessingError(err error) map[string]any {
	out := map[string]any{
		"error": "contract processing failed",
	}
	var ssErr *shapeshifter.ShapeShifterError
	if errors.As(err, &ssErr) {
		out["route"] = ssErr.Route
		out["contract"] = ssErr.ContractID
		out["phase"] = ssErr.Phase
		out["stage"] = ssErr.Stage
		if len(ssErr.Errors) > 0 {
			out["details"] = ssErr.Errors
		}
		return out
	}
	switch {
	case errors.Is(err, shapeshifter.ErrUnsupportedContentType):
		out["details"] = []shapeshifter.ValidationError{{Field: ".", Message: "unsupported content type", Code: shapeshifter.CodeUnsupportedContentType}}
	case errors.Is(err, shapeshifter.ErrRequestTooLarge):
		out["details"] = []shapeshifter.ValidationError{{Field: ".", Message: "request body too large", Code: shapeshifter.CodeRequestTooLarge}}
	}
	return out
}

func WriteProcessingError(w http.ResponseWriter, err error) {
	ResetHeadersForProcessingError(w.Header())
	status := StatusForError(err)
	payload, marshalErr := json.Marshal(FormatProcessingError(err))
	if marshalErr != nil {
		payload = []byte(`{"error":"contract processing failed"}`)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func ResetHeadersForProcessingError(h http.Header) {
	safe := http.Header{}
	for _, key := range []string{"Request-ID", "X-Request-ID", "Traceparent"} {
		if values := h.Values(key); len(values) > 0 {
			for _, value := range values {
				safe.Add(key, value)
			}
		}
	}
	for key := range h {
		h.Del(key)
	}
	for key, values := range safe {
		for _, value := range values {
			h.Add(key, value)
		}
	}
	h.Del("Content-Encoding")
	h.Del("Content-Length")
	h.Del("Transfer-Encoding")
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
}
