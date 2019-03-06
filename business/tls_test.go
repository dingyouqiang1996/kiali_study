package business

import (
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/tests/data"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCorrectMeshPolicy(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetMeshPolicies", "test").Return(fakeMeshPolicyEmptyMTLS("default"), nil)

	tlsService := TLSService{k8s: k8s}
	meshPolicyEnabled, err := (tlsService).hasMeshPolicyEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(true, meshPolicyEnabled)
}

func TestPolicyWithWrongName(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetMeshPolicies", "test").Return(fakeMeshPolicyEmptyMTLS("wrong-name"), nil)

	tlsService := TLSService{k8s: k8s}
	isGloballyEnabled, err := (tlsService).hasMeshPolicyEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(false, isGloballyEnabled)
}

func TestPolicyWithPermissiveMode(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetMeshPolicies", "test").Return(fakePermissiveMeshPolicy("default"), nil)

	tlsService := TLSService{k8s: k8s}
	isGloballyEnabled, err := (tlsService).hasMeshPolicyEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(false, isGloballyEnabled)
}

func TestPolicyWithStrictMode(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetMeshPolicies", "test").Return(fakeStrictMeshPolicy("default"), nil)

	tlsService := TLSService{k8s: k8s}
	isGloballyEnabled, err := (tlsService).hasMeshPolicyEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(true, isGloballyEnabled)
}

func fakeMeshPolicyEmptyMTLS(name string) []kubernetes.IstioObject {
	mtls := []interface{}{
		map[string]interface{}{
			"mtls": nil,
		},
	}
	return fakeMeshPolicy(name, mtls)
}

func fakePermissiveMeshPolicy(name string) []kubernetes.IstioObject {
	return fakeMeshPolicyWithMtlsMode(name, "PERMISSIVE")
}

func fakeStrictMeshPolicy(name string) []kubernetes.IstioObject {
	return fakeMeshPolicyWithMtlsMode(name, "STRICT")
}

func fakeMeshPolicyWithMtlsMode(name, mTLSmode string) []kubernetes.IstioObject {
	mtls := []interface{}{
		map[string]interface{}{
			"mtls": map[string]interface{}{
				"mode": mTLSmode,
			},
		},
	}
	return fakeMeshPolicy(name, mtls)
}

func fakeMeshPolicy(name string, peers []interface{}) []kubernetes.IstioObject {
	return []kubernetes.IstioObject{data.CreateEmptyMeshPolicy(name, peers)}
}

func TestWithoutMeshPolicy(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetMeshPolicies", "test").Return([]kubernetes.IstioObject{}, nil)

	tlsService := TLSService{k8s: k8s}
	meshPolicyEnabled, err := (tlsService).hasMeshPolicyEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(false, meshPolicyEnabled)
}

func TestMeshPolicyWithTargets(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetMeshPolicies", "test").Return(fakeMeshPolicyEnablingMTLSSpecificTarget(), nil)

	tlsService := TLSService{k8s: k8s}
	meshPolicyEnabled, err := (tlsService).hasMeshPolicyEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(false, meshPolicyEnabled)
}

func fakeMeshPolicyEnablingMTLSSpecificTarget() []kubernetes.IstioObject {
	targets := []interface{}{
		map[string]interface{}{
			"name": "productpage",
		},
	}

	mtls := []interface{}{
		map[string]interface{}{
			"mtls": "",
		},
	}

	policy := data.AddTargetsToMeshPolicy(targets,
		data.CreateEmptyMeshPolicy("non-global-tls-enabled", mtls))

	return []kubernetes.IstioObject{policy}
}

func TestDestinationRuleEnabled(t *testing.T) {
	assert := assert.New(t)

	dr := data.AddTrafficPolicyToDestinationRule(data.CreateMTLSTrafficPolicyForDestinationRules(),
		data.CreateEmptyDestinationRule("istio-system", "default", "*.local"))

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetDestinationRules", "test", "").Return([]kubernetes.IstioObject{dr}, nil)

	tlsService := TLSService{k8s: k8s}
	drEnabled, err := (tlsService).hasDestinationRuleEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(true, drEnabled)
}

func TestDRWildcardLocalHost(t *testing.T) {
	assert := assert.New(t)

	dr := data.AddTrafficPolicyToDestinationRule(data.CreateMTLSTrafficPolicyForDestinationRules(),
		data.CreateEmptyDestinationRule("myproject", "default", "sleep.foo.svc.cluster.local"))

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetDestinationRules", "test", "").Return([]kubernetes.IstioObject{dr}, nil)

	tlsService := TLSService{k8s: k8s}
	drEnabled, err := (tlsService).hasDestinationRuleEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(false, drEnabled)
}

func TestDRNotMutualTLSMode(t *testing.T) {
	assert := assert.New(t)

	trafficPolicy := map[string]interface{}{
		"tls": map[string]interface{}{
			"mode": "SIMPLE",
		},
	}

	dr := data.AddTrafficPolicyToDestinationRule(trafficPolicy,
		data.CreateEmptyDestinationRule("istio-system", "default", "*.local"))

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetDestinationRules", "test", "").Return([]kubernetes.IstioObject{dr}, nil)

	tlsService := TLSService{k8s: k8s}
	drEnabled, err := (tlsService).hasDestinationRuleEnabled([]string{"test"})

	assert.NoError(err)
	assert.Equal(false, drEnabled)
}

func TestMeshStatusEnabled(t *testing.T) {
	assert := assert.New(t)

	dr := data.AddTrafficPolicyToDestinationRule(data.CreateMTLSTrafficPolicyForDestinationRules(),
		data.CreateEmptyDestinationRule("istio-system", "default", "*.local"))

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetDestinationRules", "test", "").Return([]kubernetes.IstioObject{dr}, nil)
	k8s.On("GetMeshPolicies", "test").Return(fakeMeshPolicyEmptyMTLS("default"), nil)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).MeshWidemTLSStatus([]string{"test"})

	assert.NoError(err)
	assert.Equal(MeshmTLSEnabled, status)
}

func TestMeshStatusPartiallyEnabled(t *testing.T) {
	assert := assert.New(t)

	dr := data.AddTrafficPolicyToDestinationRule(data.CreateMTLSTrafficPolicyForDestinationRules(),
		data.CreateEmptyDestinationRule("istio-system", "default", "sleep.foo.svc.cluster.local"))

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetDestinationRules", "test", "").Return([]kubernetes.IstioObject{dr}, nil)
	k8s.On("GetMeshPolicies", "test").Return(fakeMeshPolicyEmptyMTLS("default"), nil)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).MeshWidemTLSStatus([]string{"test"})

	assert.NoError(err)
	assert.Equal(MeshmTLSPartiallyEnabled, status)
}

func TestMeshStatusNotEnabled(t *testing.T) {
	assert := assert.New(t)

	dr := data.AddTrafficPolicyToDestinationRule(data.CreateMTLSTrafficPolicyForDestinationRules(),
		data.CreateEmptyDestinationRule("istio-system", "default", "sleep.foo.svc.cluster.local"))

	k8s := new(kubetest.K8SClientMock)
	k8s.On("GetDestinationRules", "test", "").Return([]kubernetes.IstioObject{dr}, nil)
	k8s.On("GetMeshPolicies", "test").Return(fakeMeshPolicyEmptyMTLS("wrong-name"), nil)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).MeshWidemTLSStatus([]string{"test"})

	assert.NoError(err)
	assert.Equal(MeshmTLSNotEnabled, status)
}

func TestNamespaceHasMTLSEnabled(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).NamespaceWidemTLSStatus("test")

	assert.NoError(err)
	assert.Equal("ENABLED", status)
}

func TestNamespaceHasPolicyDisabled(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).NamespaceWidemTLSStatus("test")

	assert.NoError(err)
	assert.Equal("PARTLY_ENABLED", status)
}

func TestNamespaceHasPolicyEnabledDifferentNs(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).NamespaceWidemTLSStatus("test")

	assert.NoError(err)
	assert.Equal("PARTLY_ENABLED", status)
}

func TestNamespaceHasDestinationRuleDisabled(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).NamespaceWidemTLSStatus("test")

	assert.NoError(err)
	assert.Equal("PARTLY_ENABLED", status)
}

func TestNamespaceHasDestinationRuleEnabledDifferentNs(t *testing.T) {
	assert := assert.New(t)

	k8s := new(kubetest.K8SClientMock)

	tlsService := TLSService{k8s: k8s}
	status, err := (tlsService).NamespaceWidemTLSStatus("test")

	assert.NoError(err)
	assert.Equal("PARTLY_ENABLED", status)
}
