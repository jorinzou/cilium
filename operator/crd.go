// Copyright 2020 Authors of Cilium
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	toolswatch "k8s.io/client-go/tools/watch"
)

const (
	defaultTimeout = 2 * time.Minute
)

// waitForCRD waits for the given CRD to be available until the given timeout.
// Returns an error when timeout exceeded.
func waitForCRD(client clientset.Interface, name string, timeout time.Duration) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	selector := fields.OneTermEqualSelector("metadata.name", name).String()
	w := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = selector
			return client.ApiextensionsV1beta1().CustomResourceDefinitions().List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = selector
			return client.ApiextensionsV1beta1().CustomResourceDefinitions().Watch(ctx, options)
		},
	}
	cond := func(ev watch.Event) (bool, error) {
		if crd, ok := ev.Object.(*v1beta1.CustomResourceDefinition); ok {
			// NOTE(mrostecki): Why is it done here despite having a field
			// selector above? Fake client doesn't support field selectors,
			// so the fake watcher always returns all CRDs created...
			// kubernetes/client-go#326
			// Doing that one comparison doesn't hurt and it makes unit
			// testing possible.
			if crd.ObjectMeta.Name == name {
				return true, nil
			}
			return false, errors.New("CRD not found")
		}
		return false, ErrInvalidTypeCRD
	}
	ev, err := toolswatch.UntilWithSync(ctx, w, &v1beta1.CustomResourceDefinition{}, nil, cond)
	if err != nil {
		return errors.Wrapf(err, "timeout waiting for CRD %s", name)
	}
	if _, ok := ev.Object.(*v1beta1.CustomResourceDefinition); ok {
		return nil
	}
	return ErrInvalidTypeCRD
}

// WaitForCRD waits for the given CRD to be available until the default timeout,
// after which cilium-agent should be ready. Returns an error when timeut
// esceeded.
func WaitForCRD(client clientset.Interface, name string) error {
	return waitForCRD(client, name, defaultTimeout)
}
