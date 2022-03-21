package configmap

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// configmapInterface knows how to call the ConfigMaps method of CoreV1()
type configmapInterface interface {
	ConfigMaps(namespace string) typev1.ConfigMapInterface
}

type client struct {
	configmapInterface
}

// Get a ConfigMap
func (c client) Get(ctx context.Context, namespace, name string) (*v1.ConfigMap, error) {
	return c.ConfigMaps(namespace).Get(
		ctx, name, metav1.GetOptions{},
	)
}

// Update a ConfigMap
func (c client) Update(ctx context.Context, cm *v1.ConfigMap) error {
	_, err := c.ConfigMaps(cm.Namespace).Update(
		ctx, cm, metav1.UpdateOptions{},
	)
	return err
}

// Create a CpnfigMap
func (c client) Create(ctx context.Context, cm *v1.ConfigMap) error {
	_, err := c.ConfigMaps(cm.Namespace).Create(
		ctx, cm, metav1.CreateOptions{},
	)
	return err
}

// Delete a ConfigMap
func (c client) Delete(ctx context.Context, namespace, name string) error {
	return c.ConfigMaps(namespace).Delete(
		ctx, name, metav1.DeleteOptions{},
	)
}

// Annotate a ConfigMap
func (c client) Annotate(ctx context.Context, cm *v1.ConfigMap, key, value string) error {
	if cm.Annotations != nil {
		if existing, ok := cm.Annotations[key]; ok {
			if existing == value {
				// If the annotation did not change, do not write it.
				// (issue https://github.com/open-policy-agent/kube-mgmt/issues/90)
				return nil
			}
		}
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				key: value,
			},
		},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to serialize patch for %v/%v: %w", cm.Namespace, cm.Name, err)
	}
	_, err = c.ConfigMaps(cm.Namespace).Patch(ctx, cm.Name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to annotate %v/%v with %v: %w", cm.Namespace, cm.Name, key, err)
	}
	return nil
}
