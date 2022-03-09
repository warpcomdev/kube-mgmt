package data

import (
	"encoding/json"
	"fmt"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
)

// mockData emulates OPA Data Client API
type mockData struct {
	prefix   []string
	data     opa_client.Batch
	onUpdate func(path string, value interface{}) // called after each PutData
	onRemove func(path string)                    // called after each PatchData
}

// Prefix implements Data
func (f *mockData) Prefix(path string) opa_client.Data {
	f.prefix = append(f.prefix, path)
	return f
}

// PatchData implements Data. Currently only "remove" supported
func (f *mockData) PatchData(path string, op string, value *interface{}) error {
	if f.onRemove != nil {
		defer f.onRemove(path)
	}
	if op != "remove" {
		return fmt.Errorf("unsupported operation %s", op)
	}
	return f.data.Remove(path)
}

// PutData implements Data
func (f *mockData) PutData(path string, value interface{}) error {
	if f.onUpdate != nil {
		defer f.onUpdate(path, value)
	}
	if path == "" || path == "/" { // on initial load
		f.data = opa_client.Batch(value.(map[string]interface{}))
		return nil
	}
	return f.data.Add(path, value)
}

// PostData implements Data. Currently not supported.
func (f *mockData) PostData(path string, value interface{}) (json.RawMessage, error) {
	return nil, nil
}
