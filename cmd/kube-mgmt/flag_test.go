package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/open-policy-agent/kube-mgmt/pkg/configmap"
	"github.com/spf13/cobra"
)

func TestFlagParsing(t *testing.T) {
	var f gvkFlag

	badPaths := []string{
		"foo/bar/",
		"foo",
		"foo:",
		"foo/bar:",
		"foo/:bar",
	}

	for _, tc := range badPaths {
		if err := f.Set(tc); err == nil {
			t.Fatalf("Expected error from %v", tc)
		}
	}

	goodPaths := []struct {
		path string
		gvk  groupVersionKind
	}{
		{
			path: "example.org/Foo/bar",
			gvk:  groupVersionKind{"example.org", "foo", "bar", ""},
		},
		{
			path: "example.org/Bar/baz",
			gvk:  groupVersionKind{"example.org", "bar", "baz", ""},
		},
		{
			path: "v2/corge",
			gvk:  groupVersionKind{"", "v2", "corge", ""},
		},
		{
			path: "short/path:ns1",
			gvk:  groupVersionKind{"", "short", "path", "ns1"},
		},
		{
			path: "star/removed:*",
			gvk:  groupVersionKind{"", "star", "removed", ""},
		},
		{
			path: "example.org/long/path:ns2",
			gvk:  groupVersionKind{"example.org", "long", "path", "ns2"},
		},
	}

	for index, tc := range goodPaths {
		err := f.Set(tc.path)
		if err != nil {
			t.Fatalf("Expected %v from %v but got error: %v", tc.gvk, tc.path, err)
		}
		if len(f) != index+1 {
			t.Fatalf("Expected %v from %s but flag was not parsed", tc.gvk, tc.path)
		}
		if !reflect.DeepEqual(tc.gvk, f[index]) {
			t.Fatalf("Expected %v from %s but got: %v", tc.gvk, tc.path, f[index])
		}
	}
}

func TestFlagString(t *testing.T) {

	var f gvkFlag
	expected := "[example.org/foo/bar]"

	if err := f.Set("example.org/foo/bar"); err != nil || f.String() != expected {
		t.Fatalf("Exepcted %v but got: %v (err: %v)", expected, f.String(), err)
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
