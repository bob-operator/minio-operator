package utils

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/util/intstr"
	miniov1alpha1 "minio-operator/api/v1alpha1"
)

// 根据 MinIO 实例创建一个 MinIO 服务的 Kubernetes Service 实例
func NewServiceForMinIO(m *miniov1alpha1.MinIO) *corev1.Service {
	var port int32 = miniov1alpha1.MinIOPortSVC
	name := miniov1alpha1.MinIOServiceHTTPPortName
	if m.TLS() {
		port = miniov1alpha1.MinIOTLSPortSVC
		name = miniov1alpha1.MinIOServiceHTTPSPortName
	}

	var labels, annotations map[string]string

	labels = m.MinIOPodLabels()

	minioPort := corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.FromInt(miniov1alpha1.MinIOPort),
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Annotations:     annotations,
			Name:            m.MinIOCIServiceName(),
			Namespace:       m.Namespace,
			OwnerReferences: m.OwnerRef(),
		},
		Spec: corev1.ServiceSpec{
			Ports:    []corev1.ServicePort{minioPort},
			Selector: m.MinIOPodLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	if m.ExposeMinIOSvc() {
		svc.Spec.Type = corev1.ServiceTypeNodePort
	}

	return svc
}

// 根据 MinIO 实例创建一个 MinIO Console 服务的 Kubernetes Service 实例
func NewConsoleServiceForMinIO(m *miniov1alpha1.MinIO) *corev1.Service {
	var labels, annotations map[string]string
	consolePort := corev1.ServicePort{
		Name:       miniov1alpha1.ConsoleServicePortName,
		Port:       miniov1alpha1.ConsolePort,
		TargetPort: intstr.FromInt(miniov1alpha1.ConsolePort),
	}

	// TODO:暂不启用 tls
	// if m.TLS() {
	// 	consolePort = corev1.ServicePort{
	// 		Name:       miniov1alpha1.ConsoleServiceTLSPortName,
	// 		Port:       miniov1alpha1.ConsoleTLSPort,
	// 		TargetPort: intstr.FromInt(miniov1alpha1.ConsoleTLSPort),
	// 	}
	// }

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Name:            m.MinIOConsoleServiceName(),
			Namespace:       m.Namespace,
			OwnerReferences: m.OwnerRef(),
			Annotations:     annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				consolePort,
			},
			Selector: m.MinIOPodLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	if m.ExposeMinIOConsoleSvc() {
		svc.Spec.Type = corev1.ServiceTypeNodePort
	}

	return svc
}

// 创建用于内部通信的 Headless Service 实例
func NewHeadlessServiceForMinIO(m *miniov1alpha1.MinIO) *corev1.Service {
	name := miniov1alpha1.MinIOServiceHTTPPortName
	if m.TLS() {
		name = miniov1alpha1.MinIOServiceHTTPSPortName
	}

	minioPort := corev1.ServicePort{Port: miniov1alpha1.MinIOPort, Name: name}
	ports := []corev1.ServicePort{minioPort}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          m.MinIOPodLabels(),
			Name:            m.MinIOHLServiceName(),
			Namespace:       m.Namespace,
			OwnerReferences: m.OwnerRef(),
		},
		Spec: corev1.ServiceSpec{
			Ports:                    ports,
			Selector:                 m.MinIOPodLabels(),
			Type:                     corev1.ServiceTypeClusterIP,
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
		},
	}

	return svc
}

// 校验 Service 是否有更新
func MinioSvcMatchesSpecification(svc *corev1.Service, expectedSvc *corev1.Service) (bool, error) {
	for k, expVal := range expectedSvc.ObjectMeta.Labels {
		if value, ok := svc.ObjectMeta.Labels[k]; !ok || value != expVal {
			return false, errors.New("service labels don't match")
		}
	}
	for k, expVal := range expectedSvc.ObjectMeta.Annotations {
		if value, ok := svc.ObjectMeta.Annotations[k]; !ok || value != expVal {
			return false, errors.New("service annotations don't match")
		}
	}
	// expected ports match
	if len(svc.Spec.Ports) != len(expectedSvc.Spec.Ports) {
		return false, errors.New("service ports don't match")
	}

	for i, expPort := range expectedSvc.Spec.Ports {
		if expPort.Name != svc.Spec.Ports[i].Name ||
			expPort.Port != svc.Spec.Ports[i].Port ||
			expPort.TargetPort != svc.Spec.Ports[i].TargetPort {
			return false, errors.New("service ports don't match")
		}
	}
	// compare selector
	if !equality.Semantic.DeepDerivative(expectedSvc.Spec.Selector, svc.Spec.Selector) {
		// some field set by the operator has changed
		return false, errors.New("selectors don't match")
	}
	if svc.Spec.Type != expectedSvc.Spec.Type {
		return false, errors.New("Service type doesn't match")
	}
	return true, nil
}
