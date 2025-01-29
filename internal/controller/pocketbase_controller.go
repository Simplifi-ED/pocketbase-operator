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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	baasv1 "pb.simplified/controller/api/v1"
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

	// Create PVC
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
	if err := r.Create(ctx, pvc); err != nil && !errors.IsAlreadyExists(err) {
		logger.Error(err, "Failed to create PVC")
		return ctrl.Result{}, err
	}
	logger.Info("PVC created successfully")

	// Create ConfigMap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.Name + "-config",
			Namespace: pb.Namespace,
			Labels:    labelsForPocketbase(pb),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pb, baasv1.GroupVersion.WithKind("Pocketbase")),
			},
		},
		Data: map[string]string{}, // Add your config data here
	}
	if err := r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
		logger.Error(err, "Failed to create ConfigMap")
		return ctrl.Result{}, err
	}
	logger.Info("ConfigMap created successfully")

	// Create Service
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
	if err := r.Create(ctx, svc); err != nil && !errors.IsAlreadyExists(err) {
		logger.Error(err, "Failed to create Service")
		return ctrl.Result{}, err
	}
	logger.Info("Service created successfully")

	// Create Deployment
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
	if err := r.Create(ctx, deploy); err != nil && !errors.IsAlreadyExists(err) {
		logger.Error(err, "Failed to create Deployment")
		return ctrl.Result{}, err
	}
	logger.Info("Deployment created successfully")

	return ctrl.Result{}, nil
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
