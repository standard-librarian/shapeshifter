package conformance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/standard-librarian/shapeshifter"
)

type conformanceCase struct {
	Name     string                `json:"name"`
	Spec     string                `json:"spec"`
	Route    shapeshifter.RouteKey `json:"route"`
	Contract string                `json:"contract"`
	Phase    shapeshifter.Phase    `json:"phase"`
	Mode     string                `json:"mode"`
	Input    json.RawMessage       `json:"input"`
	InputRaw string                `json:"input_raw"`
	Expect   expectation           `json:"expect"`
}

type expectation struct {
	Status          string                               `json:"status"`
	Contract        string                               `json:"contract"`
	Available       []string                             `json:"available"`
	HasRequest      bool                                 `json:"has_request"`
	HasResponse     bool                                 `json:"has_response"`
	Payload         json.RawMessage                      `json:"payload"`
	SkippedHandlers []string                             `json:"skipped_handlers"`
	Stage           shapeshifter.Stage                   `json:"stage"`
	ErrorCode       shapeshifter.ErrorCode               `json:"error_code"`
	SelectionReason shapeshifter.ContractSelectionReason `json:"selection_reason"`
}

func TestConformanceCases(t *testing.T) {
	cases := loadCases(t)
	engines := map[string]*shapeshifter.Engine{}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			engine := engines[tc.Spec]
			if engine == nil {
				engine = loadEngine(t, tc.Spec)
				engines[tc.Spec] = engine
			}

			if tc.Phase == "selection" {
				assertSelection(t, engine, tc)
				return
			}

			selection, err := engine.ResolveContract(tc.Route, tc.Contract)
			if err != nil {
				t.Fatalf("ResolveContract: %v", err)
			}

			mode := shapeshifter.ModeRuntime
			if tc.Mode == "preview" {
				mode = shapeshifter.ModePreview
			}

			input := caseInput(t, tc)
			if len(input) == 0 {
				t.Fatal("input is required for process cases")
			}

			var result shapeshifter.ProcessResult
			switch tc.Phase {
			case shapeshifter.PhaseRequest:
				result, err = engine.ProcessRequest(context.Background(), selection, input, mode)
			case shapeshifter.PhaseResponse:
				result, err = engine.ProcessResponse(context.Background(), selection, input, mode)
			default:
				t.Fatalf("unknown phase %q", tc.Phase)
			}

			switch tc.Expect.Status {
			case "ok":
				if err != nil {
					t.Fatalf("process error: %v", err)
				}
				assertJSONEqual(t, result.Payload, tc.Expect.Payload)
				if !reflect.DeepEqual(result.SkippedHandlers, tc.Expect.SkippedHandlers) {
					t.Fatalf("skipped handlers = %#v, want %#v", result.SkippedHandlers, tc.Expect.SkippedHandlers)
				}
			case "process_error":
				assertProcessError(t, err, tc.Expect)
			default:
				t.Fatalf("unknown expected status %q", tc.Expect.Status)
			}
		})
	}
}

func caseInput(t *testing.T, tc conformanceCase) []byte {
	t.Helper()
	if tc.InputRaw != "" {
		return []byte(tc.InputRaw)
	}
	return bytes.TrimSpace(tc.Input)
}

func loadCases(t *testing.T) []conformanceCase {
	t.Helper()
	entries, err := os.ReadDir("cases")
	if err != nil {
		t.Fatal(err)
	}
	var out []conformanceCase
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("cases", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var cases []conformanceCase
		if err := json.Unmarshal(data, &cases); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		out = append(out, cases...)
	}
	if len(out) == 0 {
		t.Fatal("no conformance cases found")
	}
	return out
}

func loadEngine(t *testing.T, specName string) *shapeshifter.Engine {
	t.Helper()
	registry := shapeshifter.NewRegistry()
	mustRegister(t, registry, "safeSuffix", func(input map[string]any) (map[string]any, error) {
		input["name"] = input["name"].(string) + "!"
		return input, nil
	}, shapeshifter.HandlerOptions{PreviewSafe: true})
	mustRegister(t, registry, "unsafeName", func(input map[string]any) (map[string]any, error) {
		input["name"] = "runtime"
		return input, nil
	})
	mustRegister(t, registry, "handlerValidation", func(input map[string]any) (map[string]any, error) {
		return nil, &shapeshifter.HandlerValidationError{Errors: []shapeshifter.ValidationError{{
			Field:   ".name",
			Message: "invalid handler value",
			Code:    shapeshifter.CodeHandlerValidationFailed,
		}}}
	})
	mustRegister(t, registry, "handlerFail", func(input map[string]any) (map[string]any, error) {
		return nil, errors.New("handler system failure")
	})
	mustRegister(t, registry, "describeNumbers", func(input map[string]any) (map[string]any, error) {
		return map[string]any{
			"count_type":  typeName(input["count"]),
			"amount_type": typeName(input["amount"]),
		}, nil
	}, shapeshifter.HandlerOptions{PreviewSafe: true})

	spec, err := shapeshifter.LoadSpecFile(filepath.Join("specs", specName), registry.Snapshot())
	if err != nil {
		t.Fatalf("LoadSpecFile(%s): %v", specName, err)
	}
	engine, err := shapeshifter.NewEngine(spec)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func typeName(value any) string {
	switch value.(type) {
	case int64:
		return "int64"
	case float64:
		return "float64"
	default:
		return "other"
	}
}

func mustRegister(t *testing.T, registry *shapeshifter.Registry, name string, fn shapeshifter.HandlerFunc, opts ...shapeshifter.HandlerOptions) {
	t.Helper()
	if err := registry.Register(name, fn, opts...); err != nil {
		t.Fatal(err)
	}
}

func assertSelection(t *testing.T, engine *shapeshifter.Engine, tc conformanceCase) {
	t.Helper()
	selection, err := engine.ResolveContract(tc.Route, tc.Contract)
	switch tc.Expect.Status {
	case "ok":
		if err != nil {
			t.Fatalf("ResolveContract: %v", err)
		}
		if selection.ContractID != tc.Expect.Contract {
			t.Fatalf("contract = %q, want %q", selection.ContractID, tc.Expect.Contract)
		}
		if !reflect.DeepEqual(selection.Available, tc.Expect.Available) {
			t.Fatalf("available = %#v, want %#v", selection.Available, tc.Expect.Available)
		}
		if selection.HasRequest != tc.Expect.HasRequest || selection.HasResponse != tc.Expect.HasResponse {
			t.Fatalf("selection sides = request:%v response:%v", selection.HasRequest, selection.HasResponse)
		}
	case "selection_error":
		var selectionErr *shapeshifter.ContractSelectionError
		if !errors.As(err, &selectionErr) {
			t.Fatalf("selection error = %v", err)
		}
		if selectionErr.Reason != tc.Expect.SelectionReason {
			t.Fatalf("selection reason = %q, want %q", selectionErr.Reason, tc.Expect.SelectionReason)
		}
	default:
		t.Fatalf("unknown expected status %q", tc.Expect.Status)
	}
}

func assertProcessError(t *testing.T, err error, expect expectation) {
	t.Helper()
	var ssErr *shapeshifter.ShapeShifterError
	if !errors.As(err, &ssErr) {
		t.Fatalf("process error = %v", err)
	}
	if ssErr.Stage != expect.Stage {
		t.Fatalf("stage = %q, want %q", ssErr.Stage, expect.Stage)
	}
	for _, detail := range ssErr.Errors {
		if detail.Code == expect.ErrorCode {
			return
		}
	}
	t.Fatalf("error details = %#v, want code %q", ssErr.Errors, expect.ErrorCode)
}

func assertJSONEqual(t *testing.T, got, want json.RawMessage) {
	t.Helper()
	var gotValue any
	var wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("want is not JSON: %v\n%s", err, want)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("payload = %#v, want %#v", gotValue, wantValue)
	}
}
