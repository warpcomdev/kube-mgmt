package opa

import (
	"fmt"
	"strings"
)

// Batch aggregates several object updates under the same root
type Batch map[string]interface{}

// Add an object under the given subtree
func (f Batch) Add(path string, value interface{}) error {
	fullPath, root := f.fullpath(path), f
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
	root[fullPath[len(fullPath)-1]] = value
	return nil
}

// Remove the subtree rooted at the given path
func (f Batch) Remove(path string) error {
	fullPath, root := f.fullpath(path), f
	for _, segment := range fullPath[0 : len(fullPath)-1] {
		curr, ok := root[segment]
		if !ok {
			return nil
		}
		if root, ok = curr.(map[string]interface{}); !ok {
			return fmt.Errorf("wrong path %s", fullPath)
		}
	}
	delete(root, fullPath[len(fullPath)-1])
	// Traverse the map again, removing empty nodes in the path
	root = f
	for _, segment := range fullPath[0 : len(fullPath)-1] {
		curr := root[segment].(map[string]interface{})
		if len(curr) <= 0 {
			delete(root, segment)
		}
		root = curr
	}
	return nil
}

func (f *Batch) fullpath(path string) []string {
	offset, segments := 0, strings.Split(path, "/")
	// Remove empty parts of path
	for _, segment := range segments {
		if segment != "" {
			segments[offset] = segment
			offset++
		}
	}
	return segments[:offset]
}
