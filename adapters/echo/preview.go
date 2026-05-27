package echo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	labstackecho "github.com/labstack/echo/v4"
	"github.com/standard-librarian/shapeshifter"
)

const DefaultPreviewPrefix = "/_shapeshifter/api"

type PreviewRouter interface {
	GET(path string, h labstackecho.HandlerFunc, m ...labstackecho.MiddlewareFunc) *labstackecho.Route
	POST(path string, h labstackecho.HandlerFunc, m ...labstackecho.MiddlewareFunc) *labstackecho.Route
}

type PreviewOption func(*previewOptions)

type previewOptions struct {
	prefix string
}

func WithPreviewPrefix(prefix string) PreviewOption {
	return func(o *previewOptions) {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			return
		}
		o.prefix = "/" + strings.Trim(prefix, "/")
	}
}

func MountPreviewAPI(router PreviewRouter, engine *shapeshifter.Engine, opts ...PreviewOption) {
	cfg := previewOptions{prefix: DefaultPreviewPrefix}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	router.GET(cfg.prefix+"/spec", previewSpecHandler(engine))
	router.POST(cfg.prefix+"/process/request", previewProcessHandler(engine, shapeshifter.PhaseRequest))
	router.POST(cfg.prefix+"/process/response", previewProcessHandler(engine, shapeshifter.PhaseResponse))
}

func previewSpecHandler(engine *shapeshifter.Engine) labstackecho.HandlerFunc {
	return func(c labstackecho.Context) error {
		if engine == nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": "preview engine unavailable"})
		}
		return c.JSON(http.StatusOK, engine.SanitizedSpec())
	}
}

type previewProcessRequest struct {
	Route    shapeshifter.RouteKey `json:"route"`
	Contract string                `json:"contract,omitempty"`
	Body     json.RawMessage       `json:"body,omitempty"`
	Payload  json.RawMessage       `json:"payload,omitempty"`
}

type previewProcessResponse struct {
	Route           shapeshifter.RouteKey `json:"route"`
	Contract        string                `json:"contract"`
	Phase           shapeshifter.Phase    `json:"phase"`
	Payload         json.RawMessage       `json:"payload"`
	SkippedHandlers []string              `json:"skipped_handlers,omitempty"`
}

func previewProcessHandler(engine *shapeshifter.Engine, phase shapeshifter.Phase) labstackecho.HandlerFunc {
	return func(c labstackecho.Context) error {
		if engine == nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": "preview engine unavailable"})
		}
		var req previewProcessRequest
		dec := json.NewDecoder(c.Request().Body)
		if err := dec.Decode(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid preview request"})
		}
		if len(req.Body) == 0 && len(req.Payload) > 0 {
			req.Body = req.Payload
		}
		if len(req.Body) == 0 {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "preview body is required"})
		}

		selection, err := engine.ResolveContract(req.Route, req.Contract)
		if err != nil {
			return c.JSON(http.StatusBadRequest, formatContractError(err))
		}
		if phase == shapeshifter.PhaseRequest && !selection.HasRequest {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "contract has no request side"})
		}
		if phase == shapeshifter.PhaseResponse && !selection.HasResponse {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "contract has no response side"})
		}

		var result shapeshifter.ProcessResult
		if phase == shapeshifter.PhaseRequest {
			result, err = engine.ProcessRequest(c.Request().Context(), selection, req.Body, shapeshifter.ModePreview)
		} else {
			result, err = engine.ProcessResponse(c.Request().Context(), selection, req.Body, shapeshifter.ModePreview)
		}
		if err != nil {
			return c.JSON(statusForError(err), formatProcessingError(err))
		}
		if !json.Valid(result.Payload) {
			err := &shapeshifter.ShapeShifterError{
				Route:      selection.Route,
				ContractID: selection.ContractID,
				Phase:      phase,
				Stage:      shapeshifter.StageMarshal,
				Errors: []shapeshifter.ValidationError{{
					Field:   ".",
					Message: "marshal failed",
					Code:    shapeshifter.CodeMarshalFailed,
				}},
				Cause: errors.New("preview payload is not valid JSON"),
			}
			return c.JSON(statusForError(err), formatProcessingError(err))
		}

		return c.JSON(http.StatusOK, previewProcessResponse{
			Route:           selection.Route,
			Contract:        selection.ContractID,
			Phase:           phase,
			Payload:         json.RawMessage(result.Payload),
			SkippedHandlers: result.SkippedHandlers,
		})
	}
}
