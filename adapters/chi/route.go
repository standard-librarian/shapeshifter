package chi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/standard-librarian/shapeshifter"
	"github.com/standard-librarian/shapeshifter/internal/adapter"
	"github.com/standard-librarian/shapeshifter/internal/capture"
)

func Route(engine *shapeshifter.Engine, pattern string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := shapeshifter.RouteKey{
				Method: r.Method,
				Path:   pattern,
			}

			if !engine.HasEndpoint(route) {
				engine.Emit(shapeshifter.BypassEvent(route, "", shapeshifter.BypassNoEndpoint))
				next.ServeHTTP(w, r)
				return
			}

			selection, err := engine.ResolveContract(route, r.Header.Get(engine.HeaderName()))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, adapter.FormatContractError(err))
				return
			}

			if selection.HasRequest {
				if !capture.IsJSONRequest(r.Header) {
					writeJSON(w, http.StatusBadRequest, adapter.FormatProcessingError(adapter.UnsupportedContentTypeError(selection)))
					return
				}

				reqBody, err := adapter.ReadLimited(r.Body, selection.Limits.RequestBodyBytes)
				_ = r.Body.Close()
				if errors.Is(err, shapeshifter.ErrRequestTooLarge) {
					engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventRequestTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseRequest, Reason: err.Error()})
					writeJSON(w, http.StatusRequestEntityTooLarge, adapter.FormatProcessingError(adapter.RequestTooLargeError(selection)))
					return
				}
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, adapter.FormatProcessingError(err))
					return
				}

				transformedReq, err := engine.ProcessRequest(r.Context(), selection, reqBody, shapeshifter.ModeRuntime)
				if err != nil {
					writeJSON(w, adapter.StatusForError(err), adapter.FormatProcessingError(err))
					return
				}

				r.Body = io.NopCloser(bytes.NewReader(transformedReq.Payload))
				r.ContentLength = int64(len(transformedReq.Payload))
			}

			if !selection.HasResponse {
				engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, shapeshifter.BypassNoResponseSide))
				next.ServeHTTP(w, r)
				return
			}

			rc := capture.NewResponseCapture(w, selection.Limits.ResponseBodyBytes)
			next.ServeHTTP(rc, r)

			if rc.TooLarge() {
				ssErr := adapter.ResponseTooLargeError(selection)
				engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventResponseTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseResponse, Stage: shapeshifter.StageTransform, Err: ssErr})
				adapter.WriteProcessingError(w, ssErr)
				return
			}

			ok, reason := capture.IsTransformableJSON(r.Method, rc.StatusCode(), rc.Header(), rc.BodyLen(), selection.HasResponse)
			if !ok {
				engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, reason))
				rc.FlushOriginal(r.Method)
				return
			}

			transformedResp, err := engine.ProcessResponse(r.Context(), selection, rc.BodyBytes(), shapeshifter.ModeRuntime)
			if err != nil {
				adapter.WriteProcessingError(w, err)
				return
			}

			rc.FlushTransformed(rc.StatusCode(), transformedResp.Payload)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		payload = []byte(`{"error":"contract processing failed"}`)
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", strconv.Itoa(len(payload)))
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
