package schema

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func Compile(name string, raw any) (*jsonschema.Schema, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal schema %q: %w", name, err)
	}
	c := jsonschema.NewCompiler()
	resource := "shapeshifter://shape/" + name
	if err := c.AddResource(resource, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("add schema %q: %w", name, err)
	}
	compiled, err := c.Compile(resource)
	if err != nil {
		return nil, fmt.Errorf("compile schema %q: %w", name, err)
	}
	return compiled, nil
}

func FlattenValidationError(err error) []struct {
	Field   string
	Message string
} {
	if err == nil {
		return nil
	}
	var out []struct {
		Field   string
		Message string
	}
	if ve, ok := err.(*jsonschema.ValidationError); ok {
		flatten(ve, &out)
	}
	if len(out) == 0 {
		out = append(out, struct {
			Field   string
			Message string
		}{Field: ".", Message: "schema validation failed"})
	}
	return out
}

func flatten(ve *jsonschema.ValidationError, out *[]struct {
	Field   string
	Message string
}) {
	if ve == nil {
		return
	}
	if len(ve.Causes) == 0 {
		*out = append(*out, struct {
			Field   string
			Message string
		}{Field: instanceLocationToPath(ve.InstanceLocation), Message: ve.Message})
		return
	}
	for _, cause := range ve.Causes {
		flatten(cause, out)
	}
}

func instanceLocationToPath(loc string) string {
	if loc == "" || loc == "/" {
		return "."
	}
	if loc[0] != '/' {
		return loc
	}
	out := "."
	for i := 1; i < len(loc); i++ {
		if loc[i] == '/' {
			out += "."
			continue
		}
		if loc[i] == '~' && i+1 < len(loc) {
			switch loc[i+1] {
			case '0':
				out += "~"
				i++
				continue
			case '1':
				out += "/"
				i++
				continue
			}
		}
		out += string(loc[i])
	}
	return out
}
