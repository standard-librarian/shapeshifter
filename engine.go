package shapeshifter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	schemadetail "github.com/standard-librarian/shapeshifter/internal/schema"
	"github.com/standard-librarian/shapeshifter/internal/transform"
)

type ContractSelection struct {
	Route       RouteKey
	ContractID  string
	Available   []string
	HasRequest  bool
	HasResponse bool
	Limits      Limits
}

type ProcessResult struct {
	Payload         []byte
	SkippedHandlers []string
}

type ProcessMode int

const (
	ModeRuntime ProcessMode = iota
	ModePreview
)

type Engine struct {
	spec     *Spec
	observer Observer
	jqLimits transform.JQLimits
}

func NewEngine(spec *Spec, opts ...Option) (*Engine, error) {
	if spec == nil {
		return nil, errors.New("shapeshifter: spec is nil")
	}
	cfg := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &Engine{spec: spec, observer: cfg.observer, jqLimits: cfg.jqLimits}, nil
}

func (e *Engine) HeaderName() string {
	if e == nil || e.spec == nil {
		return ""
	}
	return e.spec.header
}

func (e *Engine) HasEndpoint(route RouteKey) bool {
	if e == nil || e.spec == nil {
		return false
	}
	route.Method = strings.ToUpper(route.Method)
	_, ok := e.spec.endpoints[route]
	return ok
}

func (e *Engine) Contracts(route RouteKey) []string {
	if e == nil || e.spec == nil {
		return nil
	}
	route.Method = strings.ToUpper(route.Method)
	return sortedContractIDs(e.spec.endpoints[route])
}

func (e *Engine) SanitizedSpec() SanitizedSpec {
	if e == nil || e.spec == nil {
		return SanitizedSpec{Version: "1"}
	}
	return e.spec.Sanitized()
}

func (e *Engine) ResolveContract(route RouteKey, headerValue string) (ContractSelection, error) {
	var zero ContractSelection
	if e == nil || e.spec == nil {
		return zero, errors.New("shapeshifter: engine is nil")
	}
	route.Method = strings.ToUpper(route.Method)
	ep := e.spec.endpoints[route]
	if ep == nil {
		return zero, &ContractSelectionError{
			Route:      route,
			HeaderName: e.spec.header,
			Reason:     ContractReasonNoEndpoint,
		}
	}
	available := sortedContractIDs(ep)
	id := strings.TrimSpace(headerValue)
	if id == "" {
		if ep.defaultContract == "" {
			return zero, &ContractSelectionError{
				Route:      route,
				HeaderName: e.spec.header,
				Available:  available,
				Reason:     ContractReasonMissing,
			}
		}
		id = ep.defaultContract
	}
	contract := ep.contracts[id]
	if contract == nil {
		return zero, &ContractSelectionError{
			Route:       route,
			HeaderName:  e.spec.header,
			HeaderValue: id,
			Available:   available,
			Reason:      ContractReasonUnknown,
		}
	}
	sel := ContractSelection{
		Route:       route,
		ContractID:  id,
		Available:   available,
		HasRequest:  contract.request != nil,
		HasResponse: contract.response != nil,
		Limits:      ep.limits,
	}
	e.Emit(Event{Kind: EventContractSelected, Route: route, ContractID: id, InBytes: len(headerValue)})
	return sel, nil
}

func (e *Engine) Emit(event Event) {
	if e == nil || e.observer == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	e.observer.OnShapeShifterEvent(event)
}

func (e *Engine) ProcessRequest(ctx context.Context, sel ContractSelection, input []byte, mode ProcessMode) (ProcessResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	side, err := e.side(sel, PhaseRequest)
	if err != nil {
		return ProcessResult{}, err
	}
	return e.process(ctx, sel, side, input, mode, PhaseRequest)
}

func (e *Engine) ProcessResponse(ctx context.Context, sel ContractSelection, input []byte, mode ProcessMode) (ProcessResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	side, err := e.side(sel, PhaseResponse)
	if err != nil {
		return ProcessResult{}, err
	}
	return e.process(ctx, sel, side, input, mode, PhaseResponse)
}

func (e *Engine) side(sel ContractSelection, phase Phase) (*sideSpec, error) {
	if e == nil || e.spec == nil {
		return nil, errors.New("shapeshifter: engine is nil")
	}
	route := sel.Route
	route.Method = strings.ToUpper(route.Method)
	ep := e.spec.endpoints[route]
	if ep == nil {
		return nil, fmt.Errorf("shapeshifter: endpoint %s %s not found", route.Method, route.Path)
	}
	contract := ep.contracts[sel.ContractID]
	if contract == nil {
		return nil, fmt.Errorf("shapeshifter: contract %q not found for %s %s", sel.ContractID, route.Method, route.Path)
	}
	if phase == PhaseRequest {
		if contract.request == nil {
			return nil, fmt.Errorf("shapeshifter: contract %q has no request side", sel.ContractID)
		}
		return contract.request, nil
	}
	if contract.response == nil {
		return nil, fmt.Errorf("shapeshifter: contract %q has no response side", sel.ContractID)
	}
	return contract.response, nil
}

func (e *Engine) process(ctx context.Context, sel ContractSelection, side *sideSpec, input []byte, mode ProcessMode, phase Phase) (ProcessResult, error) {
	start := time.Now()
	source, err := decodeJSONObject(input)
	if err != nil {
		code := CodeMalformedJSON
		if errors.Is(err, io.EOF) {
			code = CodeEmptyRequestBody
		}
		return ProcessResult{}, e.processingError(sel, phase, StageDecode, []ValidationError{{
			Field:   ".",
			Message: decodeMessage(code),
			Code:    code,
		}}, err)
	}

	if side.source != nil {
		if err := side.source.Validate(source); err != nil {
			return ProcessResult{}, e.processingError(sel, phase, StageSourceValidate, schemaErrors(err, CodeSourceSchemaFailed), err)
		}
	}

	normalized, err := transform.NormalizeNumbers(source)
	if err != nil {
		return ProcessResult{}, e.processingError(sel, phase, StageNumberNormalize, []ValidationError{{
			Field:   ".",
			Message: "number normalization failed",
			Code:    CodeNumberNormalizationFailed,
		}}, err)
	}
	source = normalized.(map[string]any)

	if errs := e.runValidationRules(ctx, source, side.transform.validations); len(errs) > 0 {
		return ProcessResult{}, e.processingError(sel, phase, StageTransform, errs, nil)
	}

	target, errs := e.buildTarget(ctx, source, side.transform)
	if len(errs) > 0 {
		return ProcessResult{}, e.processingError(sel, phase, StageTransform, errs, nil)
	}

	for _, rule := range side.transform.coercions {
		if err := transform.ApplyCoerce(target, rule.field, rule.typ); err != nil {
			return ProcessResult{}, e.processingError(sel, phase, StageTransform, []ValidationError{{
				Field:   rule.fieldString,
				Message: "coercion failed",
				Code:    CodeCoercionFailed,
			}}, err)
		}
	}

	result := ProcessResult{}
	if side.transform.handler != nil {
		handler := *side.transform.handler
		if mode == ModePreview && !handler.PreviewSafe {
			result.SkippedHandlers = append(result.SkippedHandlers, handler.Name)
		} else {
			target, err = callHandler(handler, target)
			if err != nil {
				var hv *HandlerValidationError
				if errors.As(err, &hv) {
					details := normalizeHandlerValidationErrors(hv.Errors)
					return ProcessResult{}, e.processingError(sel, phase, StageHandler, details, err)
				}
				return ProcessResult{}, e.processingError(sel, phase, StageHandler, []ValidationError{{
					Field:   ".",
					Message: "handler failed",
					Code:    CodeHandlerFailed,
				}}, err)
			}
			if target == nil {
				err := errors.New("handler returned nil target")
				return ProcessResult{}, e.processingError(sel, phase, StageHandler, []ValidationError{{
					Field:   ".",
					Message: "handler returned nil target",
					Code:    CodeHandlerFailed,
				}}, err)
			}
		}
	}

	if side.target != nil {
		if err := side.target.Validate(target); err != nil {
			return ProcessResult{}, e.processingError(sel, phase, StageTargetValidate, schemaErrors(err, CodeTargetSchemaFailed), err)
		}
	}

	payload, err := json.Marshal(target)
	if err != nil {
		return ProcessResult{}, e.processingError(sel, phase, StageMarshal, []ValidationError{{
			Field:   ".",
			Message: "marshal failed",
			Code:    CodeMarshalFailed,
		}}, err)
	}
	result.Payload = payload

	kind := EventRequestTransformed
	if phase == PhaseResponse {
		kind = EventResponseTransformed
	}
	e.Emit(Event{
		Kind:       kind,
		Route:      sel.Route,
		ContractID: sel.ContractID,
		Phase:      phase,
		Duration:   time.Since(start),
		InBytes:    len(input),
		OutBytes:   len(payload),
	})

	return result, nil
}

func decodeJSONObject(input []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return nil, io.EOF
	}
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON root must be an object")
	}
	return obj, nil
}

func decodeMessage(code ErrorCode) string {
	if code == CodeEmptyRequestBody {
		return "empty request body"
	}
	return "malformed JSON"
}

func (e *Engine) runValidationRules(ctx context.Context, source map[string]any, rules []validationRule) []ValidationError {
	var errs []ValidationError
	for _, rule := range rules {
		value, err := rule.field.EvalOne(ctx, source, e.jqLimits)
		if err != nil {
			errs = append(errs, jqValidationError(rule.fieldString, err, "validation field failed"))
			continue
		}
		if value == nil {
			if rule.required {
				errs = append(errs, ValidationError{Field: rule.fieldString, Message: rule.message, Code: CodeMissingRequiredField})
			}
			continue
		}
		result, err := rule.rule.EvalOne(ctx, value, e.jqLimits)
		if err != nil {
			errs = append(errs, jqValidationError(rule.fieldString, err, rule.message))
			continue
		}
		ok, isBool := result.(bool)
		if !isBool || !ok {
			errs = append(errs, ValidationError{Field: rule.fieldString, Message: rule.message, Code: CodeValidationRuleFailed})
		}
	}
	return errs
}

func (e *Engine) buildTarget(ctx context.Context, source map[string]any, compiled compiledTransform) (map[string]any, []ValidationError) {
	if compiled.passthrough {
		return source, nil
	}
	target := map[string]any{}
	var errs []ValidationError
	for _, field := range compiled.fields {
		value, err := field.from.EvalOne(ctx, source, e.jqLimits)
		if err != nil {
			errs = append(errs, jqValidationError(field.toString, err, "field mapping failed"))
			continue
		}
		if value == nil {
			if field.required {
				errs = append(errs, ValidationError{Field: field.toString, Message: "missing required field", Code: CodeMissingRequiredField})
			}
			continue
		}
		if err := transform.SetPath(target, field.to, value); err != nil {
			errs = append(errs, ValidationError{Field: field.toString, Message: "field mapping failed", Code: CodeValidationRuleFailed})
		}
	}
	return target, errs
}

func jqValidationError(field string, err error, message string) ValidationError {
	switch {
	case errors.Is(err, transform.ErrMultipleJQOutput):
		return ValidationError{Field: field, Message: "jq expression produced multiple outputs", Code: CodeMultipleJQOutputs}
	case errors.Is(err, transform.ErrNoJQOutput):
		return ValidationError{Field: field, Message: "missing required field", Code: CodeMissingRequiredField}
	default:
		return ValidationError{Field: field, Message: message, Code: CodeValidationRuleFailed}
	}
}

func callHandler(handler RegisteredHandler, target map[string]any) (out map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler %q panicked", handler.Name)
			out = nil
		}
	}()
	return handler.Fn(target)
}

func normalizeHandlerValidationErrors(errs []ValidationError) []ValidationError {
	if len(errs) == 0 {
		return []ValidationError{{Field: ".", Message: "handler validation failed", Code: CodeHandlerValidationFailed}}
	}
	out := make([]ValidationError, len(errs))
	for i, err := range errs {
		out[i] = err
		if out[i].Code == "" {
			out[i].Code = CodeHandlerValidationFailed
		}
		if out[i].Message == "" {
			out[i].Message = "handler validation failed"
		}
		if out[i].Field == "" {
			out[i].Field = "."
		}
	}
	return out
}

func schemaErrors(err error, code ErrorCode) []ValidationError {
	raw := schemadetail.FlattenValidationError(err)
	out := make([]ValidationError, 0, len(raw))
	for _, item := range raw {
		out = append(out, ValidationError{Field: item.Field, Message: item.Message, Code: code})
	}
	return out
}

func (e *Engine) processingError(sel ContractSelection, phase Phase, stage Stage, details []ValidationError, cause error) *ShapeShifterError {
	err := &ShapeShifterError{
		Route:      sel.Route,
		ContractID: sel.ContractID,
		Phase:      phase,
		Stage:      stage,
		Errors:     details,
		Cause:      cause,
	}
	e.emitProcessingFailure(err)
	return err
}

func (e *Engine) emitProcessingFailure(err *ShapeShifterError) {
	kind := EventTransformFailed
	switch err.Stage {
	case StageSourceValidate, StageTargetValidate:
		if err.Phase == PhaseRequest {
			kind = EventRequestValidationFailed
		} else {
			kind = EventResponseValidationFailed
		}
	case StageHandler:
		kind = EventHandlerFailed
	}
	e.Emit(Event{
		Kind:       kind,
		Route:      err.Route,
		ContractID: err.ContractID,
		Phase:      err.Phase,
		Stage:      err.Stage,
		Err:        err.Cause,
	})
}
