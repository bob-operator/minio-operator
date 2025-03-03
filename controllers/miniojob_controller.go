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
	"minio-operator/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/klog/v2"

	miniov1alpha1 "minio-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MinIOJobReconciler reconciles a MinIOJob object
type MinIOJobReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	KubeClient *kubernetes.Clientset
}

// +kubebuilder:rbac:groups=minio.bob.com,resources=miniojobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minio.bob.com,resources=miniojobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=minio.bob.com,resources=miniojobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinIOJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *MinIOJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mJob miniov1alpha1.MinIOJob
	if err := r.Get(ctx, req.NamespacedName, &mJob); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	}

	// 查询 MinIO 实例
	var minio miniov1alpha1.MinIO
	minio.Namespace = mJob.Namespace
	minio.Name = mJob.Spec.MinIORef
	if err := r.Get(ctx, client.ObjectKeyFromObject(&minio), &minio); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	}

	// 确保部署状态为 Completed 才能重启服务
	if minio.Status.Status != miniov1alpha1.DeployStatusCompleted {
		klog.Infof("MinIO %s/%s not deploy completed", minio.Namespace, minio.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	minioConfiguration, err := r.getMinIOCredentials(ctx, &minio)
	if err != nil {
		klog.Errorf("get MinIO %s/%s credentials failed, %s", minio.Namespace, minio.Name, err)
		return ctrl.Result{}, err
	}

	adminClnt, err := minio.NewMinIOAdmin(minioConfiguration, utils.CreateTransport())
	if err != nil {
		klog.Errorf("create MinIO %s/%s admin client failed, %s", minio.Namespace, minio.Name, err)
		return ctrl.Result{}, err
	}

	// 重启服务,重启完成后要删除 MinIOJob 实例
	restartCount := minio.Status.RestartCount
	if mJob.Spec.Action == miniov1alpha1.RestartMinIOJobAction {
		if err := adminClnt.ServiceRestart(ctx); err != nil {
			klog.Errorf("restart service for MinIO %s/%s error, %s", minio.Namespace, minio.Name, err)
			return ctrl.Result{Requeue: true}, err
		} else {
			restartCount++
			minio.Status.RestartCount = restartCount
			if err := updateMinIOStatus(ctx, r.Client, &minio); err != nil {
				return ctrl.Result{}, err
			}
			// 删除 MinIOJob 实例
			if err := r.Delete(ctx, &mJob); err != nil {
				klog.Errorf("delete MinIOJob %s/%s error, %s", mJob.Namespace, mJob.Name, err)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// 合并 env 和 config.env 中的配置的环境变量
func (r *MinIOJobReconciler) getMinIOCredentials(ctx context.Context, minio *miniov1alpha1.MinIO) (map[string][]byte, error) {
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
func (r *MinIOJobReconciler) getMinIOConfiguration(ctx context.Context, minio *miniov1alpha1.MinIO) (map[string][]byte, error) {
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
func (r *MinIOJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.MinIOJob{}).
		Complete(r)
}
