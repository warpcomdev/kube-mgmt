package configmap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/kube-mgmt/internal/expect"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// dataConfig implements testConfig for data configmaps
type policyConfig struct {
	namespace          string
	enablePolicy       bool
	requirePolicyLabel bool
	policyLabelKey     string
	policyLabelValue   string
	configMapKey       string
	configMapValue     string
	matchNoLabel       bool
	matchLabel         bool
}

// Title implements testConfig
func (d policyConfig) Title() string {
	desc := []string{
		fmt.Sprintf("Namespace:%s", d.namespace),
		fmt.Sprintf("Policies:%v", d.enablePolicy),
		fmt.Sprintf("RequirePolicyLabel:%v", d.requirePolicyLabel),
	}
	if d.policyLabelKey == d.configMapKey && d.policyLabelValue == d.configMapValue {
		if d.policyLabelKey == PolicyLabelKey {
			desc = append(desc, "Labels:match")
		} else {
			desc = append(desc, "CustomLabels:match")
		}
	} else {
		desc = append(desc, "labels:mismatch")
	}
	return strings.Join(desc, ",")
}

// Matcher implements testConfig
func (d policyConfig) Matcher() func(*v1.ConfigMap) (bool, bool) {
	return DefaultConfigMapMatcher([]string{d.namespace}, d.requirePolicyLabel, d.enablePolicy, true, d.policyLabelKey, d.policyLabelValue)
}

// Label implements testConfig
func (d policyConfig) Label(cm *v1.ConfigMap) *v1.ConfigMap {
	labeledCM := cm.DeepCopy()
	labeledCM.Labels = map[string]string{
		d.configMapKey: d.configMapValue,
	}
	labeledCM.ResourceVersion = "other" + cm.ResourceVersion
	return labeledCM
}

// MatchLabel implements testConfig
func (d policyConfig) MatchLabel() bool {
	return d.matchLabel
}

// MatchNoLabel implements testConfig
func (d policyConfig) MatchNoLabel() bool {
	return d.matchNoLabel
}

type policyTestCase struct {
	title     string
	configMap *v1.ConfigMap
	expected  []request
}

// OnAdd implements scriptWriter
func (tc policyTestCase) OnAddition(then expect.Action) expect.Script {
	play := expect.Script{}
	for _, expected := range tc.expected {
		play = append(
			play,
			expect.InsertPolicy(expected.path, []byte(expected.value)).Do(nil),
		)
	}
	// TODO: FIX: The current implementation of configmap adds the
	// same configmap twice, because the fake client does not change
	// the resource version when the configmap is annotated.
	for _, expected := range tc.expected {
		play = append(
			play,
			expect.InsertPolicy(expected.path, []byte(expected.value)).Do(nil),
		)
	}
	play[len(play)-1].Action = then
	return play
}

// OnDelete implements scriptWriter
func (tc policyTestCase) OnRemoval(then expect.Action) expect.Script {
	play := expect.Script{}
	for _, expected := range tc.expected {
		play = append(
			play,
			expect.DeletePolicy(expected.path).Do(nil),
		)
	}
	play[len(play)-1].Action = then
	return play
}

// ConfigMap implements scriptWriter
func (tc policyTestCase) ConfigMap() *v1.ConfigMap {
	return tc.configMap
}

func TestPolicyConfigMap(t *testing.T) {
	t.Parallel()

	testCases := []policyTestCase{
		{
			title: "Single Key",
			configMap: &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "policy1",
					Namespace:       "ns1",
					ResourceVersion: "0",
				},
				Data: map[string]string{
					"item1": "first rego policy",
				},
			},
			expected: []request{
				{path: "ns1/policy1/item1", value: `first rego policy`},
			},
		},
		{
			title: "Policy ConfigMap With Two Keys",
			configMap: &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "policy2",
					Namespace:       "ns2",
					ResourceVersion: "0",
				},
				Data: map[string]string{
					"item1": "first rego policy",
					"item2": "second rego policy",
				},
			},
			expected: []request{
				{path: "ns2/policy2/item1", value: "first rego policy"},
				{path: "ns2/policy2/item2", value: "second rego policy"},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			namespace := tc.configMap.Namespace
			configs := []testConfig{
				// namespace, enablePolicy, requirePolicyLabel,policyLabelKey, policyLabelValue, configMapKey, configMapValue, matchNoLabel, matchLabel
				policyConfig{namespace, true, true, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, true},
				policyConfig{namespace, true, true, "other_key", "other_value", "other_key", "other_value", false, true},
				policyConfig{namespace, true, true, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{namespace, true, false, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, true, true},
				policyConfig{namespace, true, false, "other_key", "other_value", "other_key", "other_value", true, true},
				policyConfig{namespace, true, false, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", true, true},
				policyConfig{namespace, false, true, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, false},
				policyConfig{namespace, false, true, "other_key", "other_value", "other_key", "other_value", false, false},
				policyConfig{namespace, false, true, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{namespace, false, false, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, false},
				policyConfig{namespace, false, false, "other_key", "other_value", "other_key", "other_value", false, false},
				policyConfig{namespace, false, false, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{"*", true, true, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, true},
				policyConfig{"*", true, true, "other_key", "other_value", "other_key", "other_value", false, true},
				policyConfig{"*", true, true, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{"*", true, false, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, true, true},
				policyConfig{"*", true, false, "other_key", "other_value", "other_key", "other_value", true, true},
				policyConfig{"*", true, false, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", true, true},
				policyConfig{"*", false, true, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, false},
				policyConfig{"*", false, true, "other_key", "other_value", "other_key", "other_value", false, false},
				policyConfig{"*", false, true, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{"*", false, false, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, false},
				policyConfig{"*", false, false, "other_key", "other_value", "other_key", "other_value", false, false},
				policyConfig{"*", false, false, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{"different_ns", true, true, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, false},
				policyConfig{"different_ns", true, true, "other_key", "other_value", "other_key", "other_value", false, false},
				policyConfig{"different_ns", true, true, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				// Watch out!
				policyConfig{"different_ns", true, false, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, true},
				policyConfig{"different_ns", true, false, "other_key", "other_value", "other_key", "other_value", false, true},
				policyConfig{"different_ns", true, false, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{"different_ns", false, true, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, false},
				policyConfig{"different_ns", false, true, "other_key", "other_value", "other_key", "other_value", false, false},
				policyConfig{"different_ns", false, true, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
				policyConfig{"different_ns", false, false, PolicyLabelKey, PolicyLabelValue, PolicyLabelKey, PolicyLabelValue, false, false},
				policyConfig{"different_ns", false, false, "other_key", "other_value", "other_key", "other_value", false, false},
				policyConfig{"different_ns", false, false, PolicyLabelKey, PolicyLabelValue, "other_key", "other_value", false, false},
			}

			processTests := testMatcher(t, configs, tc.ConfigMap(), true)
			testErrorAnnotation(t, tc, policyConfig{
				namespace, true, false, PolicyLabelKey, PolicyLabelValue,
				PolicyLabelKey, PolicyLabelValue, true, true}, policyStatusAnnotationKey)
			testProcesses(t, tc, processTests, policyStatusAnnotationKey)
		})
	}
}
