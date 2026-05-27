package shapeshifter

import (
	"errors"
	"fmt"
)

var (
	ErrRequestTooLarge        = errors.New("shapeshifter: request body too large")
	ErrUnsupportedContentType = errors.New("shapeshifter: unsupported content type")
)

type ValidationError struct {
	Field   string    `json:"field"`
	Message string    `json:"message"`
	Code    ErrorCode `json:"code"`
}

type ErrorCode string

const (
	CodeMalformedJSON             ErrorCode = "malformed_json"
	CodeEmptyRequestBody          ErrorCode = "empty_request_body"
	CodeUnsupportedContentType    ErrorCode = "unsupported_content_type"
	CodeRequestTooLarge           ErrorCode = "request_too_large"
	CodeSourceSchemaFailed        ErrorCode = "source_schema_failed"
	CodeValidationRuleFailed      ErrorCode = "validation_rule_failed"
	CodeMissingRequiredField      ErrorCode = "missing_required_field"
	CodeMultipleJQOutputs         ErrorCode = "multiple_jq_outputs"
	CodeNumberNormalizationFailed ErrorCode = "number_normalization_failed"
	CodeCoercionFailed            ErrorCode = "coercion_failed"
	CodeHandlerValidationFailed   ErrorCode = "handler_validation_failed"
	CodeHandlerFailed             ErrorCode = "handler_failed"
	CodeTargetSchemaFailed        ErrorCode = "target_schema_failed"
	CodeMarshalFailed             ErrorCode = "marshal_failed"
	CodeResponseTooLarge          ErrorCode = "response_too_large"
	CodeMissingContractHeader     ErrorCode = "missing_contract_header"
	CodeUnknownContract           ErrorCode = "unknown_contract"
)

type Phase string

const (
	PhaseRequest  Phase = "request"
	PhaseResponse Phase = "response"
)

type Stage string

const (
	StageDecode          Stage = "decode"
	StageSourceValidate  Stage = "source_validate"
	StageNumberNormalize Stage = "number_normalize"
	StageTransform       Stage = "transform"
	StageHandler         Stage = "handler"
	StageTargetValidate  Stage = "target_validate"
	StageMarshal         Stage = "marshal"
)

type ShapeShifterError struct {
	Route      RouteKey
	ContractID string
	Phase      Phase
	Stage      Stage
	Errors     []ValidationError
	Cause      error
}

func (e *ShapeShifterError) Error() string {
	if e == nil {
		return "shapeshifter error"
	}
	return fmt.Sprintf("shapeshifter %s/%s failed for %s %s contract %s", e.Phase, e.Stage, e.Route.Method, e.Route.Path, e.ContractID)
}

func (e *ShapeShifterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type HandlerValidationError struct {
	Errors []ValidationError
}

func (e *HandlerValidationError) Error() string {
	return "shapeshifter handler validation failed"
}

type ContractSelectionReason string

const (
	ContractReasonMissing    ContractSelectionReason = "missing"
	ContractReasonUnknown    ContractSelectionReason = "unknown"
	ContractReasonNoEndpoint ContractSelectionReason = "no_endpoint"
)

type ContractSelectionError struct {
	Route       RouteKey
	HeaderName  string
	HeaderValue string
	Available   []string
	Reason      ContractSelectionReason
}

func (e *ContractSelectionError) Error() string {
	if e == nil {
		return "shapeshifter contract selection failed"
	}
	return fmt.Sprintf("shapeshifter contract selection failed for %s %s: %s", e.Route.Method, e.Route.Path, e.Reason)
}
