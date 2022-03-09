package opa

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestBatchAdd(t *testing.T) {

	type testCase struct {
		label    string
		input    map[string]string
		expected string
	}

	testCases := []testCase{
		{
			label: "Short paths",
			input: map[string]string{
				"path1": `{ "key1": "value1" }`,
				"path2": `{ "key2": "value2" }`,
			},
			expected: `{
				"path1": {
					"key1": "value1"
				},
				"path2": {
					"key2": "value2"
				}
			}`,
		},
		{
			label: "Long paths",
			input: map[string]string{
				"long/path1":        `{ "key1": "value1" }`,
				"even/longer/path2": `{ "key2": "value2" }`,
			},
			expected: `{
				"long": {
					"path1": {
						"key1": "value1"
					}
				},
				"even": {
					"longer": {
						"path2": {
							"key2": "value2"
						}
					}
				}
			}`,
		},
		{
			label: "mixed paths",
			input: map[string]string{
				"root/path1":        `{ "key1": "value1" }`,
				"root/branch/path2": `{ "key2": "value2" }`,
			},
			expected: `{
				"root": {
					"path1": {
						"key1": "value1"
					},
					"branch": {
						"path2": {
							"key2": "value2"
						}
					}
				}
			}`,
		},
	}

	test := func(t *testing.T, tc testCase, addSlash string) {
		t.Helper()
		f := make(Batch)
		for k, v := range tc.input {
			dataJson := mustUnmarshalJSON(bytes.NewReader([]byte(v)))
			if err := f.Add(addSlash+k, dataJson); err != nil {
				t.Errorf("Unexpected error in Add: %v", err)
				t.FailNow()
			}
		}
		expectedJson := mustUnmarshalJSON(bytes.NewReader([]byte(tc.expected)))
		if !reflect.DeepEqual(map[string]interface{}(f), expectedJson) {
			t.Errorf("Failed to add data. Expected %v, got %v", expectedJson, f)
			t.Fail()
		}
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Test Batch Add %s", tc.label), func(t *testing.T) {
			test(t, tc, "")
		})
		t.Run(fmt.Sprintf("Test Batch Add (Rooted) %s", tc.label), func(t *testing.T) {
			test(t, tc, "/")
		})
	}
}

func TestBatchRemove(t *testing.T) {

	type testCase struct {
		label    string
		data     string
		path     string
		expected string
	}

	testCases := []testCase{
		{
			label: "Short paths",
			data: `{
				"path1": {
					"key1": "value1"
				},
				"path2": {
					"key2": "value2"
				}
			}`,
			path: "path1",
			expected: `{
				"path2": {
					"key2": "value2"
				}
			}`,
		},
		{
			label: "Long paths",
			data: `{
				"long": {
					"path1": {
						"key1": "value1"
					}
				},
				"even": {
					"longer": {
						"path2": {
							"key2": "value2"
						}
					}
				}
			}`,
			path: "even/longer",
			expected: `{
				"long": {
					"path1": {
						"key1": "value1"
					}
				}
			}`,
		},
		{
			label: "mixed paths",
			data: `{
				"root": {
					"path1": {
						"key1": "value1"
					},
					"branch": {
						"path2": {
							"key2": "value2"
						}
					}
				}
			}`,
			path: "root/branch/path2",
			expected: `{
				"root": {
					"path1": {
						"key1": "value1"
					}
				}
			}`,
		},
	}

	test := func(t *testing.T, tc testCase, addSlash string) {
		t.Helper()
		dataJson := mustUnmarshalJSON(bytes.NewReader([]byte(tc.data)))
		f := Batch(dataJson.(map[string]interface{}))
		if err := f.Remove(addSlash + tc.path); err != nil {
			t.Errorf("Unexpected error in PatchData: %v", err)
			t.FailNow()
		}
		expectedJson := mustUnmarshalJSON(bytes.NewReader([]byte(tc.expected)))
		if !reflect.DeepEqual(map[string]interface{}(f), expectedJson) {
			t.Errorf("Failed to remove at path '%s'. Expected %v, got %v", addSlash+tc.path, expectedJson, f)
			t.Fail()
		}
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Test Batch Remove '%s'", tc.label), func(t *testing.T) {
			test(t, tc, "")
		})
		t.Run(fmt.Sprintf("Test Batch Remove (Rooted) '%s'", tc.label), func(t *testing.T) {
			test(t, tc, "/")
		})
	}
}
