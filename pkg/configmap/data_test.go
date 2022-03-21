package configmap

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/kube-mgmt/internal/expect"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// dataConfig implements testConfig for data configmaps
type dataConfig struct {
	title      string
	namespace  string
	enableData bool
	matchLabel bool
}

// Title implements testConfig
func (d dataConfig) Title() string {
	return d.title
}

// Matcher implements testConfig
func (d dataConfig) Matcher() func(*v1.ConfigMap) (bool, bool) {
	return DefaultConfigMapMatcher([]string{d.namespace}, true, true, d.enableData, PolicyLabelKey, PolicyLabelValue)
}

// Label implements testConfig
func (d dataConfig) Label(cm *v1.ConfigMap) *v1.ConfigMap {
	labeledCM := cm.DeepCopy()
	labeledCM.Labels = map[string]string{
		dataLabelKey: dataLabelValue,
	}
	labeledCM.ResourceVersion = "other" + cm.ResourceVersion
	return labeledCM
}

// MatchLabel implements testConfig
func (d dataConfig) MatchLabel() bool {
	return d.matchLabel
}

// MatchNoLabel implements testConfig
func (d dataConfig) MatchNoLabel() bool {
	return false
}

type request struct {
	path  string
	value string
}

type testCase struct {
	title     string
	configMap *v1.ConfigMap
	expected  []request
}

// OnAdd implements scriptWriter
func (tc testCase) OnAddition(then expect.Action) expect.Script {
	// The config map will be updated key by key
	play := expect.Script{}
	for _, expected := range tc.expected {
		play = append(
			play,
			expect.PutData(expected.path, []byte(expected.value)).Do(nil),
		)
	}
	// TODO: FIX: The current implementation of configmap adds the
	// same configmap twice, because the fake client does not change
	// the resource version when the configmap is annotated.
	for _, expected := range tc.expected {
		play = append(
			play,
			expect.PutData(expected.path, []byte(expected.value)).Do(nil),
		)
	}
	play[len(play)-1].Action = then
	return play
}

// OnDelete implements scriptWriter
func (tc testCase) OnRemoval(then expect.Action) expect.Script {
	// The configmap will be removed at once
	path := strings.Join([]string{tc.configMap.Namespace, tc.configMap.Name}, "/")
	return expect.Script{
		expect.PatchData(path, "remove").Do(then),
	}
}

// ConfigMap implements scriptWriter
func (tc testCase) ConfigMap() *v1.ConfigMap {
	return tc.configMap
}

func TestDataConfigMap(t *testing.T) {
	t.Parallel()

	testCases := []testCase{
		{
			title: "Single Key",
			configMap: &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "datacm1",
					Namespace:       "ns1",
					ResourceVersion: "0",
				},
				Data: map[string]string{
					"item1": `{"valid": "json"}`,
				},
			},
			expected: []request{
				{path: "ns1/datacm1/item1", value: `{"valid":"json"}`},
			},
		},
		{
			title: "Two Keys",
			configMap: &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "datacm2",
					Namespace:       "ns2",
					ResourceVersion: "0",
				},
				Data: map[string]string{
					"item1": `{"valid": "json"}`,
					"item2": `{"other": "value"}`,
				},
			},
			expected: []request{
				{path: "ns2/datacm2/item1", value: `{"valid":"json"}`},
				{path: "ns2/datacm2/item2", value: `{"other":"value"}`},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			namespace := tc.configMap.Namespace
			configs := []testConfig{
				dataConfig{"same namespace, data enabled", namespace, true, true},
				dataConfig{"same namespace, data disabled", namespace, false, false},
				dataConfig{"any namespace, data enabled", "*", true, true},
				dataConfig{"any namespace, data disabled", "*", false, false},
				dataConfig{"other namespace, data enabled", "other", true, false},
				dataConfig{"other namespace, data disabled", "other", false, false},
			}

			processTests := testMatcher(t, configs, tc.ConfigMap(), false)
			testErrorAnnotation(t, tc, dataConfig{"data error", namespace, true, true}, dataStatusAnnotationKey)
			testProcesses(t, tc, processTests, dataStatusAnnotationKey)
		})
	}
}
