package spec

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Version   string         `yaml:"version"`
	Header    string         `yaml:"header"`
	Limits    *Limits        `yaml:"limits"`
	Shapes    map[string]any `yaml:"shapes"`
	Endpoints []Endpoint     `yaml:"endpoints"`
}

type Limits struct {
	RequestBodyBytes  *int64 `yaml:"request_body_bytes"`
	ResponseBodyBytes *int64 `yaml:"response_body_bytes"`
}

type Endpoint struct {
	Path            string     `yaml:"path"`
	Method          string     `yaml:"method"`
	DefaultContract string     `yaml:"default_contract"`
	Limits          *Limits    `yaml:"limits"`
	Contracts       []Contract `yaml:"contracts"`
}

type Contract struct {
	ID       string `yaml:"id"`
	Request  *Side  `yaml:"request"`
	Response *Side  `yaml:"response"`
}

type Side struct {
	Shape       string    `yaml:"shape"`
	TargetShape string    `yaml:"target_shape"`
	SourceShape string    `yaml:"source_shape"`
	Transform   Transform `yaml:"transform"`
}

type Transform struct {
	Fields      []Field      `yaml:"fields"`
	Validate    []Validation `yaml:"validate"`
	Coerce      []Coerce     `yaml:"coerce"`
	Handler     string       `yaml:"handler"`
	Passthrough bool         `yaml:"passthrough"`
}

type Field struct {
	From     string `yaml:"from"`
	To       string `yaml:"to"`
	Required *bool  `yaml:"required"`
}

type Validation struct {
	Field    string `yaml:"field"`
	Rule     string `yaml:"rule"`
	Error    string `yaml:"error"`
	Required *bool  `yaml:"required"`
}

type Coerce struct {
	Field string `yaml:"field"`
	Type  string `yaml:"type"`
}

func Load(r io.Reader) (*Spec, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("shapeshifter: read spec: %w", err)
	}
	var raw Spec
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("shapeshifter: parse spec: %w", err)
	}
	if raw.Shapes == nil {
		raw.Shapes = map[string]any{}
	}
	for name, shape := range raw.Shapes {
		normalized, err := normalizeYAML(shape)
		if err != nil {
			return nil, fmt.Errorf("shapeshifter: shape %q: %w", name, err)
		}
		raw.Shapes[name] = normalized
	}
	return &raw, nil
}

func normalizeYAML(v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			normalized, err := normalizeYAML(child)
			if err != nil {
				return nil, err
			}
			out[k] = normalized
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key %v", k)
			}
			normalized, err := normalizeYAML(child)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			normalized, err := normalizeYAML(child)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	default:
		return v, nil
	}
}
