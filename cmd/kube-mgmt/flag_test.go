package main

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/kube-mgmt/pkg/configmap"
	"github.com/spf13/cobra"
)

func TestFlagParsing(t *testing.T) {
	var f gvkFlag

	badPaths := []struct {
		path       string
		namespaced bool
	}{
		{"foo/bar/", false},
		{"foo/bar:", false},
		{"foo/bar:baz", false},
		{"foo", false},
		{"bar/baz/", true},
		{"bar/baz:", true},
	}

	for _, tc := range badPaths {
		f.SupportsNamespace = tc.namespaced
		if err := f.Set(tc.path); err == nil {
			t.Fatalf("Expected error from %v", tc)
		}
	}

	testCases := []struct {
		path       string
		expected   groupVersionKind
		namespaced bool
	}{
		{"example.org/Foo/bar", groupVersionKind{"example.org", "foo", "bar", ""}, false},
		{"example.org/Bar/baz", groupVersionKind{"example.org", "bar", "baz", ""}, false},
		{"v2/corge", groupVersionKind{"", "v2", "corge", ""}, false},
		{"test/some/namespace:ns1", groupVersionKind{"test", "some", "namespace", "ns1"}, true},
		{"remove/star:*", groupVersionKind{"", "remove", "star", ""}, true},
		{"namespaced/without/namespace", groupVersionKind{"namespaced", "without", "namespace", ""}, true},
	}

	for index, testCase := range testCases {
		f.SupportsNamespace = testCase.namespaced
		err := f.Set(testCase.path)
		if err != nil {
			t.Fatalf("Error while parsing %q: %v", testCase.path, err)
		}
		if len(f.Gvk) != index+1 {
			t.Fatalf("Value %q was not added to flag", testCase.path)
		}
		if !reflect.DeepEqual(testCase.expected, f.Gvk[index]) {
			t.Fatalf("Expected %#v but got: %#v", testCase.expected, f.Gvk)
		}
	}
}

func TestFlagString(t *testing.T) {

	var f gvkFlag
	testCases := []struct {
		path       string
		expected   string
		namespaced bool
	}{
		{"example.org/Foo/bar", "example.org/foo/bar", false},
		{"example.org/Bar/baz", "example.org/bar/baz", false},
		{"v2/corge", "v2/corge", false},
		{"test/some/namespace:ns1", "test/some/namespace:ns1", true},
		{"remove/star:*", "remove/star", true},
		{"namespaced/without/namespace", "namespaced/without/namespace", true},
	}

	expected := make([]string, 0, len(testCases))
	for _, testCase := range testCases {
		expected = append(expected, testCase.expected)
		f.SupportsNamespace = testCase.namespaced
		if err := f.Set(testCase.path); err != nil || f.String() != fmt.Sprint(expected) {
			t.Fatalf("Expected %v but got: %v (err: %v)", fmt.Sprint(expected), f.String(), err)
		}
	}
}

func TestPolicyFlags(t *testing.T) {
	tt := []struct {
		name           string
		flag           string
		value          string
		expectFullFlag string
		err            error
	}{
		{
			name:           "valid",
			flag:           "openpolicyagent.org/policy",
			value:          "rego",
			expectFullFlag: "openpolicyagent.org/policy=rego",
			err:            nil,
		},
		{
			name:           "invalidFlag",
			flag:           "-foo",
			value:          "rego",
			expectFullFlag: "",
			err:            errors.New(`key: Invalid value: "-foo": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`),
		},
		{
			name:           "invalidValue",
			flag:           "foo",
			value:          "-rego",
			expectFullFlag: "",
			err:            errors.New(`values[0][foo]: Invalid value: "-rego": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd := &cobra.Command{
				Use:   "test",
				Short: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			var params params
			rootCmd.Flags().StringVarP(&params.policyLabel, "policy-label", "", "", "replace label openpolicyagent.org/policy")
			rootCmd.Flags().StringVarP(&params.policyValue, "policy-value", "", "", "replace value rego")

			rootCmd.SetArgs([]string{"--policy-label=" + tc.flag, "--policy-value=" + tc.value})
			rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
				if rootCmd.Flag("policy-label").Value.String() != "" || rootCmd.Flag("policy-value").Value.String() != "" {
					f, err := configmap.CustomPolicyLabel(params.policyLabel, params.policyValue)
					if err != nil {
						if tc.err.Error() != err.Error() {
							t.Errorf("exp: %v\ngot: %v\n", tc.err.Error(), err.Error())
							t.FailNow()
						}
					}

					if tc.expectFullFlag != f {
						t.Errorf("expected: flag:%v got: %v", tc.expectFullFlag, f)
						t.FailNow()
					}
				}
				return nil
			}
			rootCmd.Execute()
		})
	}
}
