package opa

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestFakePrefix(t *testing.T) {

	testCases := []struct {
		prefix   string
		expected []string
	}{
		{"short_prefix", []string{"short_prefix"}},
		{"long/prefix", []string{"long", "prefix"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Test prefix %s", tc.prefix), func(t *testing.T) {
			f := (&Fake{}).Prefix(tc.prefix).(*Fake)
			if !reflect.DeepEqual(f.prefix, tc.expected) {
				t.Errorf("Failed to parse prefix '%s'. Expected %v, got %v", tc.prefix, tc.expected, f.prefix)
				t.Fail()
			}
		})
	}
}

type testCase struct {
	prefix   string
	path     string
	data     string
	expected string
}

func TestFakePutData(t *testing.T) {

	testCases := []testCase{
		{
			prefix: "short_prefix",
			path:   "short_path",
			data: `{
				"key1": "value1"
			}`,
			expected: `{
				"short_prefix": {
					"short_path": {
						"key1": "value1"
					}
				}
			}`,
		},
		{
			prefix: "short_prefix",
			path:   "long/path",
			data: `{
				"key2": "value2"
			}`,
			expected: `{
				"short_prefix": {
					"long": {
						"path": {
							"key2": "value2"
						}
					}
				}
			}`,
		},
		{
			prefix: "long/prefix",
			path:   "short_path",
			data: `{
				"key3": "value3"
			}`,
			expected: `{
				"long": {
					"prefix": {
						"short_path": {
							"key3": "value3"
						}
					}
				}
			}`,
		},
		{
			prefix: "long/prefix",
			path:   "long/path",
			data: `{
				"key4": "value4"
			}`,
			expected: `{
				"long": {
					"prefix": {
						"long": {
							"path": {
								"key4": "value4"
							}
						}
					}
				}
			}`,
		},
	}

	for _, tc := range variations(testCases) {
		t.Run(fmt.Sprintf("Test PutData %s, %s", tc.prefix, tc.path), func(t *testing.T) {
			f := (&Fake{}).Prefix(tc.prefix).(*Fake)
			dataJson := mustUnmarshalJSON(bytes.NewReader([]byte(tc.data)))
			if err := f.PutData(tc.path, dataJson); err != nil {
				t.Errorf("Unexpected error in PutData: %v", err)
				t.FailNow()
			}
			expectedJson := mustUnmarshalJSON(bytes.NewReader([]byte(tc.expected)))
			if !reflect.DeepEqual(f.Data, expectedJson) {
				t.Errorf("Failed to put at prefix '%s' path '%s'. Expected %v, got %v", tc.prefix, tc.path, expectedJson, f.Data)
				t.Fail()
			}
		})
	}
}

func TestFakePatchData(t *testing.T) {

	testCases := []testCase{
		{
			prefix: "short_prefix",
			path:   "short_path",
			data: `{
				"short_prefix": {
					"short_path": {
						"key1": "value1"
					}
				}
			}`,
			expected: `{
				"short_prefix": {}
			}`,
		},
		{
			prefix: "short_prefix",
			path:   "long/path",
			data: `{
				"short_prefix": {
					"long": {
						"path": {
							"key2": "value2"
						}
					}
				}
			}`,
			expected: `{
				"short_prefix": {
					"long": {}
				}
			}`,
		},
		{
			prefix: "long/prefix",
			path:   "short_path",
			data: `{
				"long": {
					"prefix": {
						"short_path": {
							"key3": "value3"
						}
					}
				}
			}`,
			expected: `{
				"long": {
					"prefix": {}
				}
			}`,
		},
		{
			prefix: "long/prefix",
			path:   "long/path",
			data: `{
				"long": {
					"prefix": {
						"long": {
							"path": {
								"key4": "value4"
							}
						}
					}
				}
			}`,
			expected: `{
				"long": {
					"prefix": {
						"long": {}
					}
				}
			}`,
		},
	}

	for _, tc := range variations(testCases) {
		t.Run(fmt.Sprintf("Test PutData '%s', '%s'", tc.prefix, tc.path), func(t *testing.T) {
			f := (&Fake{}).Prefix(tc.prefix).(*Fake)
			dataJson := mustUnmarshalJSON(bytes.NewReader([]byte(tc.data)))
			f.Data = dataJson.(map[string]interface{})
			if err := f.PatchData(tc.path, "remove", nil); err != nil {
				t.Errorf("Unexpected error in PatchData: %v", err)
				t.FailNow()
			}
			expectedJson := mustUnmarshalJSON(bytes.NewReader([]byte(tc.expected)))
			if !reflect.DeepEqual(f.Data, expectedJson) {
				t.Errorf("Failed to remove at prefix '%s' path '%s'. Expected %v, got %v", tc.prefix, tc.path, expectedJson, f.Data)
				t.Fail()
			}
		})
	}
}

func variations(testCases []testCase) []testCase {
	variations := make([]testCase, 0, 4*len(testCases))
	for _, tc := range testCases {
		variations = append(variations, tc,
			testCase{
				prefix:   "/" + tc.prefix,
				path:     tc.path,
				data:     tc.data,
				expected: tc.expected,
			},
			testCase{
				prefix:   tc.prefix,
				path:     "/" + tc.path,
				data:     tc.data,
				expected: tc.expected,
			},
			testCase{
				prefix:   "/" + tc.prefix,
				path:     "/" + tc.path,
				data:     tc.data,
				expected: tc.expected,
			},
		)
	}
	return variations
}
