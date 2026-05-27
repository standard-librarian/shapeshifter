package fiber

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	fiberframework "github.com/gofiber/fiber/v2"
	"github.com/standard-librarian/shapeshifter"
	"github.com/standard-librarian/shapeshifter/internal/adapter"
	"github.com/standard-librarian/shapeshifter/internal/capture"
)

func Middleware(engine *shapeshifter.Engine) fiberframework.Handler {
	return func(c *fiberframework.Ctx) error {
		routePath := ""
		if route := c.Route(); route != nil {
			routePath = route.Path
		}
		route := shapeshifter.RouteKey{
			Method: c.Method(),
			Path:   routePath,
		}

		if !engine.HasEndpoint(route) {
			engine.Emit(shapeshifter.BypassEvent(route, "", shapeshifter.BypassNoEndpoint))
			return c.Next()
		}

		selection, err := engine.ResolveContract(route, c.Get(engine.HeaderName()))
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(adapter.FormatContractError(err))
		}

		if selection.HasRequest {
			if !capture.IsJSONMediaType(c.Get("Content-Type")) {
				return c.Status(http.StatusBadRequest).JSON(adapter.FormatProcessingError(adapter.UnsupportedContentTypeError(selection)))
			}

			reqBody, err := adapter.ReadLimited(bytes.NewReader(c.BodyRaw()), selection.Limits.RequestBodyBytes)
			if errors.Is(err, shapeshifter.ErrRequestTooLarge) {
				engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventRequestTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseRequest, Reason: err.Error()})
				return c.Status(http.StatusRequestEntityTooLarge).JSON(adapter.FormatProcessingError(adapter.RequestTooLargeError(selection)))
			}
			if err != nil {
				return c.Status(http.StatusInternalServerError).JSON(adapter.FormatProcessingError(err))
			}

			transformedReq, err := engine.ProcessRequest(c.UserContext(), selection, reqBody, shapeshifter.ModeRuntime)
			if err != nil {
				return c.Status(adapter.StatusForError(err)).JSON(adapter.FormatProcessingError(err))
			}

			c.Request().SetBody(transformedReq.Payload)
			c.Request().Header.SetContentLength(len(transformedReq.Payload))
		}

		if !selection.HasResponse {
			engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, shapeshifter.BypassNoResponseSide))
			return c.Next()
		}

		if err := c.Next(); err != nil {
			return err
		}

		body := c.Response().Body()
		if int64(len(body)) > selection.Limits.ResponseBodyBytes {
			ssErr := adapter.ResponseTooLargeError(selection)
			engine.Emit(shapeshifter.Event{Kind: shapeshifter.EventResponseTooLarge, Route: route, ContractID: selection.ContractID, Phase: shapeshifter.PhaseResponse, Stage: shapeshifter.StageTransform, Err: ssErr})
			return writeFiberProcessingError(c, ssErr)
		}

		header := fiberResponseHeader(c)
		ok, reason := capture.IsTransformableJSON(c.Method(), c.Response().StatusCode(), header, len(body), selection.HasResponse)
		if !ok {
			engine.Emit(shapeshifter.BypassEvent(route, selection.ContractID, reason))
			return nil
		}

		transformedResp, err := engine.ProcessResponse(c.UserContext(), selection, body, shapeshifter.ModeRuntime)
		if err != nil {
			return writeFiberProcessingError(c, err)
		}

		c.Response().Header.Del("Transfer-Encoding")
		c.Response().Header.Del("Content-Encoding")
		c.Response().Header.SetContentLength(len(transformedResp.Payload))
		c.Response().SetBody(transformedResp.Payload)
		return nil
	}
}

func fiberResponseHeader(c *fiberframework.Ctx) http.Header {
	h := http.Header{}
	c.Response().Header.VisitAll(func(key, value []byte) {
		h.Add(string(key), string(value))
	})
	return h
}

func writeFiberProcessingError(c *fiberframework.Ctx, err error) error {
	resetFiberHeadersForProcessingError(c)
	status := adapter.StatusForError(err)
	payload, marshalErr := json.Marshal(adapter.FormatProcessingError(err))
	if marshalErr != nil {
		payload = []byte(`{"error":"contract processing failed"}`)
	}
	c.Response().Header.SetContentLength(len(payload))
	c.Response().SetStatusCode(status)
	c.Response().SetBody(payload)
	return nil
}

func resetFiberHeadersForProcessingError(c *fiberframework.Ctx) {
	h := &c.Response().Header
	safe := map[string][]string{}
	for _, key := range []string{"Request-ID", "X-Request-ID", "Traceparent"} {
		values := h.PeekAll(key)
		for _, value := range values {
			safe[key] = append(safe[key], string(value))
		}
	}
	h.Reset()
	for key, values := range safe {
		for _, value := range values {
			h.Add(key, value)
		}
	}
	h.Del("Content-Encoding")
	h.Del("Content-Length")
	h.Del("Transfer-Encoding")
	h.SetContentType("application/json")
	h.Set("Cache-Control", "no-store")
}
