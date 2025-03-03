package utils

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/klog/v2"
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

type envKV struct {
	Key   string
	Value string
	Skip  bool
}

// 解析 minio 服务配置的环境变量
func parsEnvEntry(envEntry string) (envKV, error) {
	envEntry = strings.TrimSpace(envEntry)
	if envEntry == "" {
		// Skip all empty lines
		return envKV{
			Skip: true,
		}, nil
	}
	if strings.HasPrefix(envEntry, "#") {
		// Skip commented lines
		return envKV{
			Skip: true,
		}, nil
	}
	const envSeparator = "="
	envTokens := strings.SplitN(strings.TrimSpace(strings.TrimPrefix(envEntry, "export")), envSeparator, 2)
	if len(envTokens) != 2 {
		return envKV{}, fmt.Errorf("envEntry malformed; %s, expected to be of form 'KEY=value'", envEntry)
	}

	key := envTokens[0]
	val := envTokens[1]

	// Remove quotes from the value if found
	if len(val) >= 2 {
		quote := val[0]
		if (quote == '"' || quote == '\'') && val[len(val)-1] == quote {
			val = val[1 : len(val)-1]
		}
	}

	return envKV{
		Key:   key,
		Value: val,
	}, nil
}

// ParseRawConfiguration map[string][]byte 代表 MinIO 服务的 config.env 文件
func ParseRawConfiguration(configuration []byte) (config map[string][]byte) {
	config = map[string][]byte{}
	if configuration != nil {
		scanner := bufio.NewScanner(strings.NewReader(string(configuration)))
		for scanner.Scan() {
			ekv, err := parsEnvEntry(scanner.Text())
			if err != nil {
				klog.Errorf("Error parsing tenant configuration: %v", err.Error())
				continue
			}
			if ekv.Skip {
				// Skips empty lines
				continue
			}
			config[ekv.Key] = []byte(ekv.Value)
			if ekv.Key == "MINIO_ROOT_USER" || ekv.Key == "MINIO_ACCESS_KEY" {
				config["accesskey"] = config[ekv.Key]
			} else if ekv.Key == "MINIO_ROOT_PASSWORD" || ekv.Key == "MINIO_SECRET_KEY" {
				config["secretkey"] = config[ekv.Key]
			}
		}
		if err := scanner.Err(); err != nil {
			klog.Errorf("Error parsing tenant configuration: %v", err.Error())
			return config
		}
	}
	return config
}

func CreateTransport() *http.Transport {
	// rootCAs := c.fetchTransportCACertificates()
	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 15 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConnsPerHost:   1024,
		IdleConnTimeout:       15 * time.Second,
		ResponseHeaderTimeout: 15 * time.Minute,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 15 * time.Second,
		// Go net/http automatically unzip if content-type is
		// gzip disable this feature, as we are always interested
		// in raw stream.
		DisableCompression: true,
		TLSClientConfig: &tls.Config{
			// Can't use SSLv3 because of POODLE and BEAST
			// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
			// Can't use TLSv1.1 because of RC4 cipher usage
			MinVersion: tls.VersionTLS12,
			// RootCAs:    rootCAs,
		},
	}

	return transport
}
