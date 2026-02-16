package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log            *log.Logger
	KubeConfigPath string
	KubeContext    string
	InCluster      bool
}

func New(l *log.Logger, cfg api.StaticConfig) (*provider, error) {
	p := &provider{
		log: l,
	}
	var err error

	p.InCluster = cfg.Exists("inCluster")

	if !p.InCluster {
		p.KubeConfigPath, err = getKubeConfigPath(cfg)
		if err != nil {
			p.log.Debugf("vals-k8s: Unable to get a valid kubeConfig path: %s", err)
			return nil, err
		}

		p.KubeContext = getKubeContext(cfg)

		if p.KubeContext == "" {
			p.log.Debugf("vals-k8s: kubeContext was not provided. Using current context.")
		}
	}

	return p, nil
}

func getKubeConfigPath(cfg api.StaticConfig) (string, error) {
	// Use kubeConfigPath from URI parameters if specified
	if cfg.String("kubeConfigPath") != "" {
		if _, err := os.Stat(cfg.String("kubeConfigPath")); err != nil {
			return "", fmt.Errorf("kubeConfigPath URI parameter is set but path %s does not exist.", cfg.String("kubeConfigPath"))
		}
		return cfg.String("kubeConfigPath"), nil
	}

	// Use path in KUBECONFIG environment variable if set
	if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return "", fmt.Errorf("KUBECONFIG environment variable is set but path %s does not exist.", envPath)
		}
		return envPath, nil
	}

	// Use default kubeconfig path if it exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("An error occurred getting the user's home directory: %s", err)
	}

	defaultPath := homeDir + "/.kube/config"
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, nil
	}

	return "", fmt.Errorf("No path was found in any of the following: kubeContext URI param, KUBECONFIG environment variable, or default path %s does not exist.", defaultPath)
}

// fetchObjectData validates a 4-part path and fetches the object data from Kubernetes.
func (p *provider) fetchObjectData(path string) (kind, namespace, name string, objectData map[string]string, err error) {
	splits := strings.Split(path, "/")
	if len(splits) != 4 {
		return "", "", "", nil, fmt.Errorf("Invalid path %s. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>", path)
	}

	apiVersion := splits[0]
	kind = splits[1]
	namespace = splits[2]
	name = splits[3]

	if apiVersion != "v1" {
		return "", "", "", nil, fmt.Errorf("Invalid apiVersion %s. Only apiVersion v1 is supported at this time.", apiVersion)
	}

	objectData, err = getObject(kind, namespace, name, p.KubeConfigPath, p.KubeContext, p.InCluster, context.Background())
	if err != nil {
		return "", "", "", nil, fmt.Errorf("Unable to get %s %s/%s: %s", kind, namespace, name, err)
	}

	// Normalize nil data (e.g., ConfigMap with no .data) to an empty map
	// so callers get consistent behavior ({} instead of null in JSON).
	if objectData == nil {
		objectData = map[string]string{}
	}

	return kind, namespace, name, objectData, nil
}

func (p *provider) logRetrieval(message string) {
	if p.KubeContext != "" {
		message += fmt.Sprintf(" (KubeContext: %s)", p.KubeContext)
	}
	p.log.Debugf(message)
}

func (p *provider) GetString(path string) (string, error) {
	splits := strings.Split(path, "/")

	if len(splits) != 4 && len(splits) != 5 {
		return "", fmt.Errorf("Invalid path %s. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>[/<key>]", path)
	}

	// 5-part path: fetch a single key
	if len(splits) == 5 {
		key := splits[4]
		if key == "" {
			return "", fmt.Errorf("Invalid path %s. Key must not be empty in the format <apiVersion>/<kind>/<namespace>/<name>/<key>", path)
		}

		basePath := strings.Join(splits[:4], "/")
		kind, namespace, name, objectData, err := p.fetchObjectData(basePath)
		if err != nil {
			return "", err
		}

		object, exists := objectData[key]
		if !exists {
			return "", fmt.Errorf("Key %s does not exist in %s/%s", key, namespace, name)
		}

		p.logRetrieval(fmt.Sprintf("vals-k8s: Retrieved %s: %s/%s/%s", kind, namespace, name, key))
		return object, nil
	}

	// 4-part path: return all keys as JSON
	kind, namespace, name, objectData, err := p.fetchObjectData(path)
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(objectData)
	if err != nil {
		return "", fmt.Errorf("Unable to marshal %s %s/%s to JSON: %w", kind, namespace, name, err)
	}

	p.logRetrieval(fmt.Sprintf("vals-k8s: Retrieved all keys from %s: %s/%s", kind, namespace, name))
	return string(jsonBytes), nil
}

func (p *provider) GetStringMap(path string) (map[string]interface{}, error) {
	kind, namespace, name, objectData, err := p.fetchObjectData(path)
	if err != nil {
		return nil, err
	}

	// Convert map[string]string to map[string]interface{}
	result := make(map[string]interface{}, len(objectData))
	for k, v := range objectData {
		result[k] = v
	}

	p.logRetrieval(fmt.Sprintf("vals-k8s: Retrieved all keys from %s: %s/%s", kind, namespace, name))
	return result, nil
}

// Return an empty Kube context if none is provided
func getKubeContext(cfg api.StaticConfig) string {
	if cfg.String("kubeContext") != "" {
		return cfg.String("kubeContext")
	}
	return ""
}

// Build the client-go config using a specific context
func buildConfigWithContextFromFlags(context string, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}

// Fetch the object from the Kubernetes cluster
func getObject(kind string, namespace string, name string, kubeConfigPath string, kubeContext string, inCluster bool, ctx context.Context) (map[string]string, error) {
	var config *rest.Config
	var err error

	if inCluster {
		config, err = rest.InClusterConfig()
	} else {
		config, err = buildConfigWithContextFromFlags(kubeContext, kubeConfigPath)
	}

	if err != nil {
		return nil, fmt.Errorf("Unable to build config from vals configuration: %s", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Unable to create the Kubernetes client: %s", err)
	}

	var object map[string]string

	switch kind {
	case "Secret":
		secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Unable to get the Secret object from Kubernetes: %s", err)
		}
		object = convertByteMapToStringMap(secret.Data)
	case "ConfigMap":
		configmap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Unable to get the ConfigMap object from Kubernetes: %s", err)
		}
		object = configmap.Data
	default:
		return nil, fmt.Errorf("The specified kind is not valid. Valid kinds: Secret, ConfigMap")
	}

	return object, nil
}

func convertByteMapToStringMap(byteMap map[string][]byte) map[string]string {
	stringMap := make(map[string]string)

	for key, value := range byteMap {
		stringMap[key] = string(value)
	}

	return stringMap
}
