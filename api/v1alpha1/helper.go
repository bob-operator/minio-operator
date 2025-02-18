package v1alpha1

import (
	"errors"
	stderr "errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/minio/madmin-go/v2"
	corev1 "k8s.io/api/core/v1"
)

const (
	// MinIOPortLoadBalancerSVC specifies the default Service port number for the load balancer service.
	MinIOPortLoadBalancerSVC = 80

	// MinIOTLSPortLoadBalancerSVC specifies the default Service TLS port number for the load balancer service.
	MinIOTLSPortLoadBalancerSVC = 443

	clusterDomain = "CLUSTER_DOMAIN"
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
		port = MinIOTLSPortLoadBalancerSVC
	} else {
		port = MinIOPortLoadBalancerSVC
	}

	return net.JoinHostPort(m.MinIOFQDNServiceName(), strconv.Itoa(port))
}

func (m *MinIO) TLS() bool {
	return m.Spec.EnableCert
}

func (m *MinIO) MinIOFQDNServiceName() string {
	return fmt.Sprintf("%s.%s.svc.%s", m.MinIOCIServiceName(), m.Namespace, getClusterDomain())
}

// ClusterIP模式的 Service 和 MinIO 实例同名
func (m *MinIO) MinIOCIServiceName() string {
	// DO NOT CHANGE, this should be constant
	// This is possible because each namespace has only one Tenant
	// return "minio"
	return m.Name
}

// returns the Kubernetes cluster domain
func getClusterDomain() string {
	once.Do(func() {
		k8sClusterDomain = envGet(clusterDomain, "cluster.local")
	})
	return k8sClusterDomain
}

func envGet(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}
