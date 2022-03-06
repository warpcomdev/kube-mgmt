// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
)

// GenericSync replicates Kubernetes resources into OPA as raw JSON.
type GenericSync struct {
	client        dynamic.Interface
	opa           opa_client.Data
	ns            types.ResourceType
	loadCompleted bool
}

// The min/max amount of time to wait when resetting the synchronizer.
const (
	backoffMax = time.Second * 30
	backoffMin = time.Second
)

// New returns a new GenericSync that can be started.
func New(client dynamic.Interface, opa opa_client.Data, ns types.ResourceType) *GenericSync {
	return &GenericSync{
		client: client,
		ns:     ns,
		opa:    opa.Prefix(ns.Resource),
	}
}

// Run starts the synchronizer. To stop the synchronizer,
// cancel the context.
func (s *GenericSync) Run(ctx context.Context) {

	logrus.Infof("Syncing %v.", s.ns)
	defer func() {
		logrus.Infof("Sync for %v finished. Exiting.", s.ns)
	}()

	queue := workqueue.New()
	quit := ctx.Done()
	go func() {
		<-quit
		queue.ShutDown()
	}()

	baseResource := s.client.Resource(schema.GroupVersionResource{
		Group:    s.ns.Group,
		Version:  s.ns.Version,
		Resource: s.ns.Resource,
	})
	var resource dynamic.ResourceInterface = baseResource
	if s.ns.Namespaced {
		resource = baseResource.Namespace(metav1.NamespaceAll)
	}
	start := time.Now()
	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return resource.List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return resource.Watch(ctx, options)
			},
		},
		&unstructured.Unstructured{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
					var _ string = key // In case kubernetes API stops using strings as cache keys in the future...
					queue.Add(key)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				if key, err := cache.MetaNamespaceKeyFunc(newObj); err != nil {
					queue.Add(key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
					var _ string = key
					queue.Add(key)
				}
			},
		},
	)

	go controller.Run(quit)
	for !cache.WaitForCacheSync(quit, controller.HasSynced) {
		select {
		case <-quit:
			return
		default:
			logrus.Warn("Failed to sync cache, retrying...")
		}
	}
	logrus.Infof("Initial informer sync for %v completed, took %v", s.ns, time.Since(start))

	delay := backoffMin
	for {
		err := s.sync(store, queue)
		if err == nil {
			return
		}

		var errOPA *opa_client.Error
		if errors.As(err, &errOPA) {
			delay = backoffMin
			logrus.Errorf("Sync for %v failed due to OPA error. Trying again in %v. Reason: %v", s.ns, delay, err)
		} else {
			delay *= 2
			if delay > backoffMax {
				delay = backoffMax
			}
			logrus.Errorf("Sync for %v failed due to queue error. Trying again in %v. Reason: %v", s.ns, delay, err)
		}
		t := time.NewTimer(delay)
		select {
		case <-t.C: // Nop
		case <-quit:
			return
		}
	}
}

const initPath = ""

// sync starts replicating Kubernetes resources into OPA. If an error occurs
// during the replication process this function returns and indicates whether
// the synchronizer should backoff. The synchronizer will backoff whenever the
// Kubernetes API returns an error.
func (s *GenericSync) sync(store cache.Store, queue workqueue.Interface) error {
	s.loadCompleted = false
	queue.Add(initPath) // this special path will trigger the initial load
	for !queue.ShuttingDown() {
		key, shuttingDown := queue.Get()
		if shuttingDown {
			return nil
		}
		err := s.processNext(store, key)
		queue.Done(key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *GenericSync) processNext(store cache.Store, key interface{}) error {
	path, err := objPath(key, s.ns.Namespaced)
	if err != nil {
		return err
	}
	// On receiving the initPath, load a full dump of the data store
	if path == initPath {
		start, list := time.Now(), store.List()
		if err := s.syncAll(list); err != nil {
			return err
		}
		logrus.Infof("Loaded %d resources of kind %v into OPA. Took %v", len(list), s.ns, time.Since(start))
		s.loadCompleted = true
		return nil
	}
	// Ignore updates queued before the initial load
	if !s.loadCompleted {
		return nil
	}
	obj, exists, err := store.Get(key)
	if err != nil {
		return fmt.Errorf("store error: %w", err)
	}
	if exists {
		if err := s.syncAdd(path, obj); err != nil {
			return fmt.Errorf("add event: %w", err)
		}
	} else {
		if err := s.syncRemove(path); err != nil {
			return fmt.Errorf("delete event: %w", err)
		}
	}
	return nil
}

func (s *GenericSync) syncAdd(path string, obj interface{}) error {
	return s.opa.PutData(path, obj)
}

func (s *GenericSync) syncRemove(path string) error {
	return s.opa.PatchData(path, "remove", nil)
}

func (s *GenericSync) syncAll(objs []interface{}) error {

	// Build a list of patches to apply.
	payload, err := generateSyncPayload(objs, s.ns.Namespaced)
	if err != nil {
		return err
	}

	return s.opa.PutData("/", payload)
}

func generateSyncPayload(objs []interface{}, namespaced bool) (map[string]interface{}, error) {
	combined := make(map[string]interface{}, len(objs))
	for _, obj := range objs {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			return nil, err
		}
		path, err := objPath(key, namespaced)
		if err != nil {
			return nil, err
		}

		// Ensure the path in the map up to our value exists
		// We make some assumptions about the paths that do exist
		// being the correct types due to the expected uniform
		// objPath's for each of the similar object types being
		// sync'd with the GenericSync instance.
		segments := strings.Split(path, "/")
		dir := combined
		for i := 0; i < len(segments)-1; i++ {
			next, ok := combined[segments[i]]
			if !ok {
				next = map[string]interface{}{}
				dir[segments[i]] = next
			}
			dir = next.(map[string]interface{})
		}
		dir[segments[len(segments)-1]] = obj
	}

	return combined, nil
}

// objPath transforms queue key into OPA path
func objPath(key interface{}, namespaced bool) (string, error) {
	// OPA keys actually match current kubernetes cache key format
	// ("namespace/name" or "name" if resource is not namespaced)
	return key.(string), nil
}
