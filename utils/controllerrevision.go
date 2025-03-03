package utils

import (
	"context"

	"k8s.io/klog/v2"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// 校验 ControllerRevision 是否存在
func ExistControllerRevision(name, namespace string, client client.Client) (bool, *appsv1.ControllerRevision, error) {
	found := &appsv1.ControllerRevision{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		} else {
			klog.Errorf("get ControllerRevision %s/%s error, %s", namespace, name, err)
		}
		return true, found, err
	}
	return true, found, nil
}

func CreateControllerRevision(owner client.Object, cr *appsv1.ControllerRevision, client client.Client, scheme *runtime.Scheme) error {
	if err := controllerutil.SetControllerReference(owner, cr, scheme); err != nil {
		klog.Errorf("set controllerReference for ControllerRevision %s/%s failed, %s", owner.GetNamespace(), owner.GetName(), err)
		return err
	}
	klog.Infof("create ControllerRevision %s/%s ", owner.GetNamespace(), owner.GetName())
	if err := client.Create(context.TODO(), cr); err != nil {
		klog.Errorf("create ControllerRevision%s/%s error, %s", owner.GetNamespace(), owner.GetName(), err)
		return err
	}
	return nil
}

func DeleteControllerRevision(oldCr *appsv1.ControllerRevision, client client.Client) error {
	if err := client.Delete(context.TODO(), oldCr); err != nil {
		klog.Infof("delete old ControllerRevision %s/%s error, %s", oldCr.Namespace, oldCr.Name, err)
		return err
	}
	return nil
}

func RebuildControllerRevision(owner client.Object, oldCr, newCr *appsv1.ControllerRevision, client client.Client, scheme *runtime.Scheme) error {
	if err := client.Delete(context.TODO(), oldCr); err != nil {
		klog.Infof("delete old ControllerRevision %s/%s error, %s", oldCr.Namespace, oldCr.Name, err)
		return err
	}
	if err := controllerutil.SetControllerReference(owner, newCr, scheme); err != nil {
		klog.Infof("set controllerReference for ControllerRevision %s/%s error, %s", owner.GetNamespace(), owner.GetName(), err)
		return err
	}
	if err := client.Create(context.TODO(), newCr); err != nil {
		klog.Infof("create ControllerRevision%s/%s error, %s", owner.GetNamespace(), owner.GetName(), err)
		return err
	}

	return nil
}
