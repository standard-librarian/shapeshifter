package shapeshifter

import "time"

type Observer interface {
	OnShapeShifterEvent(Event)
}

type ObserverFunc func(Event)

func (f ObserverFunc) OnShapeShifterEvent(event Event) {
	if f != nil {
		f(event)
	}
}

type EventKind string

const (
	EventContractSelected         EventKind = "contract_selected"
	EventBypassNoEndpoint         EventKind = "bypass_no_endpoint"
	EventBypassNoResponseSide     EventKind = "bypass_no_response_side"
	EventBypassHeadRequest        EventKind = "bypass_head_request"
	EventBypassNonJSONContentType EventKind = "bypass_non_json_content_type"
	EventBypassContentEncoding    EventKind = "bypass_content_encoding"
	EventBypassStatus             EventKind = "bypass_status"
	EventRequestTooLarge          EventKind = "request_too_large"
	EventResponseTooLarge         EventKind = "response_too_large"
	EventRequestValidationFailed  EventKind = "request_validation_failed"
	EventResponseValidationFailed EventKind = "response_validation_failed"
	EventRequestTransformed       EventKind = "request_transformed"
	EventResponseTransformed      EventKind = "response_transformed"
	EventTransformFailed          EventKind = "transform_failed"
	EventHandlerFailed            EventKind = "handler_failed"
)

type Event struct {
	Kind       EventKind
	Route      RouteKey
	ContractID string
	Phase      Phase
	Stage      Stage
	StatusCode int
	Reason     string
	Duration   time.Duration
	InBytes    int
	OutBytes   int
	Err        error
}

type BypassReason string

const (
	BypassNone            BypassReason = ""
	BypassNoEndpoint      BypassReason = "no_endpoint"
	BypassNoResponseSide  BypassReason = "no_response_side"
	BypassHead            BypassReason = "head_request"
	BypassStatus          BypassReason = "status_not_transformable"
	BypassContentType     BypassReason = "non_json_content_type"
	BypassContentEncoding BypassReason = "encoded_response"
	BypassTooLarge        BypassReason = "response_too_large"
)

func BypassEvent(route RouteKey, contractID string, reason BypassReason) Event {
	kind := EventTransformFailed
	switch reason {
	case BypassNoEndpoint:
		kind = EventBypassNoEndpoint
	case BypassNoResponseSide:
		kind = EventBypassNoResponseSide
	case BypassHead:
		kind = EventBypassHeadRequest
	case BypassStatus:
		kind = EventBypassStatus
	case BypassContentType:
		kind = EventBypassNonJSONContentType
	case BypassContentEncoding:
		kind = EventBypassContentEncoding
	}
	return Event{Kind: kind, Route: route, ContractID: contractID, Reason: string(reason)}
}

type noopObserver struct{}

func (noopObserver) OnShapeShifterEvent(Event) {}
