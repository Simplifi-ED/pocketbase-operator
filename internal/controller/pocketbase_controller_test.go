/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	baasv1 "pb.simplified/controller/api/v1"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("Pocketbase Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		var pocketbase *baasv1.Pocketbase
		var reconciler *PocketbaseReconciler

		BeforeEach(func() {
			pocketbase = &baasv1.Pocketbase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: baasv1.PocketbaseSpec{
					Name:  resourceName,
					Image: "pocketbase/pocketbase:latest",
					Volumes: baasv1.VolumeConfig{
						StorageClassName: "standard",
						StorageSize:      "1Gi",
						AccessModes:      []string{"ReadWriteOnce"},
						VolumeMountPath:  "/pb_data",
					},
				},
			}

			reconciler = &PocketbaseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			Expect(k8sClient.Create(ctx, pocketbase)).To(Succeed())

			// Wait for initial reconciliation
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, pocketbase)
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, pocketbase)).To(Succeed())

			// Verify resource is actually deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, &baasv1.Pocketbase{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should successfully reconcile the resource", func() {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify finalizer is added
			var pb baasv1.Pocketbase
			Expect(k8sClient.Get(ctx, typeNamespacedName, &pb)).To(Succeed())
			Expect(pb.Finalizers).To(ContainElement(finalizerName))
		})

		It("should handle deletion correctly", func() {
			// First ensure resource exists with finalizer
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Trigger deletion
			Expect(k8sClient.Delete(ctx, pocketbase)).To(Succeed())

			// Verify deletion handling
			Eventually(func() error {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				return err
			}, timeout, interval).Should(Not(HaveOccurred()))

			// Verify resource is gone
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, &baasv1.Pocketbase{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
