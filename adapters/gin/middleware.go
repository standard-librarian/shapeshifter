package gin

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	ginframework "github.com/gin-gonic/gin"
	"github.com/standard-librarian/shapeshifter"
	"github.com/standard-librarian/shapeshifter/internal/adapter"
	"github.com/standard-librarian/shapeshifter/internal/capture"
)

func Middleware(engine *shapeshifter.Engine) ginframework.HandlerFunc {
	return func(c *ginframework.Context) {
		route := shapeshifter.RouteKey{
			Method: c.Request.Method,
			Path:   c.FullPath(),
		}

		if !engine.HasEndpoint(route) {
			engine.Emit(shapeshifter.BypassEvent(route, "", shapeshifter.BypassNoEndpoint))
			c.Next()
			return
		}

		selection, err := engine.ResolveContract(route, c.Request.Header.Get(engine.HeaderName()))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, adapter.FormatContractError(err))
			return
		}

		if selection.HasRequest {
			if !capture.IsJSONRequest(c.Request.Header) {
				c.AbortWithStatusJSON(http.StatusBadRequest, adapter.FormatProcessingError(adapter.UnsupportedContentTypeError(selection)))
				return
			}

			reqBody, err := adapter.ReadLimited(c.Request.Body, selection.Limits.RequestBodyBytes)
			_ = c.Request.Body.Close()
			if errors.Is(err, shapeshifter.ErrRequestTooLarge) {
				engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventRequestTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseRequest, Reason: err.Error()})
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, adapter.FormatProcessingError(adapter.RequestTooLargeError(selection)))
				return
			}
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, adapter.FormatProcessingError(err))
				return
			}

			transformedReq, err := engine.ProcessRequest(c.Request.Context(), selection, reqBody, shapeshifter.ModeRuntime)
			if err != nil {
				c.AbortWithStatusJSON(adapter.StatusForError(err), adapter.FormatProcessingError(err))
				return
			}

			c.Request.Body = io.NopCloser(bytes.NewReader(transformedReq.Payload))
			c.Request.ContentLength = int64(len(transformedReq.Payload))
		}

		if !selection.HasResponse {
			engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, shapeshifter.BypassNoResponseSide))
			c.Next()
			return
		}

		original := c.Writer
		rc := capture.NewResponseCapture(original, selection.Limits.ResponseBodyBytes)
		gw := &responseWriter{ResponseWriter: original, capture: rc}
		c.Writer = gw
		c.Next()
		c.Writer = original

		if rc.TooLarge() {
			ssErr := adapter.ResponseTooLargeError(selection)
			engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventResponseTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseResponse, Stage: shapeshifter.StageTransform, Err: ssErr})
			adapter.WriteProcessingError(original, ssErr)
			return
		}

		ok, reason := capture.IsTransformableJSON(c.Request.Method, rc.StatusCode(), rc.Header(), rc.BodyLen(), selection.HasResponse)
		if !ok {
			engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, reason))
			rc.FlushOriginal(c.Request.Method)
			return
		}

		transformedResp, err := engine.ProcessResponse(c.Request.Context(), selection, rc.BodyBytes(), shapeshifter.ModeRuntime)
		if err != nil {
			adapter.WriteProcessingError(original, err)
			return
		}

		rc.FlushTransformed(rc.StatusCode(), transformedResp.Payload)
	}
}

type responseWriter struct {
	ginframework.ResponseWriter
	capture *capture.ResponseCapture
	size    int
}

func (w *responseWriter) WriteHeader(code int) {
	w.capture.WriteHeader(code)
}

func (w *responseWriter) WriteHeaderNow() {
	if !w.Written() {
		w.WriteHeader(w.Status())
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	n, err := w.capture.Write(p)
	w.size += n
	return n, err
}

func (w *responseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *responseWriter) Status() int {
	return w.capture.StatusCode()
}

func (w *responseWriter) Size() int {
	if w.size == 0 && !w.Written() {
		return -1
	}
	return w.size
}

func (w *responseWriter) Written() bool {
	return w.capture.BodyLen() > 0 || w.Status() != http.StatusOK
}
