package util

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

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

func ConvertBody(in any, t *template.Template) (any, error) {
	if t == nil {
		return in, nil
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
