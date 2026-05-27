package transform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/itchyny/gojq"
)

var (
	ErrNoJQOutput       = errors.New("jq produced no outputs")
	ErrMultipleJQOutput = errors.New("jq produced multiple outputs")
	ErrJQValueTooLarge  = errors.New("jq output value too large")
)

type JQLimits struct {
	MaxOutputsInspected int
	MaxValueBytes       int
	MaxDuration         time.Duration
}

func DefaultJQLimits() JQLimits {
	return JQLimits{
		MaxOutputsInspected: 2,
		MaxValueBytes:       65536,
	}
}

type Expression struct {
	Source string
	code   *gojq.Code
}

func CompileExpression(source string) (*Expression, error) {
	q, err := gojq.Parse(source)
	if err != nil {
		return nil, err
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, err
	}
	return &Expression{Source: source, code: code}, nil
}

func (e *Expression) EvalOne(ctx context.Context, input any, limits JQLimits) (any, error) {
	if e == nil {
		return nil, fmt.Errorf("nil jq expression")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limits.MaxOutputsInspected <= 0 {
		limits.MaxOutputsInspected = 2
	}
	if limits.MaxOutputsInspected < 2 {
		limits.MaxOutputsInspected = 2
	}
	if limits.MaxValueBytes <= 0 {
		limits.MaxValueBytes = 65536
	}
	if limits.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, limits.MaxDuration)
		defer cancel()
	}

	iter := e.code.RunWithContext(ctx, input)
	var out any
	count := 0
	for count < limits.MaxOutputsInspected {
		value, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := value.(error); ok {
			return nil, err
		}
		count++
		if count == 1 {
			out = value
			if err := enforceValueSize(value, limits.MaxValueBytes); err != nil {
				return nil, err
			}
			continue
		}
		return nil, ErrMultipleJQOutput
	}
	if count == 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNoJQOutput
	}
	return out, nil
}

func enforceValueSize(value any, max int) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > max {
		return ErrJQValueTooLarge
	}
	return nil
}
