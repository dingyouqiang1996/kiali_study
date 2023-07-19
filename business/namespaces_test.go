package business

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/models"
)

// Namespace service setup
func setupNamespaceService(t *testing.T, k8s kubernetes.ClientInterface, conf *config.Config) NamespaceService {
	cache := NewTestingCache(t, k8s, *conf)

	k8sclients := make(map[string]kubernetes.ClientInterface)
	k8sclients[conf.KubernetesConfig.ClusterName] = k8s
	return NewNamespaceService(k8sclients, k8sclients, cache, *conf)
}

// Namespace service setup
func setupNamespaceServiceWithNs() kubernetes.ClientInterface {
	// config needs to be set by other services since those rely on the global.
	objects := []runtime.Object{
		&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
		&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "alpha"}},
		&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "beta"}},
	}
	for _, obj := range fakeNamespaces() {
		o := obj
		objects = append(objects, &o)
	}
	k8s := kubetest.NewFakeK8sClient(objects...)
	k8s.OpenShift = false
	return k8s
}

// Get namespaces
func TestGetNamespaces(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)

	nsservice := setupNamespaceService(t, k8s, conf)

	ns, _ := nsservice.GetNamespaces(context.TODO())

	assert.NotNil(t, ns)
	assert.Equal(t, len(ns), 5)
	assert.Equal(t, ns[0].Name, "alpha")
}

// Get namespace
func TestGetNamespace(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)

	nsservice := setupNamespaceService(t, k8s, conf)

	ns, _ := nsservice.GetClusterNamespace(context.TODO(), "bookinfo", config.Get().KubernetesConfig.ClusterName)

	assert.NotNil(t, ns)
	assert.Equal(t, ns.Name, "bookinfo")
}

// Get namespace error
func TestGetNamespaceWithError(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)

	nsservice := setupNamespaceService(t, k8s, conf)

	ns2, err := nsservice.GetClusterNamespace(context.TODO(), "fakeNS", config.Get().KubernetesConfig.ClusterName)

	assert.NotNil(t, err)
	assert.Nil(t, ns2)
}

// Update namespaces
func TestUpdateNamespaces(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)

	nsservice := setupNamespaceService(t, k8s, conf)

	ns, err := nsservice.UpdateNamespace(context.TODO(), "bookinfo", `{"metadata": {"labels": {"new": "label"}}}`, conf.KubernetesConfig.ClusterName)

	assert.Nil(t, err)
	assert.NotNil(t, ns)
	assert.Equal(t, ns.Name, "bookinfo")
}

func TestMultiClusterGetNamespace(t *testing.T) {
	require := require.New(t)
	// assert := assert.New(t)

	conf := config.NewConfig()
	conf.KubernetesConfig.ClusterName = "east"
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		"east": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
		),
		"west": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
		),
	}
	clientFactory.SetClients(clients)
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)
	cache := newTestingCache(t, clientFactory, *conf)

	nsservice := NewNamespaceService(clients, clients, cache, *conf)

	ns, err := nsservice.GetClusterNamespace(context.TODO(), "bookinfo", conf.KubernetesConfig.ClusterName)
	require.NoError(err)

	assert.Equal(t, conf.KubernetesConfig.ClusterName, ns.Cluster)
}

func TestMultiClusterGetNamespaces(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	conf := config.NewConfig()
	conf.KubernetesConfig.ClusterName = "east"
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		"east": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
		),
		"west": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
		),
	}
	clientFactory.SetClients(clients)
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)
	cache := newTestingCache(t, clientFactory, *conf)

	nsservice := NewNamespaceService(clients, clients, cache, *conf)
	namespaces, err := nsservice.GetNamespaces(context.TODO())
	require.NoError(err)

	require.Len(namespaces, 2)
	var clusterNames []string
	for _, ns := range namespaces {
		clusterNames = append(clusterNames, ns.Cluster)
	}

	assert.Contains(clusterNames, "east")
	assert.Contains(clusterNames, "west")
}

func TestGetNamespacesCached(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	conf := config.NewConfig()
	conf.KubernetesConfig.ClusterName = "east"
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		"east": k8s,
	}
	clientFactory.SetClients(clients)
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)
	cache := newTestingCache(t, clientFactory, *conf)
	cache.SetNamespaces(
		k8s.GetToken(),
		// gamma is only cached.
		[]models.Namespace{{Name: "bookinfo"}, {Name: "alpha"}, {Name: "beta"}, {Name: "gamma", Cluster: "west"}},
	)

	nsservice := NewNamespaceService(clients, clients, cache, *conf)
	namespaces, err := nsservice.GetNamespaces(context.TODO())
	require.NoError(err)

	require.Len(namespaces, 4)

	namespace, err := nsservice.GetClusterNamespace(context.TODO(), "gamma", "west")
	require.NoError(err)

	assert.Equal("west", namespace.Cluster)
}

type forbiddenFake struct{ kubernetes.ClientInterface }

func (f *forbiddenFake) GetNamespace(namespace string) (*core_v1.Namespace, error) {
	return nil, fmt.Errorf("forbidden")
}

// Tests that GetNamespaces won't return a namespace with the same name from another cluster
// if the user doesn't have access to that cluster but the namespace is cached.
func TestGetNamespacesForbiddenCached(t *testing.T) {
	require := require.New(t)

	conf := config.NewConfig()
	conf.KubernetesConfig.ClusterName = "east"
	config.Set(conf)

	k8s := setupNamespaceServiceWithNs()

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	// Two clusters: one the user has access to, one they don't.
	clients := map[string]kubernetes.ClientInterface{
		"east": &forbiddenFake{k8s},
		"west": k8s,
	}
	clientFactory.SetClients(clients)
	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	SetWithBackends(mockClientFactory, nil)
	cache := newTestingCache(t, clientFactory, *conf)
	cache.SetNamespaces(
		k8s.GetToken(),
		// Bookinfo is cached for the west cluster that the user has access to
		// but NOT for the east cluster that the user doesn't have access to.
		[]models.Namespace{{Name: "bookinfo", Cluster: "west"}},
	)

	nsservice := NewNamespaceService(clients, clients, cache, *conf)
	// Try to get the bookinfo namespace from the home cluster.
	_, err := nsservice.GetClusterNamespace(context.TODO(), "bookinfo", "east")
	require.Error(err)
}

// TODO: Add projects tests
