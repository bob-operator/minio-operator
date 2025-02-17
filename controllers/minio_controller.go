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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/record"

	miniov1alpha1 "minio-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MinIOReconciler reconciles a MinIO object
type MinIOReconciler struct {
	client.Client
	KubeClient *kubernetes.Clientset
	Scheme     *runtime.Scheme

	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=minio.bob.com,resources=minios,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minio.bob.com,resources=minios/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=minio.bob.com,resources=minios/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinIO object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *MinIOReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var minio miniov1alpha1.MinIO
	if err := r.Get(ctx, req.NamespacedName, &minio); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinIOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.MinIO{}).
		For(&corev1.Pod{}).
		For(&corev1.Service{}).
		Complete(r)
}
