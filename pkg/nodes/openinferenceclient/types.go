package openinferenceclient

// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

import (
	"text/template"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/util"
)

// Configuration holds the configuration for the OpenInference gRPC client
type Configuration struct {
	// Address is the gRPC server address (host:port)
	Address string `json:"address"`
	// ModelName is the name of the model to use for inference
	ModelName string `json:"model_name"`
	// ModelVersion is the version of the model (optional)
	ModelVersion string `json:"model_version,omitempty"`
	// Params are additional parameters to pass to the model
	Params map[string]string `json:"params,omitempty"`
	// NumInstances specifies the number of parallel worker goroutines
	NumInstances int `json:"num_instances,omitempty"`
	// InputTensorSpec defines how to convert input data to tensors
	// This can be a template that produces a JSON representation of the input tensors
	InputTensorSpec []TensorSpec `json:"input_tensor_spec,omitempty"`
	// OutputTensorSpec defines how to extract output from the inference response
	// This can be a template to process the response
	OutputTensorSpec []TensorSpec `json:"output_tensor_spec,omitempty"`
	// Timeout for gRPC calls in seconds
	Timeout int `json:"timeout,omitempty"`

	InputTemplates []InputTemplateSpec `json:"input_templates,omitempty"`
	OutputTemplate string              `json:"output_template,omitempty"`

	// Internal template objects (compiled from specs)
	inputTemplateObjMap map[string]*template.Template
	outputTemplateObj   *template.Template
}

// TensorSpec defines a single tensor specification
type TensorSpec struct {
	// Name of the tensor
	Name string `json:"name"`
	// DataType of the tensor (BOOL, INT8, UINT8, INT16, UINT16, INT32, UINT32, INT64, UINT64, FP16, FP32, FP64, BYTES)
	DataType string `json:"datatype"`
	// Shape of the tensor (e.g., [1, 224, 224, 3])
	Shape []int64 `json:"shape"`
	// DataField specifies the field name in the input event to use for this tensor's data
	// If not specified, the entire input body is used
	DataField string `json:"data_field,omitempty"`
}

type InputTemplateSpec struct {
	Name string `json:"name"`
	// Template string to generate input tensor specification JSON
	Template string `json:"template"`
}

// NewConfiguration creates a new Configuration with default values
func NewConfiguration(address, modelName string) *Configuration {
	conf := &Configuration{
		Address:             address,
		ModelName:           modelName,
		NumInstances:        1,
		Timeout:             30,
		inputTemplateObjMap: make(map[string]*template.Template),
	}
	return conf
}

// setInputTemplate compiles the input tensor specification template
func (conf *Configuration) setInputTemplates(inputTemplates []InputTemplateSpec) error {
	if len(inputTemplates) > 0 && conf.inputTemplateObjMap == nil {
		conf.inputTemplateObjMap = make(map[string]*template.Template)
	}
	for _, inputTemplate := range inputTemplates {
		if inputTemplate.Template != "" {
			tmpl, err := util.BuildTemplate(inputTemplate.Template, inputTemplate.Name)
			if err != nil {
				return err
			}
			conf.inputTemplateObjMap[inputTemplate.Name] = tmpl
		}
	}
	return nil
}

// setOutputTemplate compiles the output tensor specification template
func (conf *Configuration) setOutputTemplate(templateStr string) error {
	if templateStr != "" {
		tmpl, err := util.BuildTemplate(templateStr, "outputTensorSpec")
		if err != nil {
			return err
		}
		conf.outputTemplateObj = tmpl
	}
	return nil
}
