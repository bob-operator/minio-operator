package controllers

import (
	"context"
	miniov1alpha1 "minio-operator/api/v1alpha1"
	"minio-operator/utils"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MinIOHealthCheckerReconciler struct {
	client.Client
	KubeClient *kubernetes.Clientset
	Scheme     *runtime.Scheme
}

// 检查 MinIO 服务的健康状态
func (r *MinIOHealthCheckerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var minio miniov1alpha1.MinIO
	if err := r.Get(ctx, req.NamespacedName, &minio); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	minio.Status.HealthStatus = miniov1alpha1.HealthStatusUnknown
	if err := updateMinIOStatus(ctx, r.Client, &minio); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// 只能在部署完成后，MinIO 服务 pod 处于 Running 状态才能进行健康检查
	if minio.Status.Status != miniov1alpha1.DeployStatusCompleted {
		return ctrl.Result{Requeue: true}, nil
	}

	var healthStatus miniov1alpha1.HealthStatus
	if minio.MinIOHealthCheck(utils.CreateTransport()) {
		healthStatus = miniov1alpha1.HealthStatusHealth
	} else {
		healthStatus = miniov1alpha1.HealthStatusUnHealth
	}
	minio.Status.HealthStatus = healthStatus
	if err := updateMinIOStatus(ctx, r.Client, &minio); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if minio.Status.HealthStatus != miniov1alpha1.HealthStatusHealth {
		time.Sleep(time.Second)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// func (r *MinIOHealthCheckerReconciler) updateMinIOStatus(ctx context.Context, minio *miniov1alpha1.MinIO) error {
// 	// if minio.Status.Status == miniov1alpha1.DeployStatusCompleted && minio.Status.AvailableReplicas == minio.Spec.Servers {
// 	// 	return nil
// 	// }
//
// 	minioCopy := minio.DeepCopy()
// 	minioCopy.Spec = miniov1alpha1.MinIOSpec{}
// 	minioCopy.Status = minio.Status
//
// 	if err := r.Status().Update(ctx, minioCopy); err != nil {
// 		if errors.IsConflict(err) {
// 			klog.Infof("Hit conflict issue, getting latest version of MinIO %s", minio.Name)
// 			err = r.Get(ctx, client.ObjectKeyFromObject(minio), minioCopy)
// 			if err != nil {
// 				return err
// 			}
// 			return r.updateMinIOStatus(ctx, minioCopy)
// 		}
// 		return err
// 	}
// 	return nil
// }

func (r *MinIOHealthCheckerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.MinIO{}).
		For(&corev1.Pod{}).
		For(&corev1.Service{}).
		// For(&corev1.PersistentVolumeClaim{}).
		// For(&corev1.Secret{}).
		Complete(r)
}
