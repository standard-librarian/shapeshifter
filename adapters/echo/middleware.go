package echo

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	labstackecho "github.com/labstack/echo/v4"
	"github.com/standard-librarian/shapeshifter"
	"github.com/standard-librarian/shapeshifter/internal/adapter"
	"github.com/standard-librarian/shapeshifter/internal/capture"
)

func Middleware(engine *shapeshifter.Engine) labstackecho.MiddlewareFunc {
	return func(next labstackecho.HandlerFunc) labstackecho.HandlerFunc {
		return func(c labstackecho.Context) error {
			route := shapeshifter.RouteKey{
				Method: c.Request().Method,
				Path:   c.Path(),
			}

			if !engine.HasEndpoint(route) {
				engine.Emit(shapeshifter.BypassEvent(route, "", shapeshifter.BypassNoEndpoint))
				return next(c)
			}

			selection, err := engine.ResolveContract(route, c.Request().Header.Get(engine.HeaderName()))
			if err != nil {
				return c.JSON(http.StatusBadRequest, formatContractError(err))
			}

			if selection.HasRequest {
				if !isJSONRequest(c.Request().Header) {
					return c.JSON(http.StatusBadRequest, formatProcessingError(adapter.UnsupportedContentTypeError(selection)))
				}

				reqBody, err := readLimited(c.Request().Body, selection.Limits.RequestBodyBytes)
				_ = c.Request().Body.Close()
				if errors.Is(err, shapeshifter.ErrRequestTooLarge) {
					engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventRequestTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseRequest, Reason: err.Error()})
					return c.JSON(http.StatusRequestEntityTooLarge, formatProcessingError(adapter.RequestTooLargeError(selection)))
				}
				if err != nil {
					return c.JSON(http.StatusInternalServerError, formatProcessingError(err))
				}

				transformedReq, err := engine.ProcessRequest(c.Request().Context(), selection, reqBody, shapeshifter.ModeRuntime)
				if err != nil {
					return c.JSON(statusForError(err), formatProcessingError(err))
				}

				c.Request().Body = io.NopCloser(bytes.NewReader(transformedReq.Payload))
				c.Request().ContentLength = int64(len(transformedReq.Payload))
			}

			if !selection.HasResponse {
				engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, shapeshifter.BypassNoResponseSide))
				return next(c)
			}

			rc := capture.NewResponseCapture(c.Response().Writer, selection.Limits.ResponseBodyBytes)
			c.Response().Writer = rc
			err = next(c)
			c.Response().Writer = rc.ResponseWriter
			if err != nil {
				return err
			}

			if rc.TooLarge() {
				ssErr := adapter.ResponseTooLargeError(selection)
				engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventResponseTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseResponse, Stage: shapeshifter.StageTransform, Err: ssErr})
				writeProcessingError(rc.ResponseWriter, ssErr)
				return nil
			}

			ok, reason := isTransformableJSON(c.Request().Method, rc.StatusCode(), rc.Header(), rc.BodyLen(), selection.HasResponse)
			if !ok {
				engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, reason))
				rc.FlushOriginal(c.Request().Method)
				return nil
			}

			transformedResp, err := engine.ProcessResponse(c.Request().Context(), selection, rc.BodyBytes(), shapeshifter.ModeRuntime)
			if err != nil {
				writeProcessingError(rc.ResponseWriter, err)
				return nil
			}

			rc.FlushTransformed(rc.StatusCode(), transformedResp.Payload)
			return nil
		}
	}
}

func readLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	return adapter.ReadLimited(r, maxBytes)
}

func isJSONRequest(header http.Header) bool {
	return capture.IsJSONRequest(header)
}

func isTransformableJSON(method string, status int, header http.Header, bodyLen int, hasResponseSide bool) (bool, shapeshifter.BypassReason) {
	return capture.IsTransformableJSON(method, status, header, bodyLen, hasResponseSide)
}

func unsupportedContentTypeError(sel shapeshifter.ContractSelection) *shapeshifter.ShapeShifterError {
	return adapter.UnsupportedContentTypeError(sel)
}

func requestTooLargeError(sel shapeshifter.ContractSelection) *shapeshifter.ShapeShifterError {
	return adapter.RequestTooLargeError(sel)
}

func responseTooLargeError(sel shapeshifter.ContractSelection) *shapeshifter.ShapeShifterError {
	return adapter.ResponseTooLargeError(sel)
}

func statusForError(err error) int {
	return adapter.StatusForError(err)
}

func formatContractError(err error) map[string]any {
	return adapter.FormatContractError(err)
}

func formatProcessingError(err error) map[string]any {
	return adapter.FormatProcessingError(err)
}

func writeProcessingError(w http.ResponseWriter, err error) {
	adapter.WriteProcessingError(w, err)
}

func resetHeadersForProcessingError(h http.Header) {
	adapter.ResetHeadersForProcessingError(h)
}
