// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"text/template"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

var funcMap = template.FuncMap{
	// "jp" function allows evaluating a JSONPath expression on the input JSON data
	"jp": jpFunc,
	// "json" function converts a Go value to its JSON string representation
	"json": jsonFunc,
	// "tensor" function converts byte data to numerical array based on Inference Protocol v2 data type
	"tensor": tensorFunc,
}

func jpFunc(jsonPath string, jsonData any) (any, error) {
	res, err := EvaluateJSONPath(jsonData, jsonPath)
	if err != nil {
		return nil, err
	}
	if len(res) == 1 {
		return res[0], nil
	}
	return res, nil
}

func jsonFunc(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v), err
	}
	return string(data), nil
}

// tensorFunc converts byte data to a numerical array based on Inference Protocol v2 data type
func tensorFunc(dataType string, v any) (string, error) {
	var data []byte
	switch val := v.(type) {
	case []byte:
		data = val
	case string:
		data = []byte(val)
	case streams.Event:
		data = val.GetBody()
	default:
		return "", fmt.Errorf("tensor: unsupported input type %T, expected []byte, string, or streams.Event", v)
	}

	result, err := bytesToTensor(dataType, data)
	if err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("tensor: failed to marshal result: %w", err)
	}
	return string(jsonData), nil
}

// bytesToTensor converts a byte array to a numerical array based on Inference Protocol v2 data type
func bytesToTensor(dataType string, data []byte) (any, error) {
	// Normalize data type to uppercase
	dataType = strings.ToUpper(dataType)

	switch dataType {
	case "BOOL":
		result := make([]bool, len(data))
		for i, b := range data {
			result[i] = b != 0
		}
		return result, nil

	case "UINT8", "BYTES":
		// Convert to []int to avoid base64 encoding in JSON
		result := make([]int, len(data))
		for i, b := range data {
			result[i] = int(b)
		}
		return result, nil

	case "INT8":
		result := make([]int, len(data))
		for i, b := range data {
			result[i] = int(int8(b))
		}
		return result, nil

	case "UINT16":
		if len(data)%2 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 2 for UINT16", len(data))
		}
		result := make([]uint16, len(data)/2)
		for i := range result {
			result[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
		}
		return result, nil

	case "INT16":
		if len(data)%2 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 2 for INT16", len(data))
		}
		result := make([]int16, len(data)/2)
		for i := range result {
			result[i] = int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
		}
		return result, nil

	case "UINT32":
		if len(data)%4 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 4 for UINT32", len(data))
		}
		result := make([]uint32, len(data)/4)
		for i := range result {
			result[i] = binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		}
		return result, nil

	case "INT32":
		if len(data)%4 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 4 for INT32", len(data))
		}
		result := make([]int32, len(data)/4)
		for i := range result {
			result[i] = int32(binary.LittleEndian.Uint32(data[i*4 : i*4+4]))
		}
		return result, nil

	case "UINT64":
		if len(data)%8 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 8 for UINT64", len(data))
		}
		result := make([]uint64, len(data)/8)
		for i := range result {
			result[i] = binary.LittleEndian.Uint64(data[i*8 : i*8+8])
		}
		return result, nil

	case "INT64":
		if len(data)%8 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 8 for INT64", len(data))
		}
		result := make([]int64, len(data)/8)
		for i := range result {
			result[i] = int64(binary.LittleEndian.Uint64(data[i*8 : i*8+8]))
		}
		return result, nil

	case "FP16":
		// FP16 (half precision) - convert to float32 for JSON representation
		if len(data)%2 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 2 for FP16", len(data))
		}
		result := make([]float32, len(data)/2)
		for i := range result {
			bits := binary.LittleEndian.Uint16(data[i*2 : i*2+2])
			result[i] = float16ToFloat32(bits)
		}
		return result, nil

	case "FP32":
		if len(data)%4 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 4 for FP32", len(data))
		}
		result := make([]float32, len(data)/4)
		for i := range result {
			bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
			result[i] = math.Float32frombits(bits)
		}
		return result, nil

	case "FP64":
		if len(data)%8 != 0 {
			return nil, fmt.Errorf("tensor: data length %d is not a multiple of 8 for FP64", len(data))
		}
		result := make([]float64, len(data)/8)
		for i := range result {
			bits := binary.LittleEndian.Uint64(data[i*8 : i*8+8])
			result[i] = math.Float64frombits(bits)
		}
		return result, nil

	default:
		return nil, fmt.Errorf("tensor: unsupported data type '%s'. Supported types: BOOL, UINT8, INT8, UINT16, INT16, UINT32, INT32, UINT64, INT64, FP16, FP32, FP64, BYTES", dataType)
	}
}

// float16ToFloat32 converts IEEE 754 half-precision (16-bit) float to float32
func float16ToFloat32(bits uint16) float32 {
	// Extract sign, exponent, and mantissa
	sign := uint32(bits&0x8000) << 16
	exponent := uint32(bits&0x7C00) >> 10
	mantissa := uint32(bits & 0x03FF)

	var result uint32
	if exponent == 0 {
		if mantissa == 0 {
			// Zero
			result = sign
		} else {
			// Denormalized number
			exponent = 1
			for (mantissa & 0x400) == 0 {
				mantissa <<= 1
				exponent--
			}
			mantissa &= 0x3FF
			result = sign | ((exponent + 112) << 23) | (mantissa << 13)
		}
	} else if exponent == 0x1F {
		// Infinity or NaN
		result = sign | 0x7F800000 | (mantissa << 13)
	} else {
		// Normalized number
		result = sign | ((exponent + 112) << 23) | (mantissa << 13)
	}

	return math.Float32frombits(result)
}

func Convert(in, out interface{}) error {
	// marshal to json, unmarshal to config
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, out)
	if err != nil {
		return err
	}
	return nil
}

func ValidateJSONPath(jsonPath string) error {
	_, err := jp.ParseString(jsonPath)
	if err != nil {
		return fmt.Errorf("invalid json path: %s", err.Error())
	}
	return nil
}

func NormalizeJSONPath(jsonPath string) string {
	// expect json path to be a simple filter expression starting with $[?...]
	// if it does not start with $, add it
	if strings.HasPrefix(jsonPath, "$[?") {
		return jsonPath
	}
	return "$[?" + jsonPath + "]"
}

func EvaluateJSONPath(jsonData any, jsonPath string) ([]any, error) {
	// use jsonpath to evaluate the json path
	x, err := BuildJSONPathExpression(jsonPath)
	if err != nil {
		return nil, err
	}
	return EvaluateJSONPathOnExpr(jsonData, x)
}

func BuildJSONPathExpression(jsonPath string) (jp.Expr, error) {
	if jsonPath == "" {
		jsonPath = "$"
	}
	expr, err := jp.ParseString(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("invalid json path: %s", err.Error())
	}
	return expr, nil
}

func EvaluateJSONPathOnExpr(jsonData any, x jp.Expr) ([]any, error) {
	var data []byte
	switch v := jsonData.(type) {
	case streams.Event:
		data = v.GetBody()
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil, fmt.Errorf("unsupported type for jsonData: %T", v)
	}

	obj, err := oj.Parse(data)
	if err != nil {
		return nil, err
	}
	res := x.Get(obj)
	return res, nil
}

func BuildTemplate(templateStr, templateName string) (*template.Template, error) {
	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %s", err.Error())
	}
	return tmpl, nil
}

func ConvertBody(in any, t *template.Template) ([]byte, error) {
	if t == nil {
		return in.([]byte), nil
	}

	var body []byte
	switch bodyType := in.(type) {
	case streams.Event:
		body = bodyType.GetBody()
	case string:
		body = []byte(bodyType)
	case []byte:
		body = bodyType
	default:
		body = []byte(fmt.Sprintf("%v", bodyType))
	}

	var sb strings.Builder
	err := t.Execute(&sb, string(body))
	if err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}
