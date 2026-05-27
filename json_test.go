package shapeshifter

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func assertJSON(t *testing.T, actual []byte, expected string) {
	t.Helper()
	var got any
	if err := json.Unmarshal(actual, &got); err != nil {
		t.Fatalf("actual is not JSON: %s: %v", actual, err)
	}
	var want any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatalf("expected is not JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("json = %s, want %s", actual, expected)
	}
}

func bytesReader(s string) *bytes.Reader {
	return bytes.NewReader([]byte(s))
}
