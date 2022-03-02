package references

import (
	"testing"

	"github.com/stretchr/testify/assert"
	networking_v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	security_v1beta "istio.io/client-go/pkg/apis/security/v1beta1"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/models"
)

func prepareTestForServiceEntry(ap *security_v1beta.AuthorizationPolicy, dr *networking_v1alpha3.DestinationRule, se *networking_v1alpha3.ServiceEntry, sc *networking_v1alpha3.Sidecar) models.IstioReferences {
	drReferences := ServiceEntryReferences{
		Namespace: "bookinfo",
		Namespaces: models.Namespaces{
			{Name: "bookinfo"},
			{Name: "bookinfo2"},
			{Name: "bookinfo3"},
		},
		AuthorizationPolicies: []security_v1beta.AuthorizationPolicy{*ap},
		ServiceEntries:        []networking_v1alpha3.ServiceEntry{*se},
		Sidecars:              []networking_v1alpha3.Sidecar{*sc},
		DestinationRules:      []networking_v1alpha3.DestinationRule{*dr},
	}
	return *drReferences.References()[models.IstioReferenceKey{ObjectType: "serviceentry", Namespace: se.Namespace, Name: se.Name}]
}

func TestServiceEntryReferences(t *testing.T) {
	assert := assert.New(t)
	conf := config.NewConfig()
	config.Set(conf)

	// Setup mocks
	references := prepareTestForServiceEntry(getAuthPolicy(t), getAPDestinationRule(t), getAPServiceEntry(t), getSidecar(t))
	assert.Empty(references.ServiceReferences)

	// Check Workload references empty
	assert.Empty(references.WorkloadReferences)

	// Check DR and AuthPolicy references
	assert.Len(references.ObjectReferences, 3)
	assert.Equal(references.ObjectReferences[0].Name, "foo-dev")
	assert.Equal(references.ObjectReferences[0].Namespace, "istio-system")
	assert.Equal(references.ObjectReferences[0].ObjectType, "destinationrule")

	assert.Equal(references.ObjectReferences[1].Name, "foo-sidecar")
	assert.Equal(references.ObjectReferences[1].Namespace, "istio-system")
	assert.Equal(references.ObjectReferences[1].ObjectType, "sidecar")

	assert.Equal(references.ObjectReferences[2].Name, "allow-foo")
	assert.Equal(references.ObjectReferences[2].Namespace, "istio-system")
	assert.Equal(references.ObjectReferences[2].ObjectType, "authorizationpolicy")
}

func TestServiceEntryNoReferences(t *testing.T) {
	assert := assert.New(t)
	conf := config.NewConfig()
	config.Set(conf)

	// Setup mocks
	references := prepareTestForServiceEntry(getAuthPolicy(t), getAPDestinationRule(t), fakeServiceEntry(), getSidecar(t))
	assert.Empty(references.ServiceReferences)
	assert.Empty(references.WorkloadReferences)
	assert.Empty(references.ObjectReferences)
}

func getAPDestinationRule(t *testing.T) *networking_v1alpha3.DestinationRule {
	loader := yamlFixtureLoader("auth-policy.yaml")
	err := loader.Load()
	if err != nil {
		t.Error("Error loading test data.")
	}

	return loader.FindDestinationRule("foo-dev", "istio-system")
}
