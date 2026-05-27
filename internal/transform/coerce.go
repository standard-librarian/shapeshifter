package transform

import (
	"fmt"
	"math"
	"strconv"
)

type CoerceType string

const (
	CoerceInteger CoerceType = "integer"
	CoerceNumber  CoerceType = "number"
	CoerceString  CoerceType = "string"
	CoerceBoolean CoerceType = "boolean"
)

func ParseCoerceType(s string) (CoerceType, error) {
	switch CoerceType(s) {
	case CoerceInteger, CoerceNumber, CoerceString, CoerceBoolean:
		return CoerceType(s), nil
	default:
		return "", fmt.Errorf("unsupported coercion type %q", s)
	}
}

func ApplyCoerce(target map[string]any, path []string, typ CoerceType) error {
	value, ok := GetPath(target, path)
	if !ok || value == nil {
		return fmt.Errorf("missing value at %s", PathString(path))
	}
	coerced, err := CoerceValue(value, typ)
	if err != nil {
		return err
	}
	return SetPath(target, path, coerced)
}

func CoerceValue(value any, typ CoerceType) (any, error) {
	switch typ {
	case CoerceInteger:
		return coerceInteger(value)
	case CoerceNumber:
		return coerceNumber(value)
	case CoerceString:
		return coerceString(value)
	case CoerceBoolean:
		return coerceBoolean(value)
	default:
		return nil, fmt.Errorf("unsupported coercion type %q", typ)
	}
}

func coerceInteger(value any) (int64, error) {
	switch x := value.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case float64:
		if math.Trunc(x) != x {
			return 0, fmt.Errorf("number is not an integer")
		}
		if x < math.MinInt64 || x > math.MaxInt64 {
			return 0, fmt.Errorf("integer is outside int64")
		}
		return int64(x), nil
	case string:
		i, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, fmt.Errorf("cannot coerce %T to integer", value)
	}
}

func coerceNumber(value any) (any, error) {
	switch x := value.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return nil, fmt.Errorf("number is not finite")
		}
		return x, nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return nil, err
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("number is not finite")
		}
		return f, nil
	default:
		return nil, fmt.Errorf("cannot coerce %T to number", value)
	}
}

func coerceString(value any) (string, error) {
	switch x := value.(type) {
	case string:
		return x, nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case int:
		return strconv.Itoa(x), nil
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return "", fmt.Errorf("number is not finite")
		}
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(x), nil
	default:
		return "", fmt.Errorf("cannot coerce %T to string", value)
	}
}

func coerceBoolean(value any) (bool, error) {
	switch x := value.(type) {
	case bool:
		return x, nil
	case string:
		b, err := strconv.ParseBool(x)
		if err != nil {
			return false, err
		}
		return b, nil
	default:
		return false, fmt.Errorf("cannot coerce %T to boolean", value)
	}
}
