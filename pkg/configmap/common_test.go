package configmap

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/kube-mgmt/internal/expect"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// scriptWriter knows how to generate expect.Scripts for a ConfigMap
type scriptWriter interface {
	// ConfigMap that this writer uses.
	ConfigMap() *v1.ConfigMap
	// OnAddition returns the expect.Script for adding the ConfigMap
	OnAddition(then expect.Action) expect.Script
	// OnRemoval returns the expect.Script for removing the ConfigMap
	OnRemoval(then expect.Action) expect.Script
}

// testConfig knows how to build a matcher and label a CM
type testConfig interface {
	// Title for test runs
	Title() string
	// Matcher to use for Sync.New or Sync.NewFromInterface
	Matcher() func(*v1.ConfigMap) (bool, bool)
	// Label the provided ConfigMap, returns a labeled copy
	Label(*v1.ConfigMap) *v1.ConfigMap
	// MatchLabel returns true if the matcher is
	// expected to return true for the labeled ConfigMap
	MatchLabel() bool
	// MatchNoLabel returns true if the matcher is
	// expected to return true for the unlabeled ConfigMap
	MatchNoLabel() bool
}

// testProcess knows the steps to follow to test a ConfigMap
type testProcess struct {
	Title string
	// Bootstrap the clientset with this ConfigMap
	Bootstrap func() (cm *v1.ConfigMap, matches bool)
	// Trigger this expect.Action after Sync completes
	Trigger func(clientset client, t *testing.T, cm *v1.ConfigMap) expect.Action
	// Payload to use when calling the expect.Action
	Payload func() (cm *v1.ConfigMap, matches bool)
}

const guardTime = 250 * time.Millisecond

// setup creates a client and a script for the testProcess
func (p *testProcess) setup(t *testing.T, w scriptWriter) (client, expect.Script) {
	t.Helper()

	initialCM, initialMatch := p.Bootstrap()
	payloadCM, payloadMatch := p.Payload()
	clientset := newFakeClientset(initialCM)

	var trigger expect.Action
	if p.Trigger != nil && payloadCM != nil {
		trigger = p.Trigger(clientset, t, payloadCM)
	}

	var play expect.Script
	switch {

	case initialMatch && trigger != nil && payloadMatch:
		play = append(w.OnAddition(trigger), w.OnAddition(nil)...)

	case initialMatch && trigger != nil && !payloadMatch:
		play = append(w.OnAddition(trigger), w.OnRemoval(nil)...)

	case initialMatch && trigger == nil:
		play = w.OnAddition(nil)

	case !initialMatch && trigger != nil && payloadMatch:
		play = append(expect.Script{expect.Nothing(guardTime).Do(trigger)}, w.OnAddition(nil)...)

	case !initialMatch && trigger != nil && !payloadMatch:
		play = expect.Script{expect.Nothing(guardTime).Do(trigger)}

	default:
		play = expect.Script{}
	}
	play = append(play, expect.Nothing(guardTime).Do(nil))

	return clientset, play
}

// Test all processes than can involve adding, removing or relabeling a ConfigMap
func testProcesses(t *testing.T, w scriptWriter, configs []testConfig, annotationKey string) {

	// payload for MustDelete action
	deleted := func() (*v1.ConfigMap, bool) {
		return w.ConfigMap(), false
	}

	// payload for nil action
	nothing := func() (*v1.ConfigMap, bool) {
		return nil, false
	}

	for _, config := range configs {
		config := config
		t.Run(config.Title(), func(t *testing.T) {
			t.Parallel()

			unlabeledCM, unlabeledMatch := w.ConfigMap(), config.MatchNoLabel()
			unlabeled := func() (*v1.ConfigMap, bool) {
				return unlabeledCM, unlabeledMatch
			}

			labeledCM, labeledMatch := config.Label(w.ConfigMap()), config.MatchLabel()
			labeled := func() (*v1.ConfigMap, bool) {
				return labeledCM, labeledMatch
			}

			processes := []testProcess{
				{"Load empty => create unlabeled", nothing, client.MustCreate, unlabeled},
				{"Load empty => create labeled", nothing, client.MustCreate, labeled},
				{"Load unlabeled => do nothing", unlabeled, nil, nothing},
				{"Load labeled => do nothing", labeled, nil, nothing},
				{"Load unlabeled => delete", unlabeled, client.MustDelete, deleted},
				{"Load labeled => delete", labeled, client.MustDelete, deleted},
				{"Load unlabeled => update labeled", unlabeled, client.MustUpdate, labeled},
				{"Load labeled => update unlabeled", labeled, client.MustUpdate, unlabeled},
			}

			for _, process := range processes {
				process := process
				t.Run(process.Title, func(t *testing.T) {
					t.Parallel()

					clientset, play := process.setup(t, w)
					Play(t, clientset, play, labeledCM.Namespace, config.Matcher())
					if cm, matches := process.Payload(); matches {
						clientset.MustBeAnnotated(t, cm, annotationKey, `{"status":"ok"}`)
					}
				})
			}
		})
	}
}

func Play(t *testing.T, clientset client, play expect.Script, namespace string, matcher func(*v1.ConfigMap) (bool, bool)) *expect.Client {
	t.Helper()
	return expect.Play(t, play, func(ctx context.Context, client *expect.Client) {
		sync := NewFromInterface(clientset.CoreV1(), client, matcher)
		sync.RunContext(ctx, namespace)
	})
}

type errString string

func (err errString) Error() string {
	return string(err)
}

// Test Annotations when loading a CM fails
func testErrorAnnotation(t *testing.T, w scriptWriter, config testConfig, annotation string) {
	labeledCM := config.Label(w.ConfigMap())
	clientset := newFakeClientset(labeledCM)
	play := w.OnAddition(func() error { return errString("test error!") })
	Play(t, clientset, play, labeledCM.Namespace, config.Matcher())
	clientset.MustBeAnnotated(t, labeledCM, annotation, `{"status":"error","error":"test error!"}`)
}

func testMatcher(t *testing.T, configs []testConfig, cm *v1.ConfigMap, policy bool) []testConfig {
	type outcome struct {
		MatchUnlabeled bool
		MatchLabeled   bool
	}
	selectedOutcomes := make(map[outcome]struct{})
	processTests := make([]testConfig, 0, 4)

	matcherFails := make([]string, 1, len(configs)+1)
	for index, config := range configs {

		// Test the matcher first
		matcher := config.Matcher()
		if m, isPolicy := matcher(cm); m != config.MatchNoLabel() || (m && (isPolicy != policy)) {
			matcherFails = append(matcherFails, fmt.Sprintf("Matcher [%d:%s] failed for unlabeled configmap: expected %v, got %v (isPolicy: %v)",
				index, config.Title(),
				config.MatchNoLabel(),
				m, isPolicy))
		}
		if m, isPolicy := matcher(config.Label(cm)); m != config.MatchLabel() || (m && (isPolicy != policy)) {
			matcherFails = append(matcherFails, fmt.Sprintf("Matcher [%d:%s] failed for labeled configmap: expected %v, got %v (isPolicy: %v)",
				index, config.Title(),
				config.MatchLabel(),
				m, isPolicy))
		}

		// Select only one use case per outcome for the process tests.
		// coverage should be roughly the same, and tests will run much faster
		current := outcome{config.MatchNoLabel(), config.MatchLabel()}
		if _, ok := selectedOutcomes[current]; !ok {
			selectedOutcomes[current] = struct{}{}
			processTests = append(processTests, config)
		}
	}
	if len(matcherFails) > 1 {
		t.Fatal(strings.Join(matcherFails, "\n"))
	}
	return processTests
}

func newFakeClientset(cm *v1.ConfigMap) client {
	if cm == nil {
		return client{fake.NewSimpleClientset().CoreV1()}
	}
	return client{fake.NewSimpleClientset(cm).CoreV1()}
}

// CoreV1 Unwraps the inner interface
func (c client) CoreV1() typev1.CoreV1Interface {
	return c.configmapInterface.(typev1.CoreV1Interface)
}

// MustBeAnnotated checks that the CM is annotated
func (c client) MustBeAnnotated(t *testing.T, cm *v1.ConfigMap, key string, annotation string) {
	t.Helper()
	newcm, err := c.Get(context.TODO(), cm.Namespace, cm.Name)
	if err != nil {
		t.Fatalf("Failed to get ConfigMap %s: %v", cm.Name, err)
	}
	var hasAnnotation string
	if newcm.Annotations != nil {
		if ann, ok := newcm.Annotations[key]; ok {
			hasAnnotation = ann
		}
	}
	if hasAnnotation == "" {
		t.Fatal("ConfigMap is not annotated")
	}
	if hasAnnotation != annotation {
		t.Fatalf("Expected annotation %s, got %s", annotation, hasAnnotation)
	}
}

// MustUpdate returns an Action that will update a ConfigMap
func (c client) MustUpdate(t *testing.T, cm *v1.ConfigMap) expect.Action {
	t.Helper()
	return func() error {
		if err := c.Update(context.TODO(), cm); err != nil {
			t.Fatalf("Failed to update ConfigMap %s: %v", cm.Name, err)
		}
		return nil
	}
}

// MustCreate returns an Action that will create a ConfigMap
func (c client) MustCreate(t *testing.T, cm *v1.ConfigMap) expect.Action {
	t.Helper()
	return func() error {
		if err := c.Create(context.TODO(), cm); err != nil {
			t.Fatalf("Failed to create ConfigMap %s: %v", cm.Name, err)
		}
		return nil
	}
}

// MustDelete returns an action that will delete a ConfigMap
func (c client) MustDelete(t *testing.T, cm *v1.ConfigMap) expect.Action {
	t.Helper()
	return func() error {
		if err := c.Delete(context.TODO(), cm.Namespace, cm.Name); err != nil {
			t.Fatalf("Failed to delete ConfigMap %s: %v", cm.Name, err)
		}
		return nil
	}
}
