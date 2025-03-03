package controllers

import (
	"context"
	"fmt"
	miniov1alpha1 "minio-operator/api/v1alpha1"

	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"

	lctval "github.com/duke-git/lancet/v2/validator"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MinIOStatusReconciler reconciles a MinIO Status object
type MinIOStatusReconciler struct {
	client.Client
	KubeClient *kubernetes.Clientset
	Scheme     *runtime.Scheme

	Recorder record.EventRecorder
}

func (r *MinIOStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var minio miniov1alpha1.MinIO
	if err := r.Get(ctx, req.NamespacedName, &minio); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// 设置 PVC 状态
	lOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", miniov1alpha1.MinIOLable, minio.Name),
	}
	pvcList, err := r.KubeClient.CoreV1().PersistentVolumeClaims(minio.Namespace).List(ctx, lOpts)
	if err != nil {
		klog.Errorf("query PVC list error, %s", err)
		return ctrl.Result{Requeue: true}, err
	}
	var pvcStatus []miniov1alpha1.PVCStatus
	for _, pvc := range pvcList.Items {
		ps := miniov1alpha1.PVCStatus{
			Name:     pvc.Name,
			Capacity: pvc.Status.Capacity.Storage().String(),
			Status:   string(pvc.Status.Phase),
			Volume:   pvc.Spec.VolumeName,
		}
		if pvc.Spec.StorageClassName != nil {
			ps.StorageClass = *pvc.Spec.StorageClassName
		}
		pvcStatus = append(pvcStatus, ps)
	}
	minio.Status.PVCStatus = pvcStatus

	// 设置 Service 状态
	svcList, err := r.KubeClient.CoreV1().Services(minio.Namespace).List(ctx, lOpts)
	if err != nil {
		klog.Errorf("query Service list error, %s", err)
		return ctrl.Result{Requeue: true}, err
	}
	var svcStatus miniov1alpha1.MinIOServiceAddr
	for _, svc := range svcList.Items {
		if svc.Name == minio.MinIOCIServiceName() {
			switch svc.Spec.Type {
			case corev1.ServiceTypeNodePort:
				svcStatus.MinIO = fmt.Sprintf("http://%s:%s",
					r.nodeIP(),
					fmt.Sprint(svc.Spec.Ports[0].NodePort))
			case corev1.ServiceTypeClusterIP:
				svcStatus.MinIO = fmt.Sprintf("http://%s.%s.svc.%s:%s",
					svc.Name,
					svc.Namespace,
					miniov1alpha1.GetClusterDomain(),
					fmt.Sprint(svc.Spec.Ports[0].Port))
			}
		}
		if svc.Name == minio.MinIOConsoleServiceName() {
			switch svc.Spec.Type {
			case corev1.ServiceTypeNodePort:
				svcStatus.Console = fmt.Sprintf("http://%s:%s",
					r.nodeIP(),
					fmt.Sprint(svc.Spec.Ports[0].NodePort))
			case corev1.ServiceTypeClusterIP:
				svcStatus.Console = fmt.Sprintf("http://%s.%s.svc.%s:%s",
					svc.Name,
					svc.Namespace,
					miniov1alpha1.GetClusterDomain(),
					fmt.Sprint(svc.Spec.Ports[0].Port))
			}
		}
	}
	minio.Status.Service = svcStatus

	// 设置 Pool 状态
	var poolStatus []miniov1alpha1.PoolStatus
	for _, pool := range minio.Spec.Pools {

		listOpts := metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
			miniov1alpha1.MinIOLable, minio.Name, miniov1alpha1.PoolLabel, pool.Name),
		}
		podList, err := r.KubeClient.CoreV1().Pods(minio.Namespace).List(context.TODO(), listOpts)
		if err != nil {
			klog.Errorf("query Service list error, %s", err)
			return ctrl.Result{Requeue: true}, err
		}

		var servers []miniov1alpha1.MinIOServer
		var availableReplicas int
		var poolDeployStatus miniov1alpha1.PoolDeployStatus
		for _, pod := range podList.Items {
			server := miniov1alpha1.MinIOServer{
				Name:   pod.Name,
				HostIP: pod.Status.HostIP,
				PodIP:  pod.Status.PodIP,
				Status: string(pod.Status.Phase),
			}
			if pod.Status.Phase == corev1.PodRunning {
				availableReplicas++
				poolDeployStatus = miniov1alpha1.PoolStatusCompleted
			} else if pod.Status.Phase == corev1.PodPending {
				poolDeployStatus = miniov1alpha1.PoolStatusRunning
			} else {
				poolDeployStatus = miniov1alpha1.PoolStatusFailed
			}
			servers = append(servers, server)
		}
		ps := miniov1alpha1.PoolStatus{
			Name:              pool.Name,
			Status:            poolDeployStatus,
			AvailableReplicas: availableReplicas,
			Replicas:          pool.Servers,
			Servers:           servers,
		}
		poolStatus = append(poolStatus, ps)
	}
	minio.Status.PoolStatus = poolStatus

	// 设置部署状态
	deployStatus := miniov1alpha1.DeployStatusCompleted
	for _, ps := range minio.Status.PoolStatus {
		if ps.Status == miniov1alpha1.PoolStatusFailed {
			deployStatus = miniov1alpha1.DeployStatusFailed
		} else if ps.Status == miniov1alpha1.PoolStatusRunning {
			deployStatus = miniov1alpha1.DeployStatusRunning
		}
	}
	minio.Status.Status = deployStatus

	if err := updateMinIOStatus(ctx, r.Client, &minio); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if minio.Status.Status != miniov1alpha1.DeployStatusCompleted {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// func (r *MinIOStatusReconciler) updateMinIOStatus(ctx context.Context, minio *miniov1alpha1.MinIO) error {
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

func updateMinIOStatus(ctx context.Context, cc client.Client, minio *miniov1alpha1.MinIO) error {
	minioCopy := minio.DeepCopy()
	minioCopy.Spec = miniov1alpha1.MinIOSpec{}
	minioCopy.Status = minio.Status

	if err := cc.Status().Update(ctx, minioCopy); err != nil {
		if errors.IsConflict(err) {
			klog.Infof("Hit conflict issue, getting latest version of MinIO %s", minio.Name)
			err = cc.Get(ctx, client.ObjectKeyFromObject(minio), minioCopy)
			if err != nil {
				return err
			}
			return updateMinIOStatus(ctx, cc, minioCopy)
		}
		return err
	}
	return nil
}

func (r *MinIOStatusReconciler) nodeIP() string {
	nodeList, err := r.KubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("query node list error, %s", err)
		return ""
	}
	for _, node := range nodeList.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP && lctval.IsIpV4(addr.Address) {
				return addr.Address
			}
		}
	}
	return ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinIOStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.MinIO{}).
		For(&corev1.Pod{}).
		For(&corev1.Service{}).
		// For(&corev1.PersistentVolumeClaim{}).
		// For(&corev1.Secret{}).
		Complete(r)
}
