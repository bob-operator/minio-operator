package utils

import (
	"context"
	miniov1alpha1 "minio-operator/api/v1alpha1"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

// 根据 MinIO 实例构建 PVC 实例
func NewPersistentVolumeClaimForMinIOPool(ctx context.Context, minio *miniov1alpha1.MinIO, pool *miniov1alpha1.Pool) (pvcs []*corev1.PersistentVolumeClaim) {
	servers := pool.Servers
	volumesPerServer := pool.VolumesPerServer
	labels := minio.MinIOPodLabels()
	labels[miniov1alpha1.PoolLabel] = pool.Name

	for i := 0; i < servers; i++ {
		for j := 0; j < volumesPerServer; j++ {
			// pvc 名称规则: "MINIO名称-pool名称-pool索引-pool中设置的PVC名称-pod挂载的卷的索引"
			name := minio.Name + "-" + pool.Name + "-" + strconv.Itoa(i) + "-" + pool.VolumeClaimTemplate.Name + "-" + strconv.Itoa(j)
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: minio.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:        minio.Spec.Resources,
					StorageClassName: pool.VolumeClaimTemplate.Spec.StorageClassName,
				},
			}
			pvcs = append(pvcs, pvc)
		}
	}

	return pvcs
}
