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
	stderr "errors"
	"fmt"
	"minio-operator/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/record"

	miniov1alpha1 "minio-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrEmptyRootCredentials = stderr.New("empty tenant credentials")
)

const (
	OPNamespace = "operator"
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
	// cOpts := metav1.CreateOptions{}
	// uOpts := metav1.UpdateOptions{}

	var minio miniov1alpha1.MinIO
	if err := r.Get(ctx, req.NamespacedName, &minio); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	// 忽略删除中的资源
	if !minio.DeletionTimestamp.IsZero() {
		utilruntime.HandleError(fmt.Errorf("MinIO '%s' is marked for deletion, skipping", minio.GetName()))
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MinIOReconciler) updateMinIOStatus(ctx context.Context, minio *miniov1alpha1.MinIO, currentState string, availableReplicas int32) (*miniov1alpha1.MinIO, error) {
	return r.updateMinIOStatusWithRetry(ctx, minio, currentState, availableReplicas, true)
}

func (r *MinIOReconciler) updateMinIOStatusWithRetry(ctx context.Context, minio *miniov1alpha1.MinIO, currentState string, availableReplicas int32, retry bool) (*miniov1alpha1.MinIO, error) {
	// if tenant.Status.CurrentState == currentState && tenant.Status.AvailableReplicas == availableReplicas {
	// 	return tenant, nil
	// }
	//
	// tenantCopy := tenant.DeepCopy()
	// tenantCopy.Spec = miniov2.TenantSpec{}
	// tenantCopy.Status.AvailableReplicas = availableReplicas
	// tenantCopy.Status.CurrentState = currentState
	//
	// opts := metav1.UpdateOptions{}
	// t, err := c.minioClientSet.MinioV2().Tenants(tenant.Namespace).UpdateStatus(ctx, tenantCopy, opts)
	// t.EnsureDefaults()
	// if err != nil {
	// 	// if rejected due to conflict, get the latest tenant and retry once
	// 	if k8serrors.IsConflict(err) && retry {
	// 		klog.Info("Hit conflict issue, getting latest version of tenant")
	// 		tenant, err = c.minioClientSet.MinioV2().Tenants(tenant.Namespace).Get(ctx, tenant.Name, metav1.GetOptions{})
	// 		if err != nil {
	// 			return tenant, err
	// 		}
	// 		return c.updateTenantStatusWithRetry(ctx, tenant, currentState, availableReplicas, false)
	// 	}
	// 	return t, err
	// }
	// return t, nil
	return nil, nil
}

// checkMinIOSvc validates the existence of the MinIO service and validate it's status against what the specification
// states
// func (r *MinIOReconciler) checkMinIOSvc(ctx context.Context, minio *miniov1alpha1.MinIO, nsName types.NamespacedName) error {
// 	// Handle the Internal ClusterIP Service for Tenant
// 	// svc, err := r.KubeClient.Services(tenant.Namespace).Get(tenant.MinIOCIServiceName())
// 	svc, err := r.KubeClient.CoreV1().Services(minio.Namespace).Get(ctx)
// 	if err != nil {
// 		if errors.IsNotFound(err) {
// 			if tenant, err = c.updateTenantStatus(ctx, tenant, StatusProvisioningCIService, 0); err != nil {
// 				return err
// 			}
// 			klog.V(2).Infof("Creating a new Cluster IP Service for cluster %q", nsName)
// 			// Create the clusterIP service for the Tenant
// 			svc = services.NewClusterIPForMinIO(tenant)
// 			svc, err = c.kubeClientSet.CoreV1().Services(tenant.Namespace).Create(ctx, svc, metav1.CreateOptions{})
// 			if err != nil {
// 				return err
// 			}
// 			c.recorder.Event(tenant, corev1.EventTypeNormal, "SvcCreated", "MinIO Service Created")
// 		} else {
// 			return err
// 		}
// 	}
//
// 	// compare any other change from what is specified on the tenant, since some of the state of the service is saved
// 	// on the service.spec we will compare individual parts
// 	expectedSvc := services.NewClusterIPForMinIO(tenant)
//
// 	// check the expose status of the MinIO ClusterIP service
// 	minioSvcMatchesSpec, err := minioSvcMatchesSpecification(svc, expectedSvc)
//
// 	// check the specification of the MinIO ClusterIP service
// 	if !minioSvcMatchesSpec {
// 		if err != nil {
// 			klog.Infof("MinIO Services don't match: %s", err)
// 		}
//
// 		svc.ObjectMeta.Annotations = expectedSvc.ObjectMeta.Annotations
// 		svc.ObjectMeta.Labels = expectedSvc.ObjectMeta.Labels
// 		svc.Spec.Ports = expectedSvc.Spec.Ports
//
// 		// Only when ExposeServices is set an explicit value we do modifications to the service type
// 		if tenant.Spec.ExposeServices != nil {
// 			if tenant.Spec.ExposeServices.MinIO {
// 				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
// 			} else {
// 				svc.Spec.Type = corev1.ServiceTypeClusterIP
// 			}
// 		}
//
// 		// update the selector
// 		svc.Spec.Selector = expectedSvc.Spec.Selector
//
// 		_, err = c.kubeClientSet.CoreV1().Services(tenant.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
// 		if err != nil {
// 			return err
// 		}
// 		c.recorder.Event(tenant, corev1.EventTypeNormal, "Updated", "MinIO Service Updated")
// 	}
// 	return err
// }

// 合并 env 和 config.env 中的配置的环境变量
func (r *MinIOReconciler) getMinIOCredentials(ctx context.Context, minio *miniov1alpha1.MinIO) (map[string][]byte, error) {
	minioConfiguration := map[string][]byte{}

	// 查询 env 中的配置的环境变量
	for _, config := range minio.GetEnvVars() {
		minioConfiguration[config.Name] = []byte(config.Value)
	}

	// 加载 config.env 中的配置的环境变量
	config, err := r.getMinIOConfiguration(ctx, minio)
	if err != nil {
		return nil, err
	}
	for key, val := range config {
		minioConfiguration[key] = val
	}

	var accessKey string
	var secretKey string

	if _, ok := minioConfiguration["accesskey"]; ok {
		accessKey = string(minioConfiguration["accesskey"])
	}

	if _, ok := minioConfiguration["secretkey"]; ok {
		secretKey = string(minioConfiguration["secretkey"])
	}

	if accessKey == "" || secretKey == "" {
		return minioConfiguration, ErrEmptyRootCredentials
	}

	return minioConfiguration, nil
}

// 查询 config.env 中的配置的环境变量
func (r *MinIOReconciler) getMinIOConfiguration(ctx context.Context, minio *miniov1alpha1.MinIO) (map[string][]byte, error) {
	config := map[string][]byte{}
	// Load tenant configuration from file
	if minio.HasConfigurationSecret() {
		minioConfigurationSecretName := minio.Spec.Configuration.Name
		minioConfigurationSecret, err := r.KubeClient.CoreV1().Secrets(minio.Namespace).Get(ctx, minioConfigurationSecretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		configFromFile := utils.ParseRawConfiguration(minioConfigurationSecret.Data["config.env"])
		for key, val := range configFromFile {
			config[key] = val
		}
	}
	return config, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinIOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.MinIO{}).
		For(&corev1.Pod{}).
		For(&corev1.Service{}).
		For(&corev1.PersistentVolumeClaim{}).
		For(&corev1.Secret{}).
		Complete(r)
}
