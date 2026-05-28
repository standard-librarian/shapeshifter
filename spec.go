package shapeshifter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	schemacompiler "github.com/standard-librarian/shapeshifter/internal/schema"
	rawspec "github.com/standard-librarian/shapeshifter/internal/spec"
	"github.com/standard-librarian/shapeshifter/internal/transform"
)

const defaultHeaderName = "X-ShapeShifter-Contract"

type Option func(*options)

type options struct {
	observer Observer
	jqLimits transform.JQLimits
}

func defaultOptions() options {
	return options{
		observer: noopObserver{},
		jqLimits: transform.DefaultJQLimits(),
	}
}

func WithObserver(observer Observer) Option {
	return func(o *options) {
		if observer == nil {
			o.observer = noopObserver{}
			return
		}
		o.observer = observer
	}
}

type Spec struct {
	header       string
	title        string
	description  string
	shapes       []string
	shapeSchemas map[string]any
	endpoints    map[RouteKey]*endpointSpec
}

type endpointSpec struct {
	route           RouteKey
	summary         string
	description     string
	tags            []string
	defaultContract string
	limits          Limits
	contracts       map[string]*contractSpec
	order           []string
}

type contractSpec struct {
	id          string
	summary     string
	description string
	deprecated  bool
	request     *sideSpec
	response    *sideSpec
}

type sideSpec struct {
	sourceShape string
	targetShape string
	description string
	examples    []compiledExample
	source      schemaValidator
	target      schemaValidator
	transform   compiledTransform
}

type compiledExample struct {
	name        string
	description string
	body        any
}

type schemaValidator interface {
	Validate(v any) error
}

type compiledTransform struct {
	passthrough bool
	fields      []fieldMapping
	validations []validationRule
	coercions   []coerceRule
	handler     *RegisteredHandler
}

type fieldMapping struct {
	from     *transform.Expression
	to       []string
	toString string
	required bool
}

type validationRule struct {
	field       *transform.Expression
	fieldString string
	rule        *transform.Expression
	message     string
	required    bool
}

type coerceRule struct {
	field       []string
	fieldString string
	typ         transform.CoerceType
}

func LoadSpec(r io.Reader, handlers HandlerSnapshot, opts ...Option) (*Spec, error) {
	if r == nil {
		return nil, fmt.Errorf("shapeshifter: spec reader is nil")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("shapeshifter: read spec: %w", err)
	}
	return LoadSpecBytes(data, handlers, opts...)
}

func LoadSpecBytes(data []byte, handlers HandlerSnapshot, opts ...Option) (*Spec, error) {
	raw, err := rawspec.Load(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return compileSpec(raw, handlers)
}

func LoadSpecFile(path string, handlers HandlerSnapshot, opts ...Option) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("shapeshifter: read spec file %q: %w", path, err)
	}
	return LoadSpecBytes(data, handlers, opts...)
}

func compileSpec(raw *rawspec.Spec, handlers HandlerSnapshot) (*Spec, error) {
	if raw.Version != "1" {
		if raw.Version == "" {
			return nil, fmt.Errorf("shapeshifter: spec version is required")
		}
		return nil, fmt.Errorf("shapeshifter: unsupported spec version %q", raw.Version)
	}

	header := strings.TrimSpace(raw.Header)
	if header == "" {
		header = defaultHeaderName
	}

	if err := rawspec.RejectRefs(raw.Shapes); err != nil {
		return nil, err
	}

	compiledShapes, err := compileShapes(raw.Shapes)
	if err != nil {
		return nil, err
	}

	topLimits, err := compileLimits(raw.Limits, true)
	if err != nil {
		return nil, err
	}

	out := &Spec{
		header:       header,
		title:        strings.TrimSpace(raw.Title),
		description:  strings.TrimSpace(raw.Description),
		shapes:       sortedShapeNames(raw.Shapes),
		shapeSchemas: cloneShapeSchemas(raw.Shapes),
		endpoints:    map[RouteKey]*endpointSpec{},
	}

	for _, rawEndpoint := range raw.Endpoints {
		method := strings.ToUpper(strings.TrimSpace(rawEndpoint.Method))
		path := strings.TrimSpace(rawEndpoint.Path)
		if method == "" || path == "" {
			return nil, fmt.Errorf("shapeshifter: endpoint method and path are required")
		}
		route := RouteKey{Method: method, Path: path}
		if _, exists := out.endpoints[route]; exists {
			return nil, fmt.Errorf("shapeshifter: duplicate endpoint %s %s", method, path)
		}

		limits := topLimits
		if rawEndpoint.Limits != nil {
			limits, err = overrideLimits(topLimits, rawEndpoint.Limits)
			if err != nil {
				return nil, fmt.Errorf("shapeshifter: endpoint %s %s: %w", method, path, err)
			}
		}

		ep := &endpointSpec{
			route:           route,
			summary:         strings.TrimSpace(rawEndpoint.Summary),
			description:     strings.TrimSpace(rawEndpoint.Description),
			tags:            append([]string(nil), rawEndpoint.Tags...),
			defaultContract: strings.TrimSpace(rawEndpoint.DefaultContract),
			limits:          limits,
			contracts:       map[string]*contractSpec{},
		}

		for _, rawContract := range rawEndpoint.Contracts {
			id := strings.TrimSpace(rawContract.ID)
			if id == "" {
				return nil, fmt.Errorf("shapeshifter: endpoint %s %s has contract with empty id", method, path)
			}
			if _, exists := ep.contracts[id]; exists {
				return nil, fmt.Errorf("shapeshifter: duplicate contract %q for %s %s", id, method, path)
			}
			if rawContract.Request == nil && rawContract.Response == nil {
				return nil, fmt.Errorf("shapeshifter: contract %q for %s %s must define request or response", id, method, path)
			}

			contract := &contractSpec{
				id:          id,
				summary:     strings.TrimSpace(rawContract.Summary),
				description: strings.TrimSpace(rawContract.Description),
				deprecated:  rawContract.Deprecated,
			}
			if rawContract.Request != nil {
				side, err := compileRequestSide(id, rawContract.Request, compiledShapes, raw.Shapes, handlers)
				if err != nil {
					return nil, fmt.Errorf("shapeshifter: contract %q request: %w", id, err)
				}
				contract.request = side
			}
			if rawContract.Response != nil {
				side, err := compileResponseSide(id, rawContract.Response, compiledShapes, handlers)
				if err != nil {
					return nil, fmt.Errorf("shapeshifter: contract %q response: %w", id, err)
				}
				contract.response = side
			}

			ep.contracts[id] = contract
			ep.order = append(ep.order, id)
		}

		if len(ep.contracts) == 0 {
			return nil, fmt.Errorf("shapeshifter: endpoint %s %s must define at least one contract", method, path)
		}
		if ep.defaultContract != "" {
			if _, exists := ep.contracts[ep.defaultContract]; !exists {
				return nil, fmt.Errorf("shapeshifter: default_contract %q for %s %s does not match a contract id", ep.defaultContract, method, path)
			}
		}
		out.endpoints[route] = ep
	}

	return out, nil
}

type SanitizedSpec struct {
	Version      string              `json:"version"`
	Title        string              `json:"title,omitempty"`
	Description  string              `json:"description,omitempty"`
	Header       string              `json:"header"`
	Shapes       []string            `json:"shapes"`
	ShapeSchemas map[string]any      `json:"shape_schemas,omitempty"`
	Endpoints    []SanitizedEndpoint `json:"endpoints"`
}

type SanitizedEndpoint struct {
	Route           RouteKey            `json:"route"`
	Summary         string              `json:"summary,omitempty"`
	Description     string              `json:"description,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	DefaultContract string              `json:"default_contract,omitempty"`
	Limits          Limits              `json:"limits"`
	Contracts       []SanitizedContract `json:"contracts"`
}

type SanitizedContract struct {
	ID          string         `json:"id"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Deprecated  bool           `json:"deprecated,omitempty"`
	HasRequest  bool           `json:"has_request"`
	HasResponse bool           `json:"has_response"`
	Request     *SanitizedSide `json:"request,omitempty"`
	Response    *SanitizedSide `json:"response,omitempty"`
}

type SanitizedSide struct {
	Shape       string             `json:"shape,omitempty"`
	SourceShape string             `json:"source_shape,omitempty"`
	TargetShape string             `json:"target_shape,omitempty"`
	Description string             `json:"description,omitempty"`
	Examples    []SanitizedExample `json:"examples,omitempty"`
	Transform   SanitizedTransform `json:"transform"`
}

type SanitizedExample struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Body        any    `json:"body"`
}

type SanitizedTransform struct {
	Passthrough bool                  `json:"passthrough,omitempty"`
	Fields      []SanitizedField      `json:"fields,omitempty"`
	Validate    []SanitizedValidation `json:"validate,omitempty"`
	Coerce      []SanitizedCoerce     `json:"coerce,omitempty"`
	Handler     *SanitizedHandler     `json:"handler,omitempty"`
}

type SanitizedField struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Required bool   `json:"required"`
}

type SanitizedValidation struct {
	Field    string `json:"field"`
	Rule     string `json:"rule"`
	Error    string `json:"error"`
	Required bool   `json:"required"`
}

type SanitizedCoerce struct {
	Field string `json:"field"`
	Type  string `json:"type"`
}

type SanitizedHandler struct {
	Name        string `json:"name"`
	PreviewSafe bool   `json:"preview_safe"`
}

func (s *Spec) Sanitized() SanitizedSpec {
	if s == nil {
		return SanitizedSpec{Version: "1"}
	}
	out := SanitizedSpec{
		Version:      "1",
		Title:        s.title,
		Description:  s.description,
		Header:       s.header,
		Shapes:       append([]string(nil), s.shapes...),
		ShapeSchemas: cloneShapeSchemas(s.shapeSchemas),
	}
	routes := make([]RouteKey, 0, len(s.endpoints))
	for route := range s.endpoints {
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	for _, route := range routes {
		ep := s.endpoints[route]
		sanitizedEndpoint := SanitizedEndpoint{
			Route:           ep.route,
			Summary:         ep.summary,
			Description:     ep.description,
			Tags:            append([]string(nil), ep.tags...),
			DefaultContract: ep.defaultContract,
			Limits:          ep.limits,
		}
		for _, id := range sortedContractIDs(ep) {
			contract := ep.contracts[id]
			sanitizedContract := SanitizedContract{
				ID:          id,
				Summary:     contract.summary,
				Description: contract.description,
				Deprecated:  contract.deprecated,
				HasRequest:  contract.request != nil,
				HasResponse: contract.response != nil,
			}
			if contract.request != nil {
				side := sanitizeSide(contract.request, PhaseRequest)
				sanitizedContract.Request = &side
			}
			if contract.response != nil {
				side := sanitizeSide(contract.response, PhaseResponse)
				sanitizedContract.Response = &side
			}
			sanitizedEndpoint.Contracts = append(sanitizedEndpoint.Contracts, sanitizedContract)
		}
		out.Endpoints = append(out.Endpoints, sanitizedEndpoint)
	}
	return out
}

func sanitizeSide(side *sideSpec, phase Phase) SanitizedSide {
	out := SanitizedSide{
		Description: side.description,
		Examples:    sanitizeExamples(side.examples),
		Transform:   sanitizeTransform(side.transform),
	}
	if phase == PhaseResponse {
		out.Shape = side.targetShape
		out.SourceShape = side.sourceShape
	} else {
		out.Shape = side.sourceShape
		out.TargetShape = side.targetShape
	}
	return out
}

func sanitizeExamples(examples []compiledExample) []SanitizedExample {
	out := make([]SanitizedExample, 0, len(examples))
	for _, example := range examples {
		out = append(out, SanitizedExample{
			Name:        example.name,
			Description: example.description,
			Body:        cloneAny(example.body),
		})
	}
	return out
}

func sanitizeTransform(compiled compiledTransform) SanitizedTransform {
	out := SanitizedTransform{Passthrough: compiled.passthrough}
	for _, field := range compiled.fields {
		out.Fields = append(out.Fields, SanitizedField{
			From:     field.from.Source,
			To:       field.toString,
			Required: field.required,
		})
	}
	for _, validation := range compiled.validations {
		out.Validate = append(out.Validate, SanitizedValidation{
			Field:    validation.fieldString,
			Rule:     validation.rule.Source,
			Error:    validation.message,
			Required: validation.required,
		})
	}
	for _, coerce := range compiled.coercions {
		out.Coerce = append(out.Coerce, SanitizedCoerce{
			Field: coerce.fieldString,
			Type:  string(coerce.typ),
		})
	}
	if compiled.handler != nil {
		out.Handler = &SanitizedHandler{Name: compiled.handler.Name, PreviewSafe: compiled.handler.PreviewSafe}
	}
	return out
}

func compileShapes(rawShapes map[string]any) (map[string]schemaValidator, error) {
	if len(rawShapes) == 0 {
		return nil, fmt.Errorf("shapeshifter: at least one shape is required")
	}
	out := make(map[string]schemaValidator, len(rawShapes))
	for name, raw := range rawShapes {
		compiled, err := schemacompiler.Compile(name, raw)
		if err != nil {
			return nil, fmt.Errorf("shapeshifter: %w", err)
		}
		out[name] = compiled
	}
	return out, nil
}

func compileRequestSide(_ string, raw *rawspec.Side, shapes map[string]schemaValidator, rawShapes map[string]any, handlers HandlerSnapshot) (*sideSpec, error) {
	if strings.TrimSpace(raw.Shape) == "" {
		return nil, fmt.Errorf("shape is required")
	}
	source, err := lookupShape(shapes, raw.Shape)
	if err != nil {
		return nil, err
	}
	var target schemaValidator
	if strings.TrimSpace(raw.TargetShape) != "" {
		target, err = lookupShape(shapes, raw.TargetShape)
		if err != nil {
			return nil, err
		}
	}
	if raw.Transform.Passthrough && len(raw.Transform.Fields) > 0 {
		return nil, fmt.Errorf("transform cannot define both fields and passthrough")
	}
	if len(raw.Transform.Fields) == 0 && !raw.Transform.Passthrough {
		return nil, fmt.Errorf("request transform without fields requires passthrough: true")
	}
	if raw.Transform.Passthrough && !rootAdditionalPropertiesFalse(rawShapes[raw.Shape]) {
		return nil, fmt.Errorf("request passthrough requires root additionalProperties: false on shape %q", raw.Shape)
	}
	compiled, err := compileTransform(raw.Transform, handlers)
	if err != nil {
		return nil, err
	}
	examples, err := compileExamples(raw.Examples, source)
	if err != nil {
		return nil, err
	}
	return &sideSpec{
		sourceShape: raw.Shape,
		targetShape: raw.TargetShape,
		description: strings.TrimSpace(raw.Description),
		examples:    examples,
		source:      source,
		target:      target,
		transform:   compiled,
	}, nil
}

func compileResponseSide(_ string, raw *rawspec.Side, shapes map[string]schemaValidator, handlers HandlerSnapshot) (*sideSpec, error) {
	if strings.TrimSpace(raw.Shape) == "" {
		return nil, fmt.Errorf("shape is required")
	}
	target, err := lookupShape(shapes, raw.Shape)
	if err != nil {
		return nil, err
	}
	var source schemaValidator
	if strings.TrimSpace(raw.SourceShape) != "" {
		source, err = lookupShape(shapes, raw.SourceShape)
		if err != nil {
			return nil, err
		}
	}
	if raw.Transform.Passthrough {
		return nil, fmt.Errorf("response passthrough is not supported in MVP 1")
	}
	if len(raw.Transform.Fields) == 0 {
		return nil, fmt.Errorf("response transform must define explicit fields")
	}
	compiled, err := compileTransform(raw.Transform, handlers)
	if err != nil {
		return nil, err
	}
	examples, err := compileExamples(raw.Examples, target)
	if err != nil {
		return nil, err
	}
	return &sideSpec{
		sourceShape: raw.SourceShape,
		targetShape: raw.Shape,
		description: strings.TrimSpace(raw.Description),
		examples:    examples,
		source:      source,
		target:      target,
		transform:   compiled,
	}, nil
}

func compileExamples(rawExamples []rawspec.Example, shape schemaValidator) ([]compiledExample, error) {
	if len(rawExamples) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	out := make([]compiledExample, 0, len(rawExamples))
	for _, rawExample := range rawExamples {
		name := strings.TrimSpace(rawExample.Name)
		if name == "" {
			return nil, fmt.Errorf("example name is required")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate example %q", name)
		}
		seen[name] = struct{}{}
		if rawExample.Body == nil {
			return nil, fmt.Errorf("example %q body is required", name)
		}
		body := cloneAny(rawExample.Body)
		if shape != nil {
			if err := shape.Validate(body); err != nil {
				return nil, fmt.Errorf("example %q does not match shape: %w", name, err)
			}
		}
		out = append(out, compiledExample{
			name:        name,
			description: strings.TrimSpace(rawExample.Description),
			body:        body,
		})
	}
	return out, nil
}

func lookupShape(shapes map[string]schemaValidator, name string) (schemaValidator, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("shape name is required")
	}
	shape := shapes[name]
	if shape == nil {
		return nil, fmt.Errorf("shape %q is not defined", name)
	}
	return shape, nil
}

func compileTransform(raw rawspec.Transform, handlers HandlerSnapshot) (compiledTransform, error) {
	var out compiledTransform
	out.passthrough = raw.Passthrough

	for _, rawField := range raw.Fields {
		if strings.TrimSpace(rawField.From) == "" {
			return out, fmt.Errorf("field mapping from is required")
		}
		if strings.TrimSpace(rawField.To) == "" {
			return out, fmt.Errorf("field mapping to is required")
		}
		from, err := transform.CompileExpression(rawField.From)
		if err != nil {
			return out, fmt.Errorf("compile jq from %q: %w", rawField.From, err)
		}
		to, err := transform.ParseObjectPath(rawField.To)
		if err != nil {
			return out, fmt.Errorf("invalid target path %q: %w", rawField.To, err)
		}
		out.fields = append(out.fields, fieldMapping{
			from:     from,
			to:       to,
			toString: rawField.To,
			required: rawspec.RequiredDefaultTrue(rawField.Required),
		})
	}

	for _, rawValidation := range raw.Validate {
		if strings.TrimSpace(rawValidation.Field) == "" {
			return out, fmt.Errorf("validation field is required")
		}
		if strings.TrimSpace(rawValidation.Rule) == "" {
			return out, fmt.Errorf("validation rule is required")
		}
		field, err := transform.CompileExpression(rawValidation.Field)
		if err != nil {
			return out, fmt.Errorf("compile validation field %q: %w", rawValidation.Field, err)
		}
		rule, err := transform.CompileExpression(rawValidation.Rule)
		if err != nil {
			return out, fmt.Errorf("compile validation rule %q: %w", rawValidation.Rule, err)
		}
		message := rawValidation.Error
		if message == "" {
			message = "validation rule failed"
		}
		out.validations = append(out.validations, validationRule{
			field:       field,
			fieldString: rawValidation.Field,
			rule:        rule,
			message:     message,
			required:    rawspec.RequiredDefaultTrue(rawValidation.Required),
		})
	}

	for _, rawCoerce := range raw.Coerce {
		path, err := transform.ParseObjectPath(rawCoerce.Field)
		if err != nil {
			return out, fmt.Errorf("invalid coerce path %q: %w", rawCoerce.Field, err)
		}
		typ, err := transform.ParseCoerceType(rawCoerce.Type)
		if err != nil {
			return out, err
		}
		out.coercions = append(out.coercions, coerceRule{field: path, fieldString: rawCoerce.Field, typ: typ})
	}

	if strings.TrimSpace(raw.Handler) != "" {
		handler, ok := handlers.lookup(raw.Handler)
		if !ok {
			return out, fmt.Errorf("handler %q is not registered", raw.Handler)
		}
		out.handler = &handler
	}

	return out, nil
}

func rootAdditionalPropertiesFalse(shape any) bool {
	m, ok := shape.(map[string]any)
	if !ok {
		return false
	}
	v, ok := m["additionalProperties"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && !b
}

func compileLimits(raw *rawspec.Limits, topLevel bool) (Limits, error) {
	limits := Limits{RequestBodyBytes: DefaultRequestBodyBytes, ResponseBodyBytes: DefaultResponseBodyBytes}
	if raw == nil {
		return limits, nil
	}
	if raw.RequestBodyBytes == nil {
		if !topLevel {
			limits.RequestBodyBytes = DefaultRequestBodyBytes
		}
	} else if *raw.RequestBodyBytes <= 0 {
		return limits, fmt.Errorf("request_body_bytes must be positive")
	} else {
		limits.RequestBodyBytes = *raw.RequestBodyBytes
	}
	if raw.ResponseBodyBytes == nil {
		if !topLevel {
			limits.ResponseBodyBytes = DefaultResponseBodyBytes
		}
	} else if *raw.ResponseBodyBytes <= 0 {
		return limits, fmt.Errorf("response_body_bytes must be positive")
	} else {
		limits.ResponseBodyBytes = *raw.ResponseBodyBytes
	}
	return limits, nil
}

func overrideLimits(base Limits, raw *rawspec.Limits) (Limits, error) {
	limits := base
	if raw == nil {
		return limits, nil
	}
	if raw.RequestBodyBytes != nil {
		if *raw.RequestBodyBytes <= 0 {
			return limits, fmt.Errorf("request_body_bytes must be positive")
		}
		limits.RequestBodyBytes = *raw.RequestBodyBytes
	}
	if raw.ResponseBodyBytes != nil {
		if *raw.ResponseBodyBytes <= 0 {
			return limits, fmt.Errorf("response_body_bytes must be positive")
		}
		limits.ResponseBodyBytes = *raw.ResponseBodyBytes
	}
	return limits, nil
}

func sortedContractIDs(ep *endpointSpec) []string {
	if ep == nil {
		return nil
	}
	out := append([]string(nil), ep.order...)
	if len(out) == 0 {
		for id := range ep.contracts {
			out = append(out, id)
		}
		sort.Strings(out)
	}
	return out
}

func sortedShapeNames(shapes map[string]any) []string {
	out := make([]string, 0, len(shapes))
	for name := range shapes {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func cloneShapeSchemas(shapes map[string]any) map[string]any {
	if len(shapes) == 0 {
		return nil
	}
	data, err := json.Marshal(shapes)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func cloneAny(value any) any {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return value
	}
	return out
}
