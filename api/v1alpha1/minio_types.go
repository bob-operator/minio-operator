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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MinIOSpec defines the desired state of MinIO
type MinIOSpec struct {
	// 服务池
	Pools []Pool `json:"pools"`
	// MinIO 服务镜像
	Image           string                      `json:"image"`
	ImagePullPolicy corev1.PullPolicy           `json:"imagePullPolicy,omitempty"`
	ImagePullSecret corev1.LocalObjectReference `json:"imagePullSecret,omitempty"`
	Env             []corev1.EnvVar             `json:"env,omitempty"`
	// 卷的挂载路径，默认为 /data
	Mountpath string `json:"mountPath,omitempty"`
	// MinIO 服务需要的配置,由 Secret 提供
	Configuration *corev1.LocalObjectReference `json:"configuration,omitempty"`
	// 是否暴露服务
	ExposeServices ExposeServices `json:"exposeService,omitempty"`

	// 是否启用 tls
	EnableCert bool `json:"enableCert,omitempty"`
	// 是否删除PVC，如果为 true 则在同时删除 PVC
	ReclaimStorage bool `json:"reclaimStorage,omitempty"`

	ServiceAccountName string                      `json:"serviceAccountName,omitempty"`
	Tolerations        []corev1.Toleration         `json:"tolerations,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`

	Affinity  *corev1.Affinity  `json:"affinity,omitempty"`
	Liveness  *corev1.Probe     `json:"liveness,omitempty"`
	Readiness *corev1.Probe     `json:"readiness,omitempty"`
	Startup   *corev1.Probe     `json:"startup,omitempty"`
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty"`
}

// 服务池
type Pool struct {
	// 服务池名称
	Name string `json:"name"`
	// 服务池需要启动MinIO服务的pod数量
	Servers int `json:"servers"`
	// 每个服务需要挂载的卷数量
	VolumesPerServer int `json:"volumesPerServer"`
	// 指定要使用的存储卷
	VolumeClaimTemplate      *corev1.PersistentVolumeClaim `json:"volumeClaimTemplate"`
	NodeSelector             map[string]string             `json:"nodeSelector,omitempty"`
	SecurityContext          *corev1.PodSecurityContext    `json:"securityContext,omitempty"`
	ContainerSecurityContext *corev1.SecurityContext       `json:"containerSecurityContext,omitempty"`
}

type ExposeServices struct {
	// 是否暴露 MinIO 服务
	MinIO bool `json:"minio,omitempty"`
	// 是否暴露 MinIO Console 服务
	Console bool `json:"console,omitempty"`
}

// 整体部署状态
type DeployStatus string

const (
	// 初始状态
	DeployStatusNone DeployStatus = "None"
	// 服务部署完成
	DeployStatusCompleted DeployStatus = "Completed"
	// 正在运行
	DeployStatusRunning DeployStatus = "Running"
	// 部署失败
	DeployStatusFailed DeployStatus = "Failed"
)

// 服务池部署状态
type PoolDeployStatus string

const (
	// 资源池创建完成
	PoolStatusCompleted PoolDeployStatus = "Completed"
	// 资源池正在创建中
	PoolStatusRunning PoolDeployStatus = "Running"
	// 资源池创建失败
	PoolStatusFailed PoolDeployStatus = "Failed"
)

// 服务健康状态
type HealthStatus string

const (
	HealthStatusHealth   HealthStatus = "Health"
	HealthStatusUnHealth HealthStatus = "UnHealth"
	HealthStatusUnknown  HealthStatus = "Unknown"
)

// MinIOStatus defines the observed state of MinIO
type MinIOStatus struct {
	Status DeployStatus `json:"status"`
	// 状态异常信息
	Message      string       `json:"message"`
	PoolStatus   []PoolStatus `json:"poolStatus"`
	HealthStatus HealthStatus `json:"healthStatus"`
	// 服务访问地址
	Service   MinIOServiceAddr `json:"service"`
	PVCStatus []PVCStatus      `json:"pvcStatus"`
}

type PoolStatus struct {
	// MinIO 服务池名称
	Name              string           `json:"name"`
	Status            PoolDeployStatus `json:"status"`
	AvailableReplicas int              `json:"availableReplicas"`
	Replicas          int              `json:"replicas"`
	// 服务状态
	Servers []MinIOServer `json:"servers"`
}

// MinIO 服务状态
type MinIOServer struct {
	Name   string `json:"name"`
	HostIP string `json:"hostIP"`
	PodIP  string `json:"podIP"`
	Status string `json:"status"`
}

// MinIO 访问地址，包括服务地址和 Console 地址
type MinIOServiceAddr struct {
	MinIO   string `json:"minio"`
	Console string `json:"console"`
}

type PVCStatus struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Volume       string `json:"volume"`
	Capacity     string `json:"capacity"`
	StorageClass string `json:"storageClass"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=minio
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

// MinIO is the Schema for the minios API
type MinIO struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinIOSpec   `json:"spec,omitempty"`
	Status MinIOStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MinIOList contains a list of MinIO
type MinIOList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinIO `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinIO{}, &MinIOList{})
}
