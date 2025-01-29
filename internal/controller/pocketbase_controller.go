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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	baasv1 "pb.simplified/controller/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	finalizerName = "pocketbase.finalizers.controller.pb.simplified"
)

// PocketbaseReconciler reconciles a Pocketbase object
type PocketbaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Add this to your pocketbase_controller.go
func labelsForPocketbase(pb *baasv1.Pocketbase) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       pb.Name,
		"app.kubernetes.io/instance":   "pocketbase",
		"app.kubernetes.io/component":  "database",
		"app.kubernetes.io/part-of":    "pocketbase-operator",
		"app.kubernetes.io/created-by": "pocketbase-controller",
	}
}

// +kubebuilder:rbac:groups=baas.pb.simplified,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baas.pb.simplified,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baas.pb.simplified,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baas.pb.simplified,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baas.pb.simplified,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baas.pb.simplified,resources=pocketbases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baas.pb.simplified,resources=pocketbases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=baas.pb.simplified,resources=pocketbases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pocketbase object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *PocketbaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling PocketBase instance", "namespace", req.Namespace, "name", req.Name)

	pb := &baasv1.Pocketbase{}
	if err := r.Get(ctx, req.NamespacedName, pb); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PocketBase resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PocketBase resource")
		return ctrl.Result{}, err
	}

	if pb.DeletionTimestamp != nil {
		if contains(pb.Finalizers, finalizerName) {
			if err := r.deleteResources(ctx, pb); err != nil {
				logger.Error(err, "Failed to delete resources")
				return ctrl.Result{}, err
			}
			pb.Finalizers = removeString(pb.Finalizers, finalizerName)
			if err := r.Update(ctx, pb); err != nil {
				logger.Error(err, "Failed to update PocketBase after removing finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !contains(pb.Finalizers, finalizerName) {
		pb.Finalizers = append(pb.Finalizers, finalizerName)
		if err := r.Update(ctx, pb); err != nil {
			logger.Error(err, "Failed to add finalizer to PocketBase")
			return ctrl.Result{}, err
		}
	}

	if err := r.reconcilePVC(ctx, pb); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileConfigMap(ctx, pb); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, pb); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileDeployment(ctx, pb); err != nil {
		return ctrl.Result{}, err
	}
	// if err := r.reconcileIngress(ctx, pb); err != nil {
	// 	return ctrl.Result{}, err
	// }
	return ctrl.Result{}, nil
}

func (r *PocketbaseReconciler) reconcilePVC(ctx context.Context, pb *baasv1.Pocketbase) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.Name,
			Namespace: pb.Namespace,
			Labels:    labelsForPocketbase(pb),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pb, baasv1.GroupVersion.WithKind("Pocketbase")),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &pb.Spec.Volumes.StorageClassName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(pb.Spec.Volumes.AccessModes[0])},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(pb.Spec.Volumes.StorageSize),
				},
			},
		},
	}

	err := r.Get(ctx, types.NamespacedName{Name: pb.Name, Namespace: pb.Namespace}, pvc)
	if err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, pvc); err != nil {
			log.FromContext(ctx).Error(err, "Failed to create PVC")
			return err
		}
		log.FromContext(ctx).Info("PVC created successfully")
		return nil
	} else if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get PVC")
		return err
	}

	if pvc.Spec.StorageClassName != &pb.Spec.Volumes.StorageClassName ||
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] != resource.MustParse(pb.Spec.Volumes.StorageSize) {
		pvc.Spec.StorageClassName = &pb.Spec.Volumes.StorageClassName
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(pb.Spec.Volumes.StorageSize)
		if err := r.Update(ctx, pvc); err != nil {
			log.FromContext(ctx).Error(err, "Failed to update PVC")
			return err
		}
		log.FromContext(ctx).Info("PVC updated successfully")
	}

	return nil
}

func (r *PocketbaseReconciler) reconcileConfigMap(ctx context.Context, pb *baasv1.Pocketbase) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.Name + "-config",
			Namespace: pb.Namespace,
			Labels:    labelsForPocketbase(pb),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pb, baasv1.GroupVersion.WithKind("Pocketbase")),
			},
		},
	}

	err := r.Get(ctx, types.NamespacedName{Name: pb.Name + "-config", Namespace: pb.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		cm.Data = map[string]string{} // Add config data here
		if err := r.Create(ctx, cm); err != nil {
			log.FromContext(ctx).Error(err, "Failed to create ConfigMap")
			return err
		}
		log.FromContext(ctx).Info("ConfigMap created successfully")
	} else if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get ConfigMap")
		return err
	}

	return nil
}

func (r *PocketbaseReconciler) reconcileService(ctx context.Context, pb *baasv1.Pocketbase) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.Name,
			Namespace: pb.Namespace,
			Labels:    labelsForPocketbase(pb),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pb, baasv1.GroupVersion.WithKind("Pocketbase")),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       8090,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
				Name:       "http",
			}},
			Selector: labelsForPocketbase(pb),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	err := r.Get(ctx, types.NamespacedName{Name: pb.Name, Namespace: pb.Namespace}, svc)
	if err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, svc); err != nil {
			log.FromContext(ctx).Error(err, "Failed to create Service")
			return err
		}
		log.FromContext(ctx).Info("Service created successfully")
	} else if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Service")
		return err
	}

	return nil
}
func (r *PocketbaseReconciler) reconcileDeployment(ctx context.Context, pb *baasv1.Pocketbase) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.Name,
			Namespace: pb.Namespace,
			Labels:    labelsForPocketbase(pb),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pb, baasv1.GroupVersion.WithKind("Pocketbase")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForPocketbase(pb),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForPocketbase(pb),
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.To[int64](0),
						RunAsGroup: ptr.To[int64](0),
					},
					Containers: []corev1.Container{{
						Name:  "pocketbase",
						Image: pb.Spec.Image,
						SecurityContext: &corev1.SecurityContext{
							Privileged: ptr.To[bool](true),
						},
						Ports: []corev1.ContainerPort{{
							Name:          "http",
							ContainerPort: 8090,
							Protocol:      corev1.ProtocolTCP,
						}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/api/health",
									Port: intstr.FromString("http"),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/api/health",
									Port: intstr.FromString("http"),
								},
							},
						},
						Resources: pb.Spec.Resources,
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: pb.Name + "-config",
								},
							},
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      pb.Spec.Volumes.VolumeName,
							MountPath: pb.Spec.Volumes.VolumeMountPath,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: pb.Spec.Volumes.VolumeName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pb.Name,
							},
						},
					}},
				},
			},
		},
	}

	err := r.Get(ctx, types.NamespacedName{Name: pb.Name, Namespace: pb.Namespace}, deploy)
	if err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, deploy); err != nil {
			log.FromContext(ctx).Error(err, "Failed to create Deployment")
			return err
		}
		log.FromContext(ctx).Info("Deployment created successfully")
	} else if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Deployment")
		return err
	}

	updateRequired := false
	if deploy.Spec.Template.Spec.Containers[0].Image != pb.Spec.Image {
		deploy.Spec.Template.Spec.Containers[0].Image = pb.Spec.Image
		updateRequired = true
	}

	if updateRequired {
		if err := r.Update(ctx, deploy); err != nil {
			log.FromContext(ctx).Error(err, "Failed to update Deployment")
			return err
		}
		log.FromContext(ctx).Info("Deployment updated successfully")
	} else {
		log.FromContext(ctx).Info("No update required for Deployment")
	}

	return nil
}

// func (r *PocketbaseReconciler) reconcileIngress(ctx context.Context, pb *baasv1.Pocketbase) error {
// 	// Check if hosts are defined
// 	if len(pb.Spec.Ingress.Hosts) == 0 {
// 		return fmt.Errorf("no hosts defined for Ingress %s/%s", pb.Namespace, pb.Name)
// 	}

// 	// Create Ingress resource
// 	ingress := &networkingv1.Ingress{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      pb.Name,
// 			Namespace: pb.Namespace,
// 			Labels:    labelsForPocketbase(pb),
// 			OwnerReferences: []metav1.OwnerReference{
// 				*metav1.NewControllerRef(pb, baasv1.GroupVersion.WithKind("Pocketbase")),
// 			},
// 		},
// 		Spec: networkingv1.IngressSpec{
// 			IngressClassName: &pb.Spec.Ingress.IngressClassName,
// 			Rules: []networkingv1.IngressRule{
// 				{
// 					Host: pb.Spec.Ingress.Hosts[0].Host,
// 					IngressRuleValue: networkingv1.IngressRuleValue{
// 						HTTP: &networkingv1.HTTPIngressRuleValue{
// 							Paths: []networkingv1.HTTPIngressPath{
// 								{
// 									Path:     pb.Spec.Ingress.Hosts[0].Paths[0].Path,
// 									PathType: (*networkingv1.PathType)(&pb.Spec.Ingress.Hosts[0].Paths[0].PathType),
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			TLS: []networkingv1.IngressTLS{
// 				{
// 					Hosts:      []string{pb.Spec.Ingress.Hosts[0].Host},
// 					SecretName: pb.Spec.Ingress.TLS[0].SecretName,
// 				},
// 			},
// 		},
// 	}

// 	// Add annotations if provided
// 	if pb.Spec.Ingress.Annotations != nil {
// 		ingress.Annotations = pb.Spec.Ingress.Annotations
// 	}

// 	// Log ingress details
// 	log.FromContext(ctx).Info("Creating Ingress", "name", pb.Name, "namespace", pb.Namespace)

// 	// Get or create Ingress
// 	err := r.Get(ctx, types.NamespacedName{Name: pb.Name, Namespace: pb.Namespace}, ingress)
// 	if err != nil && errors.IsNotFound(err) {
// 		if err := r.Create(ctx, ingress); err != nil {
// 			log.FromContext(ctx).Error(err, "Failed to create Ingress")
// 			return err
// 		}
// 		log.FromContext(ctx).Info("Ingress created successfully")
// 	} else if err != nil {
// 		log.FromContext(ctx).Error(err, "Failed to get Ingress")
// 		return err
// 	}

// 	// Update status
// 	pb.Status.Ingress = &baasv1.IngressStatus{
// 		Host: ingress.Spec.Rules[0].Host,
// 	}

// 	// Update the Pocketbase status
// 	if err := r.Status().Update(ctx, pb); err != nil {
// 		log.FromContext(ctx).Error(err, "Failed to update Pocketbase status")
// 		return err
// 	}

// 	return nil
// }

func contains(slice []string, item string) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	for i, f := range slice {
		if f == s {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func (r *PocketbaseReconciler) deleteResources(ctx context.Context, pb *baasv1.Pocketbase) error {
	namespacedName := types.NamespacedName{Name: pb.Name, Namespace: pb.Namespace}

	// Delete Deployment
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, namespacedName, deploy); err == nil {
		if err := r.Delete(ctx, deploy); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Deployment: %w", err)
		}
		log.FromContext(ctx).Info("Deployment deleted successfully")
	}

	// Delete Service
	svc := &corev1.Service{}
	if err := r.Get(ctx, namespacedName, svc); err == nil {
		if err := r.Delete(ctx, svc); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Service: %w", err)
		}
		log.FromContext(ctx).Info("Service deleted successfully")
	}

	// Delete ConfigMap
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: pb.Name + "-config", Namespace: pb.Namespace}, cm); err == nil {
		if err := r.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ConfigMap: %w", err)
		}
		log.FromContext(ctx).Info("ConfigMap deleted successfully")
	}

	// Delete PVC
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, namespacedName, pvc); err == nil {
		if err := r.Delete(ctx, pvc); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete PVC: %w", err)
		}
		log.FromContext(ctx).Info("PVC deleted successfully")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PocketbaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&baasv1.Pocketbase{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
	// Named("pocketbase").
}
