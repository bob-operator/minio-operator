package v1alpha1

import (
	"context"
	"errors"
	stderr "errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	"k8s.io/apimachinery/pkg/util/json"

	appsv1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/minio/madmin-go/v2"
	corev1 "k8s.io/api/core/v1"
)

var (
	once                    sync.Once
	tenantMinIOImageOnce    sync.Once
	tenantKesImageOnce      sync.Once
	monitoringIntervalOnce  sync.Once
	k8sClusterDomain        string
	tenantMinIOImage        string
	tenantKesImage          string
	monitoringInterval      int
	prometheusNamespace     string
	prometheusName          string
	prometheusNamespaceOnce sync.Once
	prometheusNameOnce      sync.Once
	// gcpAppCredentialENV to denote the GCP APP credential path
	gcpAppCredentialENV = corev1.EnvVar{
		Name:  "GOOGLE_APPLICATION_CREDENTIALS",
		Value: "/var/run/secrets/tokens/gcp-ksa/google-application-credentials.json",
	}
)

// 校验是否设置了表示配置的 Secret
func (m *MinIO) HasConfigurationSecret() bool {
	return m.Spec.Configuration != nil && m.Spec.Configuration.Name != ""
}

// 查询设置的环境变量
func (m *MinIO) GetEnvVars() (env []corev1.EnvVar) {
	return m.Spec.Env
}

func (m *MinIO) NewMinIOAdmin(minioSecret map[string][]byte, tr *http.Transport) (*madmin.AdminClient, error) {
	return m.NewMinIOAdminForAddress("", minioSecret, tr)
}

func (m *MinIO) NewMinIOAdminForAddress(address string, minioSecret map[string][]byte, tr *http.Transport) (*madmin.AdminClient, error) {
	host, accessKey, secretKey, err := m.getMinIOTenantDetails(address, minioSecret)
	if err != nil {
		return nil, err
	}

	opts := &madmin.Options{
		Secure: m.TLS(),
		Creds:  credentials.NewStaticV4(string(accessKey), string(secretKey), ""),
	}

	madmClnt, err := madmin.NewWithOptions(host, opts)
	if err != nil {
		return nil, err
	}
	madmClnt.SetCustomTransport(tr)

	return madmClnt, nil
}

func (m *MinIO) getMinIOTenantDetails(address string, minioSecret map[string][]byte) (string, []byte, []byte, error) {
	host := address
	if host == "" {
		host = m.MinIOServerHostAddress()
		if host == "" {
			return "", nil, nil, stderr.New("MinIO server host is empty")
		}
	}

	accessKey, ok := minioSecret["accesskey"]
	if !ok {
		return "", nil, nil, errors.New("MinIO server accesskey not set")
	}

	secretKey, ok := minioSecret["secretkey"]
	if !ok {
		return "", nil, nil, errors.New("MinIO server secretkey not set")
	}
	return host, accessKey, secretKey, nil
}

// MinIOServerHostAddress similar to MinIOFQDNServiceName but returns host with port
func (m *MinIO) MinIOServerHostAddress() string {
	var port int

	if m.TLS() {
		port = MinIOPortSVC
	} else {
		port = MinIOTLSPortSVC
	}

	return net.JoinHostPort(m.MinIOFQDNServiceName(), strconv.Itoa(port))
}

func (m *MinIO) TLS() bool {
	return m.Spec.EnableCert
}

func (m *MinIO) MinIOFQDNServiceName() string {
	return fmt.Sprintf("%s.%s.svc.%s", m.MinIOCIServiceName(), m.Namespace, GetClusterDomain())
}

// ClusterIP模式的 Service 和 MinIO 实例同名
func (m *MinIO) MinIOCIServiceName() string {
	return m.Name
}

// 返回 Headless Service 实例
func (m *MinIO) MinIOHLServiceName() string {
	return m.Name + "hl"
}

// 返回 MinIO Console
func (m *MinIO) MinIOConsoleServiceName() string {
	return m.Name + "console"
}

func (m *MinIO) DefaultPodEnv() []corev1.EnvVar {
	var envVar []corev1.EnvVar
	envVar = append(envVar, corev1.EnvVar{
		Name:  "MINIO_STORAGE_CLASS_STANDARD",
		Value: "EC:0",
	})

	return envVar
}

func (m *MinIO) NewControllerRevision() *appsv1.ControllerRevision {
	rawData, _ := json.Marshal(m.Spec)

	cr := &appsv1.ControllerRevision{}
	cr.Namespace = m.Namespace
	cr.Name = m.Name
	cr.Data.Raw = rawData

	return cr
}

// returns the Kubernetes cluster domain
func GetClusterDomain() string {
	return "cluster.local"
}

func envGet(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}

// 返回 MinIO Pod 中的默认 Lables
func (m *MinIO) MinIOPodLabels() map[string]string {
	lbs := make(map[string]string, 1)
	lbs[MinIOLable] = m.Name
	return lbs
}

// 返回由 MinIO 纳管资源的 OwnerReference
func (m *MinIO) OwnerRef() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		*metav1.NewControllerRef(m, schema.GroupVersionKind{
			Group:   GroupVersion.Group,
			Version: GroupVersion.Version,
			Kind:    MinIOCRDResourceKind,
		}),
	}
}

func (m *MinIO) ExposeMinIOSvc() bool {
	return m.Spec.ExposeServices.MinIO
}

func (m *MinIO) ExposeMinIOConsoleSvc() bool {
	return m.Spec.ExposeServices.Console
}

// MinIO 服务健康检查
func (m *MinIO) MinIOHealthCheck(tr *http.Transport) bool {
	if tr.TLSClientConfig != nil {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}

	clnt, err := madmin.NewAnonymousClient(m.MinIOServerHostAddress(), m.TLS())
	if err != nil {
		return false
	}
	clnt.SetCustomTransport(tr)

	result, err := clnt.Healthy(context.Background(), madmin.HealthOpts{})
	if err != nil {
		return false
	}

	return result.Healthy
}
