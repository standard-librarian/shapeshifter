package transform

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNormalizeNumbers(t *testing.T) {
	got, err := NormalizeNumbers(map[string]any{
		"i": json.Number("42"),
		"f": json.Number("1.5"),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if _, ok := m["i"].(int64); !ok {
		t.Fatalf("i = %T", m["i"])
	}
	if _, ok := m["f"].(float64); !ok {
		t.Fatalf("f = %T", m["f"])
	}
	if _, err := NormalizeNumbers(json.Number("9223372036854775808")); err == nil {
		t.Fatal("expected int64 overflow error")
	}
}

func FuzzNormalizeNumbers(f *testing.F) {
	for _, seed := range []string{`0`, `1`, `-1`, `1.5`, `1e2`, `9223372036854775808`, `{"n":1}`, `[1,2.5]`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		dec := json.NewDecoder(bytes.NewReader([]byte(input)))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			return
		}
		_, _ = NormalizeNumbers(value)
	})
}
