package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	log            *log.Logger
	KubeConfigPath string
	KubeContext    string
}

func New(l *log.Logger, cfg api.StaticConfig) (*provider, error) {
	p := &provider{
		log: l,
	}
	var err error

	p.KubeConfigPath, err = getKubeConfigPath(cfg)
	if err != nil {
		p.log.Debugf("vals-k8s: Unable to get a valid kubeConfig path: %s", err)
		return nil, err
	}

	p.KubeContext = getKubeContext(cfg)

	if p.KubeContext == "" {
		p.log.Debugf("vals-k8s: kubeContext was not provided. Using current context.")
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

func (p *provider) GetString(path string) (string, error) {
	separator := "/"
	splits := strings.Split(path, separator)

	if len(splits) != 5 {
		return "", fmt.Errorf("Invalid path %s. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>/<key>", path)
	}

	apiVersion := splits[0]
	kind := splits[1]
	namespace := splits[2]
	name := splits[3]
	key := splits[4]

	if apiVersion != "v1" {
		return "", fmt.Errorf("Invalid apiVersion %s. Only apiVersion v1 is supported at this time.", apiVersion)
	}

	objectData, err := getObject(kind, namespace, name, p.KubeConfigPath, p.KubeContext, context.Background())
	if err != nil {
		return "", fmt.Errorf("Unable to get %s %s/%s: %s", kind, namespace, name, err)
	}

	object, exists := objectData[key]
	if !exists {
		return "", fmt.Errorf("Key %s does not exist in %s/%s", key, namespace, name)
	}

	// Print success message with kubeContext if provided
	message := fmt.Sprintf("vals-k8s: Retrieved %s: %s/%s/%s", kind, namespace, name, key)
	if p.KubeContext != "" {
		message += fmt.Sprintf(" (KubeContext: %s)", p.KubeContext)
	}
	p.log.Debugf(message)

	return object, nil
}

func (p *provider) GetStringMap(path string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("This provider does not support values from URI fragments")
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
func getObject(kind string, namespace string, name string, kubeConfigPath string, kubeContext string, ctx context.Context) (map[string]string, error) {
	config, err := buildConfigWithContextFromFlags(kubeContext, kubeConfigPath)

	if err != nil {
		return nil, fmt.Errorf("Unable to build Kubeconfig from vals configuration: %s", err)
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
