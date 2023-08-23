package kubernetes

import (
	"context"
	"sync"

	istio "istio.io/client-go/pkg/clientset/versioned"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	gatewayapiclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	kialiconfig "github.com/kiali/kiali/config"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/util/httputil"
)

var (
	emptyGetOptions  = meta_v1.GetOptions{}
	emptyListOptions = meta_v1.ListOptions{}
)

type PodLogs struct {
	Logs string `json:"logs,omitempty"`
}

// ClusterInfo is basically a rest.Config with a few extra fields that are useful to Kiali.
type ClusterInfo struct {
	// ClientConfig is the rest.Config is used to create clients for the various APIs.
	ClientConfig *rest.Config

	// Name is the name of the cluster this client is connected to.
	Name string

	// SecretName is the name of the secret that contains the credentials for this cluster.
	SecretName string
}

// ClientInterface for mocks (only mocked function are necessary here)
type ClientInterface interface {
	GetServerVersion() (*version.Info, error)
	GetToken() string
	IsOpenShift() bool
	IsGatewayAPI() bool
	IsIstioAPI() bool
	// ClusterInfo returns some information about the cluster this client is connected to.
	// This gets set when the client is first created.
	ClusterInfo() ClusterInfo
	K8SClientInterface
	IstioClientInterface
	OSClientInterface
}

// K8SClient is the client struct for Kubernetes and Istio APIs
// It hides the way it queries each API
type K8SClient struct {
	token          string
	k8s            kube.Interface
	istioClientset istio.Interface
	// Used for portforwarding requests.
	restConfig *rest.Config
	// Used in REST queries after bump to client-go v0.20.x
	ctx context.Context
	// isOpenShift private variable will check if kiali is deployed under an OpenShift cluster or not
	// It is represented as a pointer to include the initialization phase.
	// See kubernetes_service.go#IsOpenShift() for more details.
	isOpenShift *bool
	// isGatewayAPI private variable will check if K8s Gateway API CRD exists on cluster or not
	isGatewayAPI *bool
	gatewayapi   gatewayapiclient.Interface
	isIstioAPI   *bool
	clusterInfo  ClusterInfo

	// mutex to acquire if you want to access or modify any field in K8SClient concurrently
	rwMutex sync.RWMutex

	// Separated out for testing purposes
	getPodPortForwarderFunc func(namespace, name, portMap string) (httputil.PortForwarder, error)
}

// Ensure the K8SClient implements the ClientInterface
var _ ClientInterface = &K8SClient{}

// GetToken returns the BearerToken used from the config
func (client *K8SClient) GetToken() string {
	return client.token
}

func (client *K8SClient) ClusterInfo() ClusterInfo {
	return client.clusterInfo
}

func NewClientWithRemoteClusterInfo(config *rest.Config, remoteClusterInfo *RemoteClusterInfo) (*K8SClient, error) {
	client, err := NewClientFromConfig(config)
	if err != nil {
		return nil, err
	}

	if remoteClusterInfo != nil {
		cfg, err := remoteClusterInfo.Config.RawConfig()
		if err != nil {
			return nil, err
		}

		clusterName := getClusterName(&cfg)
		client.clusterInfo = ClusterInfo{
			Name:       clusterName,
			SecretName: remoteClusterInfo.SecretName,
		}
	} else {
		client.clusterInfo = ClusterInfo{
			Name: kialiconfig.Get().KubernetesConfig.ClusterName,
		}
	}
	// Copy config
	clientConfig := *config
	client.clusterInfo.ClientConfig = &clientConfig

	return client, nil
}

// getConfigForLocalCluster return a client with the correct configuration
// Returns configuration if Kiali is in Cluster when InCluster is true
// Returns configuration if Kiali is not in Cluster when InCluster is false
// It returns an error on any problem
func getConfigForLocalCluster() (*rest.Config, error) {
	remoteSecretPath := kialiconfig.Get().Deployment.RemoteSecretPath
	if remoteSecret, readErr := GetRemoteSecret(remoteSecretPath); readErr == nil {
		log.Debugf("Using remote secret for local cluster config found at: [%s]. Kiali must be running outside the kube cluster.", remoteSecretPath)
		return clientcmd.NewDefaultClientConfig(*remoteSecret, nil).ClientConfig()
	} else {
		log.Debugf("Unable to read remote secret. It may or may not exist. Error: %v. Falling back to in cluster config", readErr)
		// Fallback to in cluster config
		incluster, err := rest.InClusterConfig()
		if err != nil {
			log.Errorf("Error '%v' getting config for local cluster", err.Error())
			return nil, err
		}
		return incluster, nil
	}
}

// NewClientFromConfig creates a new client to the Kubernetes and Istio APIs.
// It takes the assumption that Istio is deployed into the cluster.
// It hides the access to Kubernetes/Openshift credentials.
// It hides the low level use of the API of Kubernetes and Istio, it should be considered as an implementation detail.
// It returns an error on any problem.
func NewClientFromConfig(config *rest.Config) (*K8SClient, error) {
	client := K8SClient{
		token: config.BearerToken,
	}

	log.Debugf("Rest perf config QPS: %f Burst: %d", config.QPS, config.Burst)

	k8s, err := kube.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	client.k8s = k8s
	client.restConfig = config

	client.istioClientset, err = istio.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	client.gatewayapi, err = gatewayapiclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	client.ctx = context.Background()

	return &client, nil
}

// NewClient is just used for testing purposes.
func NewClient(kubeClient kube.Interface, istioClient istio.Interface, gatewayapiClient gatewayapiclient.Interface) *K8SClient {
	return &K8SClient{
		istioClientset: istioClient,
		k8s:            kubeClient,
		gatewayapi:     gatewayapiClient,
	}
}
