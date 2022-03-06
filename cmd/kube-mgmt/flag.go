// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"
)

type groupVersionKind struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
}

const (
	formatWithNamespace    = "[group/]version/kind[:namespace]"
	formatWithoutNamespace = "[group/]version/kind"
)

var (
	errBadFormatWithNamespace    = fmt.Errorf("format: %s", formatWithNamespace)
	errBadFormatWithoutNamespace = fmt.Errorf("format: %s", formatWithoutNamespace)
)

func (gvk groupVersionKind) String() string {
	var name string
	if gvk.Group != "" {
		name = fmt.Sprintf("%v/%v/%v", gvk.Group, gvk.Version, gvk.Kind)
	} else {
		name = fmt.Sprintf("%v/%v", gvk.Version, gvk.Kind)
	}
	if gvk.Namespace != "" {
		return strings.Join([]string{name, gvk.Namespace}, ":")
	}
	return name
}

func (gvk *groupVersionKind) Parse(value string) bool {
	parts := strings.SplitN(value, "/", 3)
	for i := range parts {
		if len(parts[i]) == 0 {
			return false
		}
		parts[i] = strings.ToLower(parts[i])
	}
	if len(parts) < 2 {
		return false
	}
	if len(parts) == 2 {
		gvk.Version = parts[0]
		gvk.Kind = parts[1]
	} else {
		gvk.Group = parts[0]
		gvk.Version = parts[1]
		gvk.Kind = parts[2]
	}
	if index := strings.Index(gvk.Kind, ":"); index >= 0 {
		if index == 0 || index >= len(gvk.Kind)-1 {
			return false
		}
		namespace := gvk.Kind[index+1:]
		gvk.Kind = gvk.Kind[:index]
		if namespace != "" && namespace != "*" {
			gvk.Namespace = namespace
		}
	}
	return true
}

type gvkFlag struct {
	Gvk               []groupVersionKind
	SupportsNamespace bool
}

func (f *gvkFlag) String() string {
	return fmt.Sprint(f.Gvk)
}

func (f *gvkFlag) Set(value string) error {
	var gvk groupVersionKind
	if ok := gvk.Parse(value); !ok {
		if f.SupportsNamespace {
			return errBadFormatWithNamespace
		}
		return errBadFormatWithoutNamespace
	}
	if !f.SupportsNamespace && gvk.Namespace != "" {
		return errBadFormatWithoutNamespace
	}
	f.Gvk = append(f.Gvk, gvk)
	return nil
}

func (f *gvkFlag) Type() string {
	if f.SupportsNamespace {
		return formatWithNamespace
	}
	return formatWithoutNamespace
}
