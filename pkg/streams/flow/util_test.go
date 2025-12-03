package flow_test

import (
	"testing"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
)

func TestConcatTemplate(t *testing.T) {
	// Test with multiple JSON values
	values := []any{
		`{"name": "John", "age": 30}`,
		[]byte(`{"city": "New York", "country": "USA"}`),
	}

	templateStr := `Hello {{index . 0 "name"}}, you are {{ index . 0 "age"}} years old and live in {{index . 1 "city"}}, {{index . 1 "country"}}.`

	result, err := flow.ConcatTemplate(values, templateStr)
	if err != nil {
		t.Fatalf("ConcatTemplate failed: %v", err)
	}

	expected := "Hello John, you are 30 years old and live in New York, USA."
	if string(result.GetBody()) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result.GetBody()))
	}

	// Test with streams.Event
	event, _ := streams.NewGenericEvent([]byte(`{"job": "developer"}`), "", "", nil, nil, 200)
	valuesWithEvent := []any{
		`{"name": "Jane"}`,
		event,
	}

	templateStr2 := `{{index . 0 "name"}} is a {{index . 1 "job"}}.`
	result2, err := flow.ConcatTemplate(valuesWithEvent, templateStr2)
	if err != nil {
		t.Fatalf("ConcatTemplate with event failed: %v", err)
	}

	expected2 := "Jane is a developer."
	if string(result2.GetBody()) != expected2 {
		t.Fatalf("expected %q, got %q", expected2, string(result2.GetBody()))
	}
}
