package business_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	"github.com/kiali/kiali/business"
	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/cache"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/models"
)

func TestGetClustersResolvesTheKialiCluster(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	conf := config.NewConfig()
	conf.InCluster = false
	conf.KubernetesConfig.ClusterName = "KialiCluster"
	config.Set(conf)

	istioDeploymentMock := apps_v1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istiod",
			Namespace: "istio-system",
		},
		Spec: apps_v1.DeploymentSpec{
			Template: core_v1.PodTemplateSpec{
				Spec: core_v1.PodSpec{
					Containers: []core_v1.Container{
						{
							Env: []core_v1.EnvVar{
								{
									Name:  "CLUSTER_ID",
									Value: "KialiCluster",
								},
								{
									Name:  "PILOT_SCOPE_GATEWAY_TO_NAMESPACE",
									Value: "true",
								},
							},
						},
					},
				},
			},
		},
	}

	sidecarConfigMapMock := core_v1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istio-sidecar-injector",
			Namespace: "istio-system",
		},
		Data: map[string]string{
			"values": "{ \"global\": { \"network\": \"kialiNetwork\" } }",
		},
	}

	kialiNs := core_v1.Namespace{
		ObjectMeta: v1.ObjectMeta{Name: "foo"},
	}

	kialiSvc := []core_v1.Service{
		{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"operator-sdk/primary-resource": "kiali-operator/myKialiCR",
				},
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "kiali",
					"app.kubernetes.io/version": "v1.25",
				},
				Name:      "kiali-service",
				Namespace: "foo",
			},
		},
	}

	objects := []runtime.Object{
		&istioDeploymentMock,
		&sidecarConfigMapMock,
		&kialiNs,
	}

	for _, obj := range kialiSvc {
		o := obj
		objects = append(objects, &o)
	}

	k8s := kubetest.NewFakeK8sClient(objects...)
	k8s.KubeClusterInfo = kubernetes.ClusterInfo{
		ClientConfig: &rest.Config{
			Host: "http://127.0.0.2:9443",
		},
	}

	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	business.SetupBusinessLayer(t, k8s, *conf)

	layer := business.NewWithBackends(mockClientFactory.Clients, mockClientFactory.Clients, nil, nil)
	meshSvc := layer.Mesh

	r := httptest.NewRequest("GET", "http://kiali.url.local/", nil)
	a, err := meshSvc.GetClusters(r)
	require.Nil(err, "GetClusters returned error: %v", err)

	require.NotNil(a, "GetClusters returned nil")
	require.Len(a, 1, "GetClusters didn't resolve the Kiali cluster")
	assert.Equal("KialiCluster", a[0].Name, "Unexpected cluster name")
	assert.True(a[0].IsKialiHome, "Kiali cluster not properly marked as such")
	assert.Equal("http://127.0.0.2:9443", a[0].ApiEndpoint)
	assert.Equal("kialiNetwork", a[0].Network)

	require.Len(a[0].KialiInstances, 1, "GetClusters didn't resolve the local Kiali instance")
	assert.Equal("foo", a[0].KialiInstances[0].Namespace, "GetClusters didn't set the right namespace of the Kiali instance")
	assert.Equal("kiali-operator/myKialiCR", a[0].KialiInstances[0].OperatorResource, "GetClusters didn't set the right operator resource of the Kiali instance")
	assert.Equal("http://kiali.url.local", a[0].KialiInstances[0].Url, "GetClusters didn't set the right URL of the Kiali instance")
	assert.Equal("v1.25", a[0].KialiInstances[0].Version, "GetClusters didn't set the right version of the Kiali instance")
	assert.Equal("kiali-service", a[0].KialiInstances[0].ServiceName, "GetClusters didn't set the right service name of the Kiali instance")
}

func TestGetClustersResolvesRemoteClusters(t *testing.T) {
	check := require.New(t)

	conf := config.NewConfig()
	conf.InCluster = false
	config.Set(conf)

	remoteNs := &core_v1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{"topology.istio.io/network": "TheRemoteNetwork"},
			Name:   conf.IstioNamespace,
		},
	}

	kialiSvc := &core_v1.Service{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"operator-sdk/primary-resource": "kiali-operator/myKialiCR",
			},
			Labels: map[string]string{
				"app.kubernetes.io/version": "v1.25",
				"app.kubernetes.io/part-of": "kiali",
			},
			Name:      "kiali-service",
			Namespace: conf.IstioNamespace,
		},
	}

	remoteClient := kubetest.NewFakeK8sClient(remoteNs, kialiSvc)
	remoteClient.KubeClusterInfo = kubernetes.ClusterInfo{
		Name: "KialiCluster",
		ClientConfig: &rest.Config{
			Host: "https://192.168.144.17:123",
			TLSClientConfig: rest.TLSClientConfig{
				CAData: []byte("eAo="),
			},
		},
	}
	clients := map[string]kubernetes.ClientInterface{
		"KialiCluster":                    remoteClient,
		conf.KubernetesConfig.ClusterName: kubetest.NewFakeK8sClient(),
	}
	factory := kubetest.NewK8SClientFactoryMock(nil)
	factory.SetClients(clients)
	business.WithKialiCache(business.NewTestingCacheWithFactory(t, factory, *conf))
	layer := business.NewWithBackends(clients, clients, nil, nil)
	meshSvc := layer.Mesh

	a, err := meshSvc.GetClusters(nil)
	check.Nil(err, "GetClusters returned error: %v", err)

	remoteCluster := business.FindOrFail(t, a, func(c kubernetes.Cluster) bool { return c.Name == "KialiCluster" })

	check.NotNil(a, "GetClusters returned nil")
	check.Len(a, 2, "GetClusters didn't resolve the remote clusters")
	check.Equal("KialiCluster", remoteCluster.Name, "Unexpected cluster name")
	check.False(remoteCluster.IsKialiHome, "Remote cluster mistakenly marked as the Kiali home")
	check.Equal("https://192.168.144.17:123", remoteCluster.ApiEndpoint)
	check.Equal("TheRemoteNetwork", remoteCluster.Network)

	check.Len(remoteCluster.KialiInstances, 1, "GetClusters didn't resolve the remote Kiali instance")
	check.Equal(conf.IstioNamespace, remoteCluster.KialiInstances[0].Namespace, "GetClusters didn't set the right namespace of the Kiali instance")
	check.Equal("kiali-operator/myKialiCR", remoteCluster.KialiInstances[0].OperatorResource, "GetClusters didn't set the right operator resource of the Kiali instance")
	check.Equal("", remoteCluster.KialiInstances[0].Url, "GetClusters didn't set the right URL of the Kiali instance")
	check.Equal("v1.25", remoteCluster.KialiInstances[0].Version, "GetClusters didn't set the right version of the Kiali instance")
	check.Equal("kiali-service", remoteCluster.KialiInstances[0].ServiceName, "GetClusters didn't set the right service name of the Kiali instance")
}

type fakeClusterCache struct {
	cache.KialiCache
	clusters []kubernetes.Cluster
}

func (f *fakeClusterCache) GetClusters() []kubernetes.Cluster {
	return f.clusters
}

func (f *fakeClusterCache) SetClusters(clusters []kubernetes.Cluster) {
	f.clusters = clusters
}

// TestGetClusters verifies that GetClusters is properly caching it's findings and the cached value is being returned.
func TestResolveKialiControlPlaneClusterIsCached(t *testing.T) {
	check := assert.New(t)
	require := require.New(t)

	// Prepare mocks for first time call.
	conf := config.NewConfig()
	conf.IstioNamespace = "foo"
	conf.ExternalServices.Istio.IstiodDeploymentName = "bar"
	conf.KubernetesConfig.ClusterName = "KialiCluster"
	config.Set(conf)

	istioDeploymentMock := &apps_v1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: apps_v1.DeploymentSpec{
			Template: core_v1.PodTemplateSpec{
				Spec: core_v1.PodSpec{
					Containers: []core_v1.Container{
						{
							Env: []core_v1.EnvVar{
								{
									Name:  "CLUSTER_ID",
									Value: "KialiCluster",
								},
								{
									Name:  "PILOT_SCOPE_GATEWAY_TO_NAMESPACE",
									Value: "false",
								},
							},
						},
					},
				},
			},
		},
	}

	kialiSvc := &core_v1.Service{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"operator-sdk/primary-resource": "kiali-operator/myKialiCR",
			},
			Labels: map[string]string{
				"app.kubernetes.io/version": "v1.25",
				"app.kubernetes.io/part-of": "kiali",
			},
			Name:      "kiali-service",
			Namespace: "foo",
		},
	}

	// Create a MeshService and invoke GetClusters. This should cache the result.
	k8s := kubetest.NewFakeK8sClient(
		&core_v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "foo"}},
		istioDeploymentMock,
		kialiSvc,
	)
	cache := business.SetupBusinessLayer(t, k8s, *conf)
	getClustersCache := &fakeClusterCache{KialiCache: cache}
	business.WithKialiCache(getClustersCache)
	clients := map[string]kubernetes.ClientInterface{
		conf.KubernetesConfig.ClusterName: k8s,
	}
	layer := business.NewWithBackends(clients, clients, nil, nil)
	meshSvc := layer.Mesh
	result, err := meshSvc.GetClusters(nil)
	require.NoError(err)
	require.NotNil(result)
	require.Len(result, 1)
	check.Equal("KialiCluster", result[0].Name) // Sanity check. Rest of values are tested in TestGetClustersResolvesTheKialiCluster
	// Check that the cache now has clusters populated.
	check.Len(getClustersCache.clusters, 1)

	result, err = meshSvc.GetClusters(nil)
	require.NoError(err)
	require.NotNil(result)
	require.Len(result, 1)
	check.Equal("KialiCluster", result[0].Name)
}

// TestCanaryUpgradeNotConfigured verifies that when there is no canary upgrade configured, both the migrated and pending namespace lists are empty
func TestCanaryUpgradeNotConfigured(t *testing.T) {
	check := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioCanaryRevision.Current = "default"
	conf.ExternalServices.Istio.IstioCanaryRevision.Upgrade = "canary"

	config.Set(conf)

	k8s.On("IsOpenShift").Return(false)
	k8s.On("IsGatewayAPI").Return(false)
	k8s.On("GetNamespaces", "istio-injection=enabled").Return([]core_v1.Namespace{}, nil)
	k8s.On("GetNamespaces", "istio.io/rev=default").Return([]core_v1.Namespace{}, nil)
	k8s.On("GetNamespaces", "istio.io/rev=canary").Return([]core_v1.Namespace{}, nil)

	// Create a MeshService and invoke IsMeshConfigured
	k8sclients := make(map[string]kubernetes.ClientInterface)
	k8sclients[conf.KubernetesConfig.ClusterName] = k8s
	layer := business.NewWithBackends(k8sclients, k8sclients, nil, nil)
	meshSvc := layer.Mesh

	canaryUpgradeStatus, err := meshSvc.CanaryUpgradeStatus()

	check.Nil(err, "IstiodCanariesStatus failed: %s", err)
	check.NotNil(canaryUpgradeStatus)
}

// TestCanaryUpgradeConfigured verifies that when there is a canary upgrade in place, the migrated and pending namespaces should have namespaces
func TestCanaryUpgradeConfigured(t *testing.T) {
	check := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioCanaryRevision.Current = "default"
	conf.ExternalServices.Istio.IstioCanaryRevision.Upgrade = "canary"

	config.Set(conf)

	k8s.On("IsOpenShift").Return(false)
	k8s.On("IsGatewayAPI").Return(false)

	migratedNamespace := core_v1.Namespace{
		ObjectMeta: v1.ObjectMeta{Name: "travel-agency"},
	}
	migratedNamespaces := []core_v1.Namespace{migratedNamespace}

	pendingNamespace := core_v1.Namespace{
		ObjectMeta: v1.ObjectMeta{Name: "travel-portal"},
	}
	pendingNamespaces := []core_v1.Namespace{pendingNamespace}

	k8s.On("GetNamespaces", "istio-injection=enabled").Return(pendingNamespaces, nil)
	k8s.On("GetNamespaces", "istio.io/rev=default").Return([]core_v1.Namespace{}, nil)
	k8s.On("GetNamespaces", "istio.io/rev=canary").Return(migratedNamespaces, nil)

	// Create a MeshService and invoke IsMeshConfigured
	k8sclients := make(map[string]kubernetes.ClientInterface)
	k8sclients[conf.KubernetesConfig.ClusterName] = k8s
	layer := business.NewWithBackends(k8sclients, k8sclients, nil, nil)
	meshSvc := layer.Mesh

	canaryUpgradeStatus, err := meshSvc.CanaryUpgradeStatus()

	check.Nil(err, "IstiodCanariesStatus failed: %s", err)
	check.Contains(canaryUpgradeStatus.MigratedNamespaces, "travel-agency")
	check.Equal(1, len(canaryUpgradeStatus.MigratedNamespaces))
	check.Contains(canaryUpgradeStatus.PendingNamespaces, "travel-portal")
	check.Equal(1, len(canaryUpgradeStatus.PendingNamespaces))
}

func TestIstiodResourceThresholds(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)

	testCases := map[string]struct {
		istiodConatiner core_v1.Container
		istiodMeta      v1.ObjectMeta
		expected        *models.IstiodThresholds
		expectedErr     error
	}{
		"istiod with no limits": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
			},
			expected: &models.IstiodThresholds{
				CPU:    0,
				Memory: 0,
			},
		},
		"istiod with limits": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
				Resources: core_v1.ResourceRequirements{
					Limits: core_v1.ResourceList{
						core_v1.ResourceCPU:    resource.MustParse("1000m"),
						core_v1.ResourceMemory: resource.MustParse("1G"),
					},
				},
			},
			expected: &models.IstiodThresholds{
				CPU:    1,
				Memory: 1000,
			},
		},
		"istiod with binary limits": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
				Resources: core_v1.ResourceRequirements{
					Limits: core_v1.ResourceList{
						core_v1.ResourceCPU:    resource.MustParse("14m"),
						core_v1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
			expected: &models.IstiodThresholds{
				CPU: 0.014,
				// Rounded
				Memory: 1074,
			},
		},
		"istiod with cpu numeral": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
				Resources: core_v1.ResourceRequirements{
					Limits: core_v1.ResourceList{
						core_v1.ResourceCPU: resource.MustParse("1.5"),
					},
				},
			},
			expected: &models.IstiodThresholds{
				CPU: 1.5,
			},
		},
		"istiod with only cpu limits": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
				Resources: core_v1.ResourceRequirements{
					Limits: core_v1.ResourceList{
						core_v1.ResourceCPU: resource.MustParse("1000m"),
					},
				},
			},
			expected: &models.IstiodThresholds{
				CPU: 1,
			},
		},
		"istiod with only memory limits": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
				Resources: core_v1.ResourceRequirements{
					Limits: core_v1.ResourceList{
						core_v1.ResourceMemory: resource.MustParse("1G"),
					},
				},
			},
			expected: &models.IstiodThresholds{
				Memory: 1000,
			},
		},
		"istiod with different name": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod-rev-1",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
				Resources: core_v1.ResourceRequirements{
					Limits: core_v1.ResourceList{
						core_v1.ResourceMemory: resource.MustParse("1G"),
					},
				},
			},
			expectedErr: fmt.Errorf("istiod deployment not found"),
		},
		"istiod in a different namespace": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system-2",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
				Resources: core_v1.ResourceRequirements{
					Limits: core_v1.ResourceList{
						core_v1.ResourceMemory: resource.MustParse("1G"),
					},
				},
			},
			expectedErr: fmt.Errorf("istiod deployment not found"),
		},
		"Missing limits": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name:      "istiod",
				Resources: core_v1.ResourceRequirements{},
			},
			expected: &models.IstiodThresholds{},
		},
		"Missing resources": {
			istiodMeta: v1.ObjectMeta{
				Name:      "istiod",
				Namespace: "istio-system",
			},
			istiodConatiner: core_v1.Container{
				Name: "istiod",
			},
			expected: &models.IstiodThresholds{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			istiodDeployment := &apps_v1.Deployment{
				ObjectMeta: testCase.istiodMeta,
				Spec: apps_v1.DeploymentSpec{
					Template: core_v1.PodTemplateSpec{
						Spec: core_v1.PodSpec{
							Containers: []core_v1.Container{
								testCase.istiodConatiner,
							},
						},
					},
				},
			}
			k8s := kubetest.NewFakeK8sClient(
				&core_v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "istio-system"}},
				istiodDeployment,
			)

			business.SetupBusinessLayer(t, k8s, *config.NewConfig())

			clients := map[string]kubernetes.ClientInterface{conf.KubernetesConfig.ClusterName: k8s}
			ms := business.NewWithBackends(clients, clients, nil, nil).Mesh

			actual, err := ms.IstiodResourceThresholds()

			if testCase.expectedErr != nil {
				require.Error(err)
				// End the test early if we expect an error.
				return
			}

			require.NoError(err)
			require.Equal(testCase.expected, actual)
		})
	}
}

func TestGetMesh(t *testing.T) {
	require := require.New(t)
	conf := config.NewConfig()

	istiodDeployment := &apps_v1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istiod",
			Namespace: "istio-system",
			Labels:    map[string]string{"app": "istiod"},
		},
	}
	const configMapData = `accessLogFile: /dev/stdout
enableAutoMtls: true
rootNamespace: istio-system
trustDomain: cluster.local
`
	istioConfigMap := &core_v1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istio",
			Namespace: "istio-system",
		},
		Data: map[string]string{"mesh": configMapData},
	}
	k8s := kubetest.NewFakeK8sClient(
		&core_v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "istio-system"}},
		istiodDeployment,
		istioConfigMap,
	)

	business.SetupBusinessLayer(t, k8s, *config.NewConfig())

	clients := map[string]kubernetes.ClientInterface{conf.KubernetesConfig.ClusterName: k8s}
	svc := business.NewWithBackends(clients, clients, nil, nil).Mesh
	mesh, err := svc.GetMesh(context.TODO())
	require.NoError(err)
	require.Len(mesh.ControlPlanes, 1)
	require.True(*mesh.ControlPlanes[0].Config.EnableAutoMtls)
	require.Len(mesh.ControlPlanes[0].ManagedClusters, 1)
}

func TestGetMeshMultipleRevisions(t *testing.T) {
	istiod_1_18_Deployment := &apps_v1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istiod-1-18",
			Namespace: "istio-system",
			Labels: map[string]string{
				"app":                     "istiod",
				models.IstioRevisionLabel: "1-18-0",
			},
		},
	}
	istiod_1_19_Deployment := &apps_v1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istiod-1-19",
			Namespace: "istio-system",
			Labels: map[string]string{
				"app":                     "istiod",
				models.IstioRevisionLabel: "1-19-0",
			},
		},
	}
	const configMap_1_18_Data = `accessLogFile: /dev/stdout
enableAutoMtls: false
rootNamespace: istio-system
trustDomain: cluster.local
`
	istio_1_18_ConfigMap := &core_v1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istio-1-18-0",
			Namespace: "istio-system",
		},
		Data: map[string]string{"mesh": configMap_1_18_Data},
	}
	const configMap_1_19_Data = `accessLogFile: /dev/stdout
enableAutoMtls: true
rootNamespace: istio-system
trustDomain: cluster.local
`
	istio_1_19_ConfigMap := &core_v1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istio-1-19-0",
			Namespace: "istio-system",
		},
		Data: map[string]string{"mesh": configMap_1_19_Data},
	}
	require := require.New(t)
	conf := config.NewConfig()
	k8s := kubetest.NewFakeK8sClient(
		&core_v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "istio-system"}},
		istiod_1_18_Deployment,
		istio_1_18_ConfigMap,
		istiod_1_19_Deployment,
		istio_1_19_ConfigMap,
	)

	business.SetupBusinessLayer(t, k8s, *config.NewConfig())

	clients := map[string]kubernetes.ClientInterface{conf.KubernetesConfig.ClusterName: k8s}
	svc := business.NewWithBackends(clients, clients, nil, nil).Mesh
	mesh, err := svc.GetMesh(context.TODO())
	require.NoError(err)
	require.Len(mesh.ControlPlanes, 2)

	controlPlane_1_18 := business.FindOrFail(t, mesh.ControlPlanes, func(c business.ControlPlane) bool {
		return c.Revision == "1-18-0"
	})
	require.False(*controlPlane_1_18.Config.EnableAutoMtls)
	require.Len(controlPlane_1_18.ManagedClusters, 1)

	controlPlane_1_19 := business.FindOrFail(t, mesh.ControlPlanes, func(c business.ControlPlane) bool {
		return c.Revision == "1-19-0"
	})
	require.True(*controlPlane_1_19.Config.EnableAutoMtls)
	require.Len(controlPlane_1_19.ManagedClusters, 1)
}

func TestGetMeshRemoteClusters(t *testing.T) {
	require := require.New(t)
	conf := config.NewConfig()
	// Set to east because the default is "" which causes the check for
	// cluster name env var to fail even though it is set.
	conf.KubernetesConfig.ClusterName = "east"
	kubernetes.SetConfig(t, *conf)

	istiodDeployment := fakeIstiodDeployment(conf.KubernetesConfig.ClusterName, true)
	const configMapData = `accessLogFile: /dev/stdout
enableAutoMtls: true
rootNamespace: istio-system
trustDomain: cluster.local
`
	istioConfigMap := &core_v1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istio",
			Namespace: "istio-system",
		},
		Data: map[string]string{"mesh": configMapData},
	}
	k8s := kubetest.NewFakeK8sClient(
		&core_v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "istio-system"}},
		istiodDeployment,
		istioConfigMap,
	)
	remoteClient := kubetest.NewFakeK8sClient(
		&core_v1.Namespace{ObjectMeta: v1.ObjectMeta{
			Name:        "istio-system",
			Labels:      map[string]string{"istio-injection": "enabled"},
			Annotations: map[string]string{models.IstioControlPlaneClustersLabel: conf.KubernetesConfig.ClusterName},
		}},
		&core_v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "bookinfo"}},
	)

	clients := map[string]kubernetes.ClientInterface{conf.KubernetesConfig.ClusterName: k8s, "remote": remoteClient}
	factory := kubetest.NewK8SClientFactoryMock(nil)
	factory.SetClients(clients)
	business.WithKialiCache(business.NewTestingCacheWithFactory(t, factory, *conf))

	svc := business.NewWithBackends(clients, clients, nil, nil).Mesh
	mesh, err := svc.GetMesh(context.TODO())
	require.NoError(err)
	require.Len(mesh.ControlPlanes, 1)
	require.Len(mesh.ControlPlanes[0].ManagedClusters, 2)

	controlPlane := mesh.ControlPlanes[0]

	require.Equal(conf.KubernetesConfig.ClusterName, controlPlane.Cluster.Name)
	business.FindOrFail(t, controlPlane.ManagedClusters, func(c *kubernetes.Cluster) bool {
		return c.Name == "remote"
	})
}

func fakeIstiodDeployment(cluster string, manageExternal bool) *apps_v1.Deployment {
	deployment := &apps_v1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "istiod",
			Namespace: "istio-system",
			Labels:    map[string]string{"app": "istiod"},
		},
		Spec: apps_v1.DeploymentSpec{
			Template: core_v1.PodTemplateSpec{
				Spec: core_v1.PodSpec{
					Containers: []core_v1.Container{
						{
							Name: "discovery",
							Env: []core_v1.EnvVar{
								{
									Name:  "CLUSTER_ID",
									Value: cluster,
								},
							},
						},
					},
				},
			},
		},
	}
	if manageExternal {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, core_v1.EnvVar{
			Name:  "EXTERNAL_ISTIOD",
			Value: "true",
		})
	}
	return deployment
}
