// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package openinferenceclient

import (
	"testing"
)

func TestNewConfiguration_Defaults(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")

	if conf.Address != "localhost:8001" {
		t.Errorf("expected Address localhost:8001, got %s", conf.Address)
	}
	if conf.ModelName != "test-model" {
		t.Errorf("expected ModelName test-model, got %s", conf.ModelName)
	}
	if conf.NumInstances != 1 {
		t.Errorf("expected default NumInstances 1, got %d", conf.NumInstances)
	}
	if conf.Timeout != 30 {
		t.Errorf("expected default Timeout 30, got %d", conf.Timeout)
	}
	if conf.inputTemplateObjMap == nil {
		t.Error("expected inputTemplateObjMap to be initialized")
	}
}

func TestConfiguration_SetInputTemplates_Valid(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")

	inputTemplates := []InputTemplateSpec{
		{
			Name:     "input",
			Template: `{"data": [1.0, 2.0, 3.0]}`,
		},
	}
	err := conf.setInputTemplates(inputTemplates)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(conf.inputTemplateObjMap) != 1 {
		t.Errorf("expected 1 compiled template, got %d", len(conf.inputTemplateObjMap))
	}
}

func TestConfiguration_SetInputTemplates_Invalid(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")

	inputTemplates := []InputTemplateSpec{
		{
			Name:     "input",
			Template: `{{missing_func "test"}}`,
		},
	}
	err := conf.setInputTemplates(inputTemplates)

	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestConfiguration_SetInputTemplates_Empty(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")

	err := conf.setInputTemplates([]InputTemplateSpec{})

	if err != nil {
		t.Errorf("expected no error for empty templates, got %v", err)
	}
	if len(conf.inputTemplateObjMap) != 0 {
		t.Error("expected inputTemplateObjMap to remain empty")
	}
}

func TestConfiguration_SetOutputTemplate_Valid(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")

	tmpl := `{"predictions": {{. | jp "$.outputs[0].data"}}}`
	err := conf.setOutputTemplate(tmpl)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if conf.outputTemplateObj == nil {
		t.Error("expected outputTemplateObj to be compiled")
	}
}

func TestConfiguration_SetOutputTemplate_Invalid(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")

	tmpl := `{"result": {{undefined_func "test"}}}`
	err := conf.setOutputTemplate(tmpl)

	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestConfiguration_SetOutputTemplate_Empty(t *testing.T) {
	conf := NewConfiguration("localhost:8001", "test-model")

	err := conf.setOutputTemplate("")

	if err != nil {
		t.Errorf("expected no error for empty template, got %v", err)
	}
	if conf.outputTemplateObj != nil {
		t.Error("expected outputTemplateObj to be nil for empty string")
	}
}

func TestTensorSpec_StructFields(t *testing.T) {
	spec := TensorSpec{
		Name:      "input_tensor",
		DataType:  "FP32",
		Shape:     []int64{1, 3, 224, 224},
		DataField: "image_data",
	}

	if spec.Name != "input_tensor" {
		t.Errorf("expected Name input_tensor, got %s", spec.Name)
	}
	if spec.DataType != "FP32" {
		t.Errorf("expected DataType FP32, got %s", spec.DataType)
	}
	if len(spec.Shape) != 4 {
		t.Errorf("expected Shape length 4, got %d", len(spec.Shape))
	}
	if spec.Shape[0] != 1 || spec.Shape[1] != 3 || spec.Shape[2] != 224 || spec.Shape[3] != 224 {
		t.Errorf("expected Shape [1,3,224,224], got %v", spec.Shape)
	}
	if spec.DataField != "image_data" {
		t.Errorf("expected DataField image_data, got %s", spec.DataField)
	}
}

func TestConfiguration_AllFields(t *testing.T) {
	conf := Configuration{
		Address:      "triton.example.com:8001",
		ModelName:    "resnet50",
		ModelVersion: "2",
		NumInstances: 8,
		InputTensorSpec: []TensorSpec{
			{Name: "input", DataType: "FP32", Shape: []int64{1, 3, 224, 224}},
		},
		OutputTensorSpec: []TensorSpec{
			{Name: "output", DataType: "FP32", Shape: []int64{1, 1000}},
		},
		Timeout: 60,
		Params: map[string]string{
			"key1": "value1",
		},
		InputTemplates: []InputTemplateSpec{
			{Name: "input", Template: `{"data": [1.0]}`},
		},
		OutputTemplate: `{"result": {{. | jp "$.outputs[0].data"}}}`,
	}

	if conf.Address != "triton.example.com:8001" {
		t.Errorf("expected Address triton.example.com:8001, got %s", conf.Address)
	}
	if conf.ModelName != "resnet50" {
		t.Errorf("expected ModelName resnet50, got %s", conf.ModelName)
	}
	if conf.ModelVersion != "2" {
		t.Errorf("expected ModelVersion 2, got %s", conf.ModelVersion)
	}
	if conf.NumInstances != 8 {
		t.Errorf("expected NumInstances 8, got %d", conf.NumInstances)
	}
	if conf.Timeout != 60 {
		t.Errorf("expected Timeout 60, got %d", conf.Timeout)
	}
	if len(conf.InputTensorSpec) != 1 {
		t.Errorf("expected 1 InputTensorSpec, got %d", len(conf.InputTensorSpec))
	}
	if len(conf.OutputTensorSpec) != 1 {
		t.Errorf("expected 1 OutputTensorSpec, got %d", len(conf.OutputTensorSpec))
	}
	if conf.Params["key1"] != "value1" {
		t.Errorf("expected Params key1=value1, got %s", conf.Params["key1"])
	}
}
