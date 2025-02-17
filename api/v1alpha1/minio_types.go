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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MinIOSpec defines the desired state of MinIO
type MinIOSpec struct {
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
	// 服务池需要启动MinIO服务的pod数量
	Servers int32 `json:"servers"`
	// 每个服务需要挂载的卷数量
	VolumesPerServer int32 `json:"volumesPerServer"`
	// 指定要使用的存储卷
	VolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"volumeClaimTemplate"`
	// 是否启用 tls
	EnableCert bool `json:"enableCert,omitempty"`

	ServiceAccountName       string                      `json:"serviceAccountName,omitempty"`
	Tolerations              []corev1.Toleration         `json:"tolerations,omitempty"`
	Resources                corev1.ResourceRequirements `json:"resources,omitempty"`
	NodeSelector             map[string]string           `json:"nodeSelector,omitempty"`
	Affinity                 *corev1.Affinity            `json:"affinity,omitempty"`
	Readiness                *corev1.Probe               `json:"readiness,omitempty"`
	Startup                  *corev1.Probe               `json:"startup,omitempty"`
	Lifecycle                *corev1.Lifecycle           `json:"lifecycle,omitempty"`
	SecurityContext          *corev1.PodSecurityContext  `json:"securityContext,omitempty"`
	ContainerSecurityContext *corev1.SecurityContext     `json:"containerSecurityContext,omitempty"`
}

type ExposeServices struct {
	// 是否暴露 MinIO 服务
	MinIO bool `json:"minio,omitempty"`
	// 是否暴露 MinIO Console 服务
	Console bool `json:"console,omitempty"`
}

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
	// 未知状态
	DeployStatusUnknow DeployStatus = "Unknow"
)

type HealthStatus string

const (
	HealthStatusHealth   = "Health"
	HealthStatusUnHealth = "UnHealth"
)

// MinIOStatus defines the observed state of MinIO
type MinIOStatus struct {
	Status DeployStatus `json:"status"`
	// 状态异常信息
	Message      string       `json:"message"`
	HealthStatus HealthStatus `json:"healthStatus"`
	// 服务状态
	Servers []MinIOServer `json:"servers"`
	// 服务访问地址
	Service MinIOServiceAddr `json:"service"`
}

// MinIO 服务状态
type MinIOServer struct {
	Name   string `json:"name"`
	HostIP string `json:"hostIP"`
	PodIP  string `json:"podIP"`
	Ready  bool   `json:"ready"`
}

// MinIO 访问地址，包括服务地址和 Console 地址
type MinIOServiceAddr struct {
	MinIO   string `json:"minio"`
	Console string `json:"console"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=minio
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
