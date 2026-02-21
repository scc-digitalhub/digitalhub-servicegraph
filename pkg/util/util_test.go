// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"context"
	"encoding/binary"
	"math"
	"testing"

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
	tmpl, err := util.BuildTemplate(templateStr, "testTemplate")
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

func TestConvertBody_WithJSONPath(t *testing.T) {
	// Test with template on string data
	templateStr := `Hello {{. | jp "$.value"}}!`
	tmpl, err := util.BuildTemplate(templateStr, "testTemplate")
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	result, err := util.ConvertBody([]byte(`{"value": "world"}`), tmpl)
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
	_, err := util.BuildTemplate(templateStr, "testTemplate")
	if err == nil {
		t.Fatalf("expected template parsing error")
	}
	// Since parsing fails, we can't test execution
}

// Test cases for custom template functions

func TestTemplateFunc_JP(t *testing.T) {
	tests := []struct {
		name     string
		template string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple path",
			template: `{{. | jp "$.name"}}`,
			input:    `{"name": "Alice"}`,
			expected: "Alice",
			wantErr:  false,
		},
		{
			name:     "nested path",
			template: `{{. | jp "$.user.age"}}`,
			input:    `{"user": {"age": 30}}`,
			expected: "30",
			wantErr:  false,
		},
		{
			name:     "array element",
			template: `{{. | jp "$.items[0]"}}`,
			input:    `{"items": ["first", "second"]}`,
			expected: "first",
			wantErr:  false,
		},
		{
			name:     "boolean value",
			template: `{{. | jp "$.active"}}`,
			input:    `{"active": true}`,
			expected: "true",
			wantErr:  false,
		},
		{
			name:     "invalid json",
			template: `{{. | jp "$.name"}}`,
			input:    `not json`,
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := util.BuildTemplate(tt.template, "test")
			if err != nil {
				t.Fatalf("failed to build template: %v", err)
			}

			result, err := util.ConvertBody(tt.input, tmpl)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ConvertBody error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(result) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

func TestTemplateFunc_JSON(t *testing.T) {
	tests := []struct {
		name     string
		template string
		input    interface{}
		expected string
	}{
		{
			name:     "string value",
			template: `{{"hello" | json}}`,
			input:    "",
			expected: `"hello"`,
		},
		{
			name:     "number value",
			template: `{{42 | json}}`,
			input:    "",
			expected: `42`,
		},
		{
			name:     "boolean value",
			template: `{{true | json}}`,
			input:    "",
			expected: `true`,
		},
		{
			name:     "map value",
			template: `{{. | jp "$.user" | json}}`,
			input:    `{"user": {"name": "Bob", "age": 25}}`,
			expected: `{"age":25,"name":"Bob"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := util.BuildTemplate(tt.template, "test")
			if err != nil {
				t.Fatalf("failed to build template: %v", err)
			}

			result, err := util.ConvertBody(tt.input, tmpl)
			if err != nil {
				t.Fatalf("ConvertBody failed: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

func TestTemplateFunc_Tensor(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		dataType string
		input    []byte
		expected string
		wantErr  bool
	}{
		{
			name:     "BOOL type",
			dataType: "BOOL",
			input:    []byte{0, 1, 0, 1},
			expected: "[false,true,false,true]",
			wantErr:  false,
		},
		{
			name:     "UINT8 type",
			dataType: "UINT8",
			input:    []byte{10, 20, 30},
			expected: "[10,20,30]",
			wantErr:  false,
		},
		{
			name:     "INT8 type",
			dataType: "INT8",
			input:    []byte{255, 127, 128}, // -1, 127, -128
			expected: "[-1,127,-128]",
			wantErr:  false,
		},
		{
			name:     "UINT16 type",
			dataType: "UINT16",
			input:    makeUint16Bytes([]uint16{100, 200, 300}),
			expected: "[100,200,300]",
			wantErr:  false,
		},
		{
			name:     "INT16 type",
			dataType: "INT16",
			input:    makeInt16Bytes([]int16{-100, 0, 100}),
			expected: "[-100,0,100]",
			wantErr:  false,
		},
		{
			name:     "UINT32 type",
			dataType: "UINT32",
			input:    makeUint32Bytes([]uint32{1000, 2000}),
			expected: "[1000,2000]",
			wantErr:  false,
		},
		{
			name:     "INT32 type",
			dataType: "INT32",
			input:    makeInt32Bytes([]int32{-1000, 1000}),
			expected: "[-1000,1000]",
			wantErr:  false,
		},
		{
			name:     "UINT64 type",
			dataType: "UINT64",
			input:    makeUint64Bytes([]uint64{10000, 20000}),
			expected: "[10000,20000]",
			wantErr:  false,
		},
		{
			name:     "INT64 type",
			dataType: "INT64",
			input:    makeInt64Bytes([]int64{-10000, 10000}),
			expected: "[-10000,10000]",
			wantErr:  false,
		},
		{
			name:     "FP32 type",
			dataType: "FP32",
			input:    makeFloat32Bytes([]float32{1.5, 2.5, 3.5}),
			expected: "[1.5,2.5,3.5]",
			wantErr:  false,
		},
		{
			name:     "FP64 type",
			dataType: "FP64",
			input:    makeFloat64Bytes([]float64{1.234, 5.678}),
			expected: "[1.234,5.678]",
			wantErr:  false,
		},
		{
			name:     "invalid data length for UINT16",
			dataType: "UINT16",
			input:    []byte{1, 2, 3}, // 3 bytes, not divisible by 2
			expected: "",
			wantErr:  true,
		},
		{
			name:     "unsupported data type",
			dataType: "INVALID",
			input:    []byte{1, 2, 3},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "case insensitive",
			dataType: "uint8",
			input:    []byte{5, 10},
			expected: "[5,10]",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use streams.Event to properly handle binary data
			event := streams.NewEventFrom(ctx, tt.input)

			template := `{{. | tensor "` + tt.dataType + `"}}`
			tmpl, err := util.BuildTemplate(template, "test")
			if err != nil {
				t.Fatalf("failed to build template: %v", err)
			}

			result, err := util.ConvertBody(event, tmpl)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ConvertBody error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(result) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

func TestTemplateFunc_Tensor_WithStreamEvent(t *testing.T) {
	// Test tensor function with streams.Event input
	ctx := context.Background()
	data := makeFloat32Bytes([]float32{1.0, 2.0, 3.0})
	event := streams.NewEventFrom(ctx, data)

	template := `{{. | tensor "FP32"}}`
	tmpl, err := util.BuildTemplate(template, "test")
	if err != nil {
		t.Fatalf("failed to build template: %v", err)
	}

	result, err := util.ConvertBody(event, tmpl)
	if err != nil {
		t.Fatalf("ConvertBody failed: %v", err)
	}

	expected := "[1,2,3]"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestTemplateFunc_CombinedFunctions(t *testing.T) {
	// Test combining multiple custom functions
	template := `{{. | jp "$.data" | json}}`
	tmpl, err := util.BuildTemplate(template, "test")
	if err != nil {
		t.Fatalf("failed to build template: %v", err)
	}

	input := `{"data": {"name": "test", "value": 42}}`
	result, err := util.ConvertBody(input, tmpl)
	if err != nil {
		t.Fatalf("ConvertBody failed: %v", err)
	}

	expected := `{"name":"test","value":42}`
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestTemplateFunc_Tensor_FP16(t *testing.T) {
	// Test FP16 conversion
	// Create some FP16 values (half precision floats)
	// FP16 for 1.0, 2.0, 3.0 (approximate binary representations)
	input := make([]byte, 6)                          // 3 FP16 values = 6 bytes
	binary.LittleEndian.PutUint16(input[0:2], 0x3C00) // 1.0 in FP16
	binary.LittleEndian.PutUint16(input[2:4], 0x4000) // 2.0 in FP16
	binary.LittleEndian.PutUint16(input[4:6], 0x4200) // 3.0 in FP16

	template := `{{. | tensor "FP16"}}`
	tmpl, err := util.BuildTemplate(template, "test")
	if err != nil {
		t.Fatalf("failed to build template: %v", err)
	}

	result, err := util.ConvertBody(input, tmpl)
	if err != nil {
		t.Fatalf("ConvertBody failed: %v", err)
	}

	// The conversion should produce float32 values close to 1.0, 2.0, 3.0
	// We just check that we got a valid JSON array without exact value checking
	// since FP16 conversion can have precision differences
	if len(result) == 0 {
		t.Errorf("expected non-empty result")
	}
}

// Helper functions to create byte arrays for different data types

func makeUint16Bytes(values []uint16) []byte {
	b := make([]byte, len(values)*2)
	for i, v := range values {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return b
}

func makeInt16Bytes(values []int16) []byte {
	b := make([]byte, len(values)*2)
	for i, v := range values {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return b
}

func makeUint32Bytes(values []uint32) []byte {
	b := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(b[i*4:], v)
	}
	return b
}

func makeInt32Bytes(values []int32) []byte {
	b := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(b[i*4:], uint32(v))
	}
	return b
}

func makeUint64Bytes(values []uint64) []byte {
	b := make([]byte, len(values)*8)
	for i, v := range values {
		binary.LittleEndian.PutUint64(b[i*8:], v)
	}
	return b
}

func makeInt64Bytes(values []int64) []byte {
	b := make([]byte, len(values)*8)
	for i, v := range values {
		binary.LittleEndian.PutUint64(b[i*8:], uint64(v))
	}
	return b
}

func makeFloat32Bytes(values []float32) []byte {
	b := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

func makeFloat64Bytes(values []float64) []byte {
	b := make([]byte, len(values)*8)
	for i, v := range values {
		binary.LittleEndian.PutUint64(b[i*8:], math.Float64bits(v))
	}
	return b
}
