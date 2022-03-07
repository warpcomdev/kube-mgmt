// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types contains type information used by controllers.
package types

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceType describes a resource type in Kubernetes,
// optionally constrained to a particular namespace.
type ResourceType struct {
	// Namespaced indicates if this kind is namespaced.
	Namespaced bool
	Resource   string
	Group      string
	Version    string
	Namespace  string // only meaningful if Namespaced == true.
}

func (t ResourceType) String() string {
	parts := []string{}
	if t.Group != "" {
		parts = append(parts, t.Group)
	}
	if t.Version != "" {
		parts = append(parts, t.Version)
	}
	if t.Resource != "" {
		parts = append(parts, t.Resource)
	}
	result := strings.Join(parts, "/")
	if namespace := t.GetNamespace(); namespace != metav1.NamespaceAll {
		result = strings.Join([]string{result, namespace}, ":")
	}
	return result
}

// GetNamespace returns the specific namespace of this resource, or metav1.Namespaceall
func (t ResourceType) GetNamespace() string {
	if !t.Namespaced || t.Namespace == "*" || t.Namespace == "" {
		return metav1.NamespaceAll
	}
	return t.Namespace
}
