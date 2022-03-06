// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types contains type information used by controllers.
package types

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceType describes a resource type in Kubernetes.
type ResourceType struct {
	// Namespaced indicates if this kind is namespaced.
	Namespaced bool
	Resource   string
	Group      string
	Version    string
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
	return strings.Join(parts, "/")
}

// ConstrainedResourceType describes a Resource Type (optionally) constrained to a Namespace.
// If the resource type is not namespaced, then this should behave
// the same as a ResourceType
type ConstrainedResourceType struct {
	ResourceType
	Namespace string
}

func (t ConstrainedResourceType) String() string {
	result := t.ResourceType.String()
	if namespace := t.GetNamespace(); namespace != metav1.NamespaceAll {
		result = strings.Join([]string{result, namespace}, ":")
	}
	return result
}

// GetNamespace returns the specific namespace of this resource, or metav1.Namespaceall
func (t ConstrainedResourceType) GetNamespace() string {
	if !t.Namespaced || t.Namespace == "*" || t.Namespace == "" {
		return metav1.NamespaceAll
	}
	return t.Namespace
}
