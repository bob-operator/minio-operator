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
	"reflect"

	"k8s.io/apimachinery/pkg/util/json"

	"k8s.io/klog/v2"

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

	// 默认为 None 状态
	if minio.Status.Status == "" {
		minio.Status.Status = miniov1alpha1.DeployStatusNone
		err := r.updateMinIOStatusWithRetry(ctx, &minio, true)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// 创建或更新对应的 ControllerRevision
	var oldMinio miniov1alpha1.MinIO
	newCr := minio.NewControllerRevision()
	exist, cr, err := utils.ExistControllerRevision(minio.Name, minio.Namespace, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exist {
		if err := utils.CreateControllerRevision(&minio, newCr, r.Client, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if err := json.Unmarshal(cr.Data.Raw, &oldMinio.Spec); err != nil {
			klog.Errorf("unmarshal ControllerRevision %s/%s error, %s", minio.Namespace, minio.Name, err)
			return ctrl.Result{}, err
		}
		if !reflect.DeepEqual(oldMinio.Spec, minio.Spec) {
			if err := utils.DeleteControllerRevision(cr, r.Client); err != nil {
				return ctrl.Result{}, err
			}
			if err := utils.CreateControllerRevision(&minio, newCr, r.Client, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// 校验是否需要生成或更新 Service
	if err := r.checkMinIOSvc(ctx, &minio); err != nil {
		return ctrl.Result{}, err
	}

	// 校验是否需要生成或更新 Console Service
	if err := r.checkConsoleSvc(ctx, &minio); err != nil {
		return ctrl.Result{}, err
	}

	// 校验是否需要生成或更新 Headless Service
	if err := r.checkMinIOHLSvc(ctx, &minio); err != nil {
		return ctrl.Result{}, err
	}

	// 校验是否需要生成 PVC
	if err := r.checkPVC(ctx, &minio); err != nil {
		return ctrl.Result{}, err
	}

	var addPools []miniov1alpha1.Pool
	var updatePools []miniov1alpha1.Pool

	for _, newPool := range minio.Spec.Pools {
		add := true
		for _, oldPool := range oldMinio.Spec.Pools {
			if newPool.Name == oldPool.Name {
				add = false
				break
			}
		}
		if add {
			addPools = append(addPools, newPool)
		}
	}
	for _, oldPool := range oldMinio.Spec.Pools {
		update := false
		for _, newPool := range minio.Spec.Pools {
			if newPool.Name == oldPool.Name {
				if utils.MinioServerNeedToUpdate(oldMinio, minio, oldPool, newPool) {
					update = true
				}
			}
		}
		if update {
			updatePools = append(updatePools, oldPool)
		}
	}

	var addPods []corev1.Pod
	for _, pool := range addPools {
		pods := utils.NewPodsForMinIOPool(ctx, minio, pool)
		addPods = append(addPods, pods...)
	}
	if len(addPods) > 0 {
		for _, pod := range addPods {
			if _, err := r.KubeClient.CoreV1().Pods(minio.Namespace).Create(ctx, &pod, metav1.CreateOptions{}); err != nil {
				minio.Status.Status = miniov1alpha1.DeployStatusFailed
				minio.Status.Message = fmt.Sprintf("MinIO Pod Create Failed, %s", err.Error())
				if err := r.updateMinIOStatusWithRetry(ctx, &minio, true); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		}
	}

	var updatePods []corev1.Pod
	for _, pool := range updatePools {
		pods := utils.NewPodsForMinIOPool(ctx, minio, pool)
		updatePods = append(updatePods, pods...)
	}
	if len(updatePods) > 0 {
		for _, pod := range updatePods {
			if _, err := r.KubeClient.CoreV1().Pods(minio.Namespace).Update(ctx, &pod, metav1.UpdateOptions{}); err != nil {
				minio.Status.Status = miniov1alpha1.DeployStatusFailed
				minio.Status.Message = fmt.Sprintf("MinIO Pod Update Failed, %s", err.Error())
				if err := r.updateMinIOStatusWithRetry(ctx, &minio, true); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}
		}
	}

	minio.Status.Status = miniov1alpha1.DeployStatusRunning
	if err := r.updateMinIOStatusWithRetry(ctx, &minio, true); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// 更新 MinIO 实例状态
func (r *MinIOReconciler) updateMinIOStatusWithRetry(ctx context.Context, minio *miniov1alpha1.MinIO, retry bool) error {
	// if minio.Status.Status == miniov1alpha1.DeployStatusCompleted && minio.Status.AvailableReplicas == minio.Spec.Servers {
	// 	return nil
	// }

	minioCopy := minio.DeepCopy()
	minioCopy.Spec = miniov1alpha1.MinIOSpec{}
	minioCopy.Status = minio.Status

	if err := r.Status().Update(ctx, minioCopy); err != nil {
		if errors.IsConflict(err) && retry {
			klog.Infof("Hit conflict issue, getting latest version of MinIO %s", minio.Name)
			err = r.Get(ctx, client.ObjectKeyFromObject(minio), minioCopy)
			if err != nil {
				return err
			}
			return r.updateMinIOStatusWithRetry(ctx, minioCopy, retry)
		}
		return err
	}
	return nil
}

// 校验是否需要创建或更新 MinIO Service
func (r *MinIOReconciler) checkMinIOSvc(ctx context.Context, minio *miniov1alpha1.MinIO) error {
	svc, err := r.KubeClient.CoreV1().Services(minio.Namespace).Get(ctx, minio.MinIOCIServiceName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			minio.Status.Status = miniov1alpha1.DeployStatusRunning
			if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
				return err
			}
			klog.V(2).Infof("Creating a new Cluster IP Service %s/%s", minio.Namespace, minio.MinIOCIServiceName())
			svc = utils.NewServiceForMinIO(minio)
			svc, err = r.KubeClient.CoreV1().Services(minio.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			if err != nil {
				minio.Status.Status = miniov1alpha1.DeployStatusFailed
				minio.Status.Message = "MinIO Service Create Failed"
				if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
					return err
				}
				return err
			}
			r.Recorder.Event(minio, corev1.EventTypeNormal, "SvcCreated", "MinIO Service Created")
		} else {
			return err
		}
	}

	// 校验 Service 是否发生更新
	expectedSvc := utils.NewServiceForMinIO(minio)

	isMatch, err := utils.MinioSvcMatchesSpecification(svc, expectedSvc)
	if !isMatch {
		if err != nil {
			klog.Infof("MinIO Services don't match: %s", err)
		}
		svc.Annotations = expectedSvc.Annotations
		svc.Labels = expectedSvc.Labels
		svc.Spec = expectedSvc.Spec

		_, err = r.KubeClient.CoreV1().Services(minio.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			minio.Status.Status = miniov1alpha1.DeployStatusFailed
			minio.Status.Message = "MinIO Service Update Failed"
			if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
				return err
			}
			return err
		}
		r.Recorder.Event(minio, corev1.EventTypeNormal, "SvcUpdated", "MinIO Service Updated")
	}

	return nil
}

// 校验是否需要创建或更新 MinIO Console Service
func (r *MinIOReconciler) checkConsoleSvc(ctx context.Context, minio *miniov1alpha1.MinIO) error {
	svc, err := r.KubeClient.CoreV1().Services(minio.Namespace).Get(ctx, minio.MinIOConsoleServiceName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			minio.Status.Status = miniov1alpha1.DeployStatusRunning
			if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
				return err
			}
			klog.V(2).Infof("Creating a new Console Service %s/%s", minio.Namespace, minio.MinIOConsoleServiceName())
			svc = utils.NewConsoleServiceForMinIO(minio)
			svc, err = r.KubeClient.CoreV1().Services(minio.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			if err != nil {
				minio.Status.Status = miniov1alpha1.DeployStatusFailed
				minio.Status.Message = "MinIO Console Service Create Failed"
				if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
					return err
				}
				return err
			}
			r.Recorder.Event(minio, corev1.EventTypeNormal, "ConsoleSvcCreated", "MinIO Console Service Created")
		} else {
			return err
		}
	}

	expectedSvc := utils.NewConsoleServiceForMinIO(minio)
	isMatch, err := utils.MinioSvcMatchesSpecification(svc, expectedSvc)
	if !isMatch {
		if err != nil {
			klog.Infof("MinIO Console Services don't match: %s", err)
		}
		svc.Annotations = expectedSvc.Annotations
		svc.Labels = expectedSvc.Labels
		svc.Spec = expectedSvc.Spec

		_, err = r.KubeClient.CoreV1().Services(minio.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			minio.Status.Status = miniov1alpha1.DeployStatusFailed
			minio.Status.Message = "MinIO Console Service Created Failed"
			if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
				return err
			}
			return err
		}
		r.Recorder.Event(minio, corev1.EventTypeNormal, "ConsoleSvcUpdated", "MinIO Console Service Updated")
	}

	return nil
}

// 校验是否需要安装或更新Headless Service
func (r *MinIOReconciler) checkMinIOHLSvc(ctx context.Context, minio *miniov1alpha1.MinIO) error {
	svc, err := r.KubeClient.CoreV1().Services(minio.Namespace).Get(ctx, minio.MinIOHLServiceName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			minio.Status.Status = miniov1alpha1.DeployStatusRunning
			if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
				return err
			}
			klog.V(2).Infof("Creating a new Console Service %s/%s", minio.Namespace, minio.MinIOConsoleServiceName())
			svc = utils.NewHeadlessServiceForMinIO(minio)
			svc, err = r.KubeClient.CoreV1().Services(minio.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			if err != nil {
				minio.Status.Status = miniov1alpha1.DeployStatusFailed
				minio.Status.Message = "MinIO Headless Service Created Failed"
				if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
					return err
				}
				return err
			}
			r.Recorder.Event(minio, corev1.EventTypeNormal, "HLSvcCreated", "MinIO Headless Service Created")
		} else {
			return err
		}
	}

	expectedSvc := utils.NewConsoleServiceForMinIO(minio)
	isMatch, err := utils.MinioSvcMatchesSpecification(svc, expectedSvc)
	if !isMatch {
		if err != nil {
			klog.Infof("MinIO Console Services don't match: %s", err)
		}
		svc.Annotations = expectedSvc.Annotations
		svc.Labels = expectedSvc.Labels
		svc.Spec = expectedSvc.Spec

		_, err = r.KubeClient.CoreV1().Services(minio.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			minio.Status.Status = miniov1alpha1.DeployStatusFailed
			minio.Status.Message = "MinIO Headless Service Created Failed"
			if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
				return err
			}
			return err
		}
		r.Recorder.Event(minio, corev1.EventTypeNormal, "SvcUpdated", "MinIO Headless Service Updated")
	}

	return nil
}

// 校验是否安装了 PVC
func (r *MinIOReconciler) checkPVC(ctx context.Context, minio *miniov1alpha1.MinIO) error {
	var pvcList corev1.PersistentVolumeClaimList
	if err := r.List(ctx, &pvcList, client.InNamespace(minio.Namespace), client.MatchingLabels{miniov1alpha1.MinIOLable: minio.Name}); err != nil {
		klog.Errorf("query PVC list error")
		return err
	}
	if len(pvcList.Items) == 0 {
		minio.Status.Status = miniov1alpha1.DeployStatusRunning
		if err := r.updateMinIOStatusWithRetry(ctx, minio, true); err != nil {
			return err
		}
		klog.V(2).Info("Creating new PVC")
		var allPvcs []*corev1.PersistentVolumeClaim
		for _, pool := range minio.Spec.Pools {

			pvcs := utils.NewPersistentVolumeClaimForMinIOPool(ctx, minio, &pool)
			allPvcs = append(allPvcs, pvcs...)
		}

		for _, pvc := range allPvcs {
			if err := r.Create(ctx, pvc); err != nil {
				klog.Errorf("Create PVC %s/%s error: %s", pvc.Namespace, pvc.Name, err)
				r.Recorder.Event(minio, corev1.EventTypeWarning, "CreatePVCFailed", "Create PVC Failed")
			}
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinIOReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.MinIO{}).
		For(&corev1.Pod{}).
		For(&corev1.Service{}).
		// For(&corev1.PersistentVolumeClaim{}).
		// For(&corev1.Secret{}).
		Complete(r)
}
