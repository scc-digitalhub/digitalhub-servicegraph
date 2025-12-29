// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"context"
	"testing"
	"text/template"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

func TestConvert(t *testing.T) {
	type Src struct {
		A string
		B int
	}
	type Dst struct {
		A string
		B int
	}
	src := Src{"foo", 42}
	var dst Dst
	if err := util.Convert(src, &dst); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	if dst.A != "foo" || dst.B != 42 {
		t.Fatalf("unexpected converted value: %+v", dst)
	}
}

func TestValidateAndNormalizeJSONPath(t *testing.T) {
	valid := "$[?(@.foo)]"
	if err := util.ValidateJSONPath(valid); err != nil {
		t.Fatalf("valid path considered invalid: %v", err)
	}
	input := "@.foo"
	want := "$[?@.foo]"
	if got := util.NormalizeJSONPath(input); got != want {
		t.Fatalf("NormalizeJSONPath: got %s, want %s", got, want)
	}
}

func TestBuildAndEvaluateJSONPath(t *testing.T) {
	ctx := context.Background()

	expr, err := util.BuildJSONPathExpression("$.foo")
	if err != nil {
		t.Fatalf("BuildJSONPathExpression error: %v", err)
	}
	res, err := util.EvaluateJSONPathOnExpr(`{"foo": 123}`, expr)
	if err != nil {
		t.Fatalf("EvaluateJSONPathOnExpr error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("unexpected result length: %+v", res)
	}
	v := res[0].(int64)
	if v != 123 {
		t.Fatalf("unexpected numeric value: %v", v)
	}

	// test with streams.Event
	ev := streams.NewEventFrom(ctx, `{"foo": 456}`)
	res, err = util.EvaluateJSONPathOnExpr(ev, expr)
	if err != nil {
		t.Fatalf("EvaluateJSONPathOnExpr(event) error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("unexpected result length for event: %+v", res)
	}
	v = res[0].(int64)
	if v != 456 {
		t.Fatalf("unexpected numeric value for event: %v", v)
	}
}

// Negative and non-JSON scenarios
func TestEvaluateJSONPathOnExpr_NonJSONData(t *testing.T) {
	expr, err := util.BuildJSONPathExpression("$.foo")
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	// pass plain text which is not JSON
	_, err = util.EvaluateJSONPathOnExpr("not-json", expr)
	if err == nil {
		t.Fatalf("expected error when parsing non-json string")
	}
}

func TestEvaluateJSONPathOnExpr_InvalidJSONBytes(t *testing.T) {
	expr, _ := util.BuildJSONPathExpression("$.foo")
	// invalid json bytes
	_, err := util.EvaluateJSONPathOnExpr([]byte("{foo:}"), expr)
	if err == nil {
		t.Fatalf("expected parse error for invalid json bytes")
	}
}

func TestEvaluateJSONPathOnExpr_EventWithNonJSONBody(t *testing.T) {
	expr, _ := util.BuildJSONPathExpression("$.foo")
	ctx := context.Background()
	ev := streams.NewEventFrom(ctx, "plain text not json")
	_, err := util.EvaluateJSONPathOnExpr(ev, expr)
	if err == nil {
		t.Fatalf("expected error when event body is not JSON")
	}
}

func TestBuildJSONPathExpression_InvalidPath(t *testing.T) {
	_, err := util.BuildJSONPathExpression("[invalid")
	if err == nil {
		t.Fatalf("expected error for invalid jsonpath")
	}
}

func TestEvaluateJSONPath_UnsupportedType(t *testing.T) {
	// pass an unsupported type (int)
	_, err := util.EvaluateJSONPath(12345, "$.foo")
	if err == nil {
		t.Fatalf("expected error for unsupported jsonData type")
	}
}

func TestEvaluateJSONPath_InvalidPathViaEvaluate(t *testing.T) {
	_, err := util.EvaluateJSONPath(`{"foo":1}`, "[bad")
	if err == nil {
		t.Fatalf("expected error when EvaluateJSONPath given invalid path")
	}
}

func TestConvert_ErrorPaths(t *testing.T) {
	// marshal error: pass a channel which json can't marshal
	var dst map[string]interface{}
	err := util.Convert(make(chan int), &dst)
	if err == nil {
		t.Fatalf("expected marshal error for channel input")
	}

	// unmarshal error: pass non-pointer out
	src := struct{ A string }{A: "x"}
	var notPtr interface{}
	err = util.Convert(src, notPtr)
	if err == nil {
		t.Fatalf("expected unmarshal error when out is not a pointer")
	}
}

func TestValidateJSONPath_InvalidAndNormalizeAlreadyPrefixed(t *testing.T) {
	if err := util.ValidateJSONPath("[bad"); err == nil {
		t.Fatalf("expected ValidateJSONPath to fail for invalid path")
	}

	in := "$[?@.bar]"
	if got := util.NormalizeJSONPath(in); got != in {
		t.Fatalf("expected NormalizeJSONPath to return input when already prefixed")
	}
}

func TestEvaluateJSONPath_EmptyPath(t *testing.T) {
	// when jsonPath is empty BuildJSONPathExpression should default to $
	res, err := util.EvaluateJSONPath(`{"foo":true}`, "")
	if err != nil {
		t.Fatalf("unexpected error for empty path: %v", err)
	}
	if len(res) == 0 {
		t.Fatalf("expected non-empty result for root path")
	}
}

func TestConvertBody_WithTemplate(t *testing.T) {
	// Test with template on string data
	templateStr := `Hello {{.}}!`
	tmpl, err := template.New("test").Parse(templateStr)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	result, err := util.ConvertBody("world", tmpl)
	if err != nil {
		t.Fatalf("ConvertBody failed: %v", err)
	}

	expected := "Hello world!"
	if string(result) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result))
	}
}

func TestConvertBody_WithoutTemplate(t *testing.T) {
	// Test without template - should return the input as []byte
	input := []byte("plain text")
	result, err := util.ConvertBody(input, nil)
	if err != nil {
		t.Fatalf("ConvertBody failed: %v", err)
	}

	expected := "plain text"
	if string(result) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result))
	}
}

func TestConvertBody_DifferentTypes(t *testing.T) {
	// Test with []byte (the expected input type when template is nil)
	input := []byte("bytes body")
	result, err := util.ConvertBody(input, nil)
	if err != nil {
		t.Fatalf("ConvertBody with []byte failed: %v", err)
	}
	if string(result) != "bytes body" {
		t.Fatalf("expected 'bytes body', got %q", string(result))
	}
}

func TestConvertBody_TemplateError(t *testing.T) {
	// Template with invalid syntax
	templateStr := `Hello {{.invalid` // Missing closing braces
	_, err := template.New("test").Parse(templateStr)
	if err == nil {
		t.Fatalf("expected template parsing error")
	}
	// Since parsing fails, we can't test execution
}
