package opa

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Fake emulates OPA Data API for testing
type Fake struct {
	prefix []string
	Data   map[string]interface{}
	// These functions will be called each time the Fake
	// client is used.
	OnPatchData func(path, op string, value *interface{})
	OnPutData   func(path string, value interface{})
	OnPostData  func(path string, value interface{})
}

// Prefix implements Data
func (f *Fake) Prefix(path string) Data {
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			f.prefix = append(f.prefix, segment)
		}
	}
	return f
}

// PatchData implements Data
func (f *Fake) PatchData(path string, op string, value *interface{}) error {
	// Only remove is currently simulated
	if f.OnPatchData != nil {
		defer f.OnPatchData(path, op, value)
	}
	if op != "remove" {
		return fmt.Errorf("unsupported operation %s", op)
	}
	fullPath, root := f.fullpath(path), f.Data
	if root == nil {
		return nil // nothing to remove, parent path does not exist
	}
	for _, segment := range fullPath[0 : len(fullPath)-1] {
		curr, ok := root[segment]
		if !ok {
			return nil // nothing to remove, parent path does not exist
		}
		if root, ok = curr.(map[string]interface{}); !ok {
			return fmt.Errorf("wrong path %s", fullPath)
		}
	}
	delete(root, fullPath[len(fullPath)-1])
	return nil
}

// PutData implements Data
func (f *Fake) PutData(path string, value interface{}) error {
	if f.OnPutData != nil {
		defer f.OnPutData(path, value)
	}
	jsonValue, err := jsonRoundTrip(value)
	if err != nil {
		return err
	}
	fullPath, root := f.fullpath(path), f.Data
	if root == nil {
		root = make(map[string]interface{})
		f.Data = root
	}
	for _, segment := range fullPath[0 : len(fullPath)-1] {
		if curr, ok := root[segment]; !ok {
			newroot := make(map[string]interface{})
			root[segment] = newroot
			root = newroot
		} else {
			if root, ok = curr.(map[string]interface{}); !ok {
				return fmt.Errorf("wrong path %s", fullPath)
			}
		}
	}
	root[fullPath[len(fullPath)-1]] = jsonValue
	return nil
}

var errPostDataNotSupported error = errors.New("unsupported function PostData")

// PostData implements Data
func (f *Fake) PostData(path string, value interface{}) (json.RawMessage, error) {
	if f.OnPostData != nil {
		defer f.OnPostData(path, value)
	}
	return nil, errPostDataNotSupported
}

func (f *Fake) fullpath(path string) []string {
	fullpath := append([]string{}, f.prefix...)
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			fullpath = append(fullpath, segment)
		}
	}
	return fullpath
}

func jsonRoundTrip(obj interface{}) (interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
