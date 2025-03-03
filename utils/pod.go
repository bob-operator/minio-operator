package utils

import (
	"context"
	miniov1alpha1 "minio-operator/api/v1alpha1"
	"reflect"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

// 校验是否需要对 MinIO 服务进行更新
func MinioServerNeedToUpdate(oldMinio, newMinio miniov1alpha1.MinIO, oldPool, newPool miniov1alpha1.Pool) bool {
	if oldMinio.Spec.Image != newMinio.Spec.Image ||
		oldMinio.Spec.ImagePullPolicy != newMinio.Spec.ImagePullPolicy ||
		oldMinio.Spec.ImagePullSecret.Name != newMinio.Spec.ImagePullSecret.Name ||
		(oldMinio.Spec.Liveness != nil && newMinio.Spec.Liveness != nil && !reflect.DeepEqual(oldMinio.Spec.Liveness, newMinio.Spec.Liveness)) ||
		(oldMinio.Spec.Readiness != nil && newMinio.Spec.Readiness != nil && !reflect.DeepEqual(oldMinio.Spec.Readiness, newMinio.Spec.Readiness)) ||
		(oldMinio.Spec.Startup != nil && newMinio.Spec.Startup != nil && !reflect.DeepEqual(oldMinio.Spec.Startup, newMinio.Spec.Startup)) ||
		(oldMinio.Spec.Affinity != nil && newMinio.Spec.Affinity != nil && !reflect.DeepEqual(oldMinio.Spec.Affinity, newMinio.Spec.Affinity)) {
		return true
	}

	if !reflect.DeepEqual(oldMinio.Spec.Resources, newMinio.Spec.Resources) {
		return true
	}

	if reflect.DeepEqual(oldPool.NodeSelector, newPool.NodeSelector) {
		return true
	}

	return false
}

// 返回 Pod 列表
func NewPodsForMinIOPool(ctx context.Context, minio miniov1alpha1.MinIO, pool miniov1alpha1.Pool) (pods []corev1.Pod) {
	servers := pool.Servers
	volumesPerServer := pool.VolumesPerServer

	for i := 0; i < servers; i++ {
		var volumes []corev1.Volume
		var volMounts []corev1.VolumeMount

		// 设置 volumes
		for j := 0; j < volumesPerServer; j++ {
			volName := pool.VolumeClaimTemplate.Name + strconv.Itoa(j)
			vol := corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: minio.Name + "-" + pool.Name + "-" + strconv.Itoa(i) + "-" + pool.VolumeClaimTemplate.Name + "-" + strconv.Itoa(j),
					},
				},
			}
			volumes = append(volumes, vol)
		}

		var mountPath string
		if strings.HasSuffix(minio.Spec.Mountpath, "/") {
			mountPath = strings.TrimRight(minio.Spec.Mountpath, "/")
		} else {
			mountPath = minio.Spec.Mountpath
		}

		// 设置 volumeMounts
		if volumesPerServer == 1 {
			volMount := corev1.VolumeMount{
				Name:      pool.VolumeClaimTemplate.Name + strconv.Itoa(0),
				MountPath: mountPath,
			}
			volMounts = append(volMounts, volMount)
		} else {
			for j := 0; j < volumesPerServer; j++ {
				volMount := corev1.VolumeMount{
					Name:      pool.VolumeClaimTemplate.Name + strconv.Itoa(j),
					MountPath: mountPath + "-" + strconv.Itoa(j),
				}
				volMounts = append(volMounts, volMount)
			}
		}

		containers := []corev1.Container{
			minioServerContainer(minio, pool, volMounts),
		}
		labels := minio.MinIOPodLabels()
		labels[miniov1alpha1.PoolLabel] = pool.Name
		podName := pool.Name + "-" + strconv.Itoa(i)

		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:            podName,
				Namespace:       minio.Namespace,
				Labels:          labels,
				OwnerReferences: minio.OwnerRef(),
			},
			Spec: corev1.PodSpec{
				Volumes:            volumes,
				Containers:         containers,
				NodeSelector:       pool.NodeSelector,
				ServiceAccountName: minio.Spec.ServiceAccountName,
				Hostname:           podName,
				Subdomain:          minio.MinIOHLServiceName(),
				Affinity:           minio.Spec.Affinity,
				Tolerations:        minio.Spec.Tolerations,
			},
		}
		pods = append(pods, pod)
	}

	return pods
}

func minioServerContainer(m miniov1alpha1.MinIO, pool miniov1alpha1.Pool, volumeMounts []corev1.VolumeMount) corev1.Container {
	consolePort := miniov1alpha1.ConsolePort
	if m.TLS() {
		consolePort = miniov1alpha1.ConsoleTLSPort
	}

	args := []string{
		"server",
		"--certs-dir", miniov1alpha1.MinIOCertPath,
		"--console-address", ":" + strconv.Itoa(consolePort),
	}

	containerPorts := []corev1.ContainerPort{
		{
			ContainerPort: miniov1alpha1.MinIOPort,
		},
		{
			ContainerPort: int32(consolePort),
		},
	}

	// 设置默认环境变量，不开启奇偶校验
	env := m.DefaultPodEnv()

	return corev1.Container{
		Name:            miniov1alpha1.MinIOServerName,
		Image:           m.Spec.Image,
		Ports:           containerPorts,
		ImagePullPolicy: m.Spec.ImagePullPolicy,
		VolumeMounts:    volumeMounts,
		Args:            args,
		Env:             env,
		Resources:       m.Spec.Resources,
		LivenessProbe:   m.Spec.Liveness,
		ReadinessProbe:  m.Spec.Readiness,
		StartupProbe:    m.Spec.Startup,
		Lifecycle:       m.Spec.Lifecycle,
		SecurityContext: pool.ContainerSecurityContext,
	}
}
