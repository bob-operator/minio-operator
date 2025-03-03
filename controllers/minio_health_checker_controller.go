package controllers

import (
	"context"
	"crypto/tls"
	miniov1alpha1 "minio-operator/api/v1alpha1"
	"net"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

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
	if err := r.updateMinIOStatus(ctx, &minio); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// 只能在部署完成后，MinIO 服务 pod 处于 Running 状态才能进行健康检查
	if minio.Status.Status != miniov1alpha1.DeployStatusCompleted {
		return ctrl.Result{Requeue: true}, nil
	}

	var healthStatus miniov1alpha1.HealthStatus
	if minio.MinIOHealthCheck(r.createTransport()) {
		healthStatus = miniov1alpha1.HealthStatusHealth
	} else {
		healthStatus = miniov1alpha1.HealthStatusUnHealth
	}
	minio.Status.HealthStatus = healthStatus
	if err := r.updateMinIOStatus(ctx, &minio); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if minio.Status.HealthStatus != miniov1alpha1.HealthStatusHealth {
		time.Sleep(time.Second)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MinIOHealthCheckerReconciler) updateMinIOStatus(ctx context.Context, minio *miniov1alpha1.MinIO) error {
	// if minio.Status.Status == miniov1alpha1.DeployStatusCompleted && minio.Status.AvailableReplicas == minio.Spec.Servers {
	// 	return nil
	// }

	minioCopy := minio.DeepCopy()
	minioCopy.Spec = miniov1alpha1.MinIOSpec{}
	minioCopy.Status = minio.Status

	if err := r.Status().Update(ctx, minioCopy); err != nil {
		if errors.IsConflict(err) {
			klog.Infof("Hit conflict issue, getting latest version of MinIO %s", minio.Name)
			err = r.Get(ctx, client.ObjectKeyFromObject(minio), minioCopy)
			if err != nil {
				return err
			}
			return r.updateMinIOStatus(ctx, minioCopy)
		}
		return err
	}
	return nil
}

// 创建 transport
func (r *MinIOHealthCheckerReconciler) createTransport() *http.Transport {
	// rootCAs := c.fetchTransportCACertificates()
	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 15 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConnsPerHost:   1024,
		IdleConnTimeout:       15 * time.Second,
		ResponseHeaderTimeout: 15 * time.Minute,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 15 * time.Second,
		// Go net/http automatically unzip if content-type is
		// gzip disable this feature, as we are always interested
		// in raw stream.
		DisableCompression: true,
		TLSClientConfig: &tls.Config{
			// Can't use SSLv3 because of POODLE and BEAST
			// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
			// Can't use TLSv1.1 because of RC4 cipher usage
			MinVersion: tls.VersionTLS12,
			// RootCAs:    rootCAs,
		},
	}

	return transport
}

// 合并 env 和 config.env 中的配置的环境变量
// func (r *MinIOHealthCheckerReconciler) getMinIOCredentials(ctx context.Context, minio *miniov1alpha1.MinIO) (map[string][]byte, error) {
// 	minioConfiguration := map[string][]byte{}
//
// 	// 查询 env 中的配置的环境变量
// 	for _, config := range minio.GetEnvVars() {
// 		minioConfiguration[config.Name] = []byte(config.Value)
// 	}
//
// 	// 加载 config.env 中的配置的环境变量
// 	config, err := r.getMinIOConfiguration(ctx, minio)
// 	if err != nil {
// 		return nil, err
// 	}
// 	for key, val := range config {
// 		minioConfiguration[key] = val
// 	}
//
// 	var accessKey string
// 	var secretKey string
//
// 	if _, ok := minioConfiguration["accesskey"]; ok {
// 		accessKey = string(minioConfiguration["accesskey"])
// 	}
//
// 	if _, ok := minioConfiguration["secretkey"]; ok {
// 		secretKey = string(minioConfiguration["secretkey"])
// 	}
//
// 	if accessKey == "" || secretKey == "" {
// 		return minioConfiguration, ErrEmptyRootCredentials
// 	}
//
// 	return minioConfiguration, nil
// }
//
// // 查询 config.env 中的配置的环境变量
// func (r *MinIOHealthCheckerReconciler) getMinIOConfiguration(ctx context.Context, minio *miniov1alpha1.MinIO) (map[string][]byte, error) {
// 	config := map[string][]byte{}
// 	// Load tenant configuration from file
// 	if minio.HasConfigurationSecret() {
// 		minioConfigurationSecretName := minio.Spec.Configuration.Name
// 		minioConfigurationSecret, err := r.KubeClient.CoreV1().Secrets(minio.Namespace).Get(ctx, minioConfigurationSecretName, metav1.GetOptions{})
// 		if err != nil {
// 			return nil, err
// 		}
// 		configFromFile := utils.ParseRawConfiguration(minioConfigurationSecret.Data["config.env"])
// 		for key, val := range configFromFile {
// 			config[key] = val
// 		}
// 	}
// 	return config, nil
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
