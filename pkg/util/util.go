package util

import (
	"encoding/json"
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
