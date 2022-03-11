// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"strings"
)

type groupVersionKind struct {
	Group     string
	Version   string
	Kind      string
	Namespace string // optional
}

var errBadFormat = errors.New("format: group/version/kind[:namespace]")

func (gvk groupVersionKind) String() string {
	var gvkString string
	if gvk.Group != "" {
		gvkString = strings.Join([]string{gvk.Group, gvk.Version, gvk.Kind}, "/")
	} else {
		gvkString = strings.Join([]string{gvk.Version, gvk.Kind}, "/")
	}
	if gvk.Namespace != "" && gvk.Namespace != "*" {
		gvkString = strings.Join([]string{gvkString, gvk.Namespace}, ":")
	}
	return gvkString
}

func (gvk *groupVersionKind) Parse(value string) error {
	parts := strings.SplitN(value, "/", 3)
	for i := range parts {
		if len(parts[i]) == 0 {
			return errBadFormat
		}
		parts[i] = strings.ToLower(parts[i])
	}
	if len(parts) < 2 {
		return errBadFormat
	}
	if len(parts) == 2 {
		gvk.Version = parts[0]
		gvk.Kind = parts[1]
	} else {
		gvk.Group = parts[0]
		gvk.Version = parts[1]
		gvk.Kind = parts[2]
	}
	if strings.Contains(gvk.Kind, ":") {
		parts = strings.SplitN(gvk.Kind, ":", 2)
		if len(parts) != 2 || len(parts[0]) <= 0 || len(parts[1]) <= 0 {
			return errBadFormat
		}
		gvk.Kind = parts[0]
		if parts[1] != "*" {
			gvk.Namespace = parts[1]
		}
	}
	return nil
}

type gvkFlag []groupVersionKind

func (f *gvkFlag) String() string {
	return fmt.Sprint(*f)
}

func (f *gvkFlag) Set(value string) error {
	var gvk groupVersionKind
	if err := gvk.Parse(value); err != nil {
		return err
	}
	*f = append(*f, gvk)
	return nil
}

func (f *gvkFlag) Type() string {
	return "[group/]version/resource[:namespace]"
}
