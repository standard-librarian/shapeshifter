package transform

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func NormalizeNumbers(v any) (any, error) {
	switch x := v.(type) {
	case json.Number:
		return normalizeNumber(x)
	case map[string]any:
		for k, child := range x {
			normalized, err := NormalizeNumbers(child)
			if err != nil {
				return nil, err
			}
			x[k] = normalized
		}
		return x, nil
	case []any:
		for i, child := range x {
			normalized, err := NormalizeNumbers(child)
			if err != nil {
				return nil, err
			}
			x[i] = normalized
		}
		return x, nil
	default:
		return v, nil
	}
}

func normalizeNumber(n json.Number) (any, error) {
	s := n.String()
	if isIntegerLexical(s) {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("integer %q is outside int64 or invalid: %w", s, err)
		}
		return i, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("number %q is invalid: %w", s, err)
	}
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, fmt.Errorf("number %q is not finite", s)
	}
	return f, nil
}

func isIntegerLexical(s string) bool {
	return !strings.ContainsAny(s, ".eE")
}
