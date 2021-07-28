package checkers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/tests/data"
	"github.com/kiali/kiali/tests/testutils/validations"
)

func TestNoCrashOnNil(t *testing.T) {
	assert := assert.New(t)

	typeValidations := NoServiceChecker{
		Namespace:    "test",
		IstioDetails: nil,
		Services:     nil,
	}.Check()

	assert.Empty(typeValidations)
}

func TestAllIstioObjectWithServices(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)

	assert := assert.New(t)

	vals := NoServiceChecker{
		Namespace: "test",
		WorkloadList: data.CreateWorkloadList("test",
			data.CreateWorkloadListItem("reviewsv1", appVersionLabel("reviews", "v1")),
			data.CreateWorkloadListItem("reviewsv2", appVersionLabel("reviews", "v2")),
			data.CreateWorkloadListItem("detailsv1", appVersionLabel("details", "v1")),
			data.CreateWorkloadListItem("detailsv2", appVersionLabel("details", "v2")),
			data.CreateWorkloadListItem("productv1", appVersionLabel("product", "v1")),
			data.CreateWorkloadListItem("productv2", appVersionLabel("product", "v2")),
			data.CreateWorkloadListItem("customerv1", appVersionLabel("customer", "v1")),
			data.CreateWorkloadListItem("customerv2", appVersionLabel("customer", "v2")),
		),
		IstioDetails:         fakeIstioDetails(),
		Services:             fakeServiceDetails([]string{"reviews", "details", "product", "customer"}),
		AuthorizationDetails: &kubernetes.RBACDetails{},
	}.Check()

	assert.NotEmpty(vals)
	assert.NotEmpty(vals[models.IstioValidationKey{ObjectType: "virtualservice", Namespace: "test", Name: "product-vs"}])
	assert.NotEmpty(vals[models.IstioValidationKey{ObjectType: "destinationrule", Namespace: "test", Name: "customer-dr"}])
	assert.True(vals[models.IstioValidationKey{ObjectType: "virtualservice", Namespace: "test", Name: "product-vs"}].Valid)
	assert.True(vals[models.IstioValidationKey{ObjectType: "destinationrule", Namespace: "test", Name: "customer-dr"}].Valid)
}

func TestDetectObjectWithoutService(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)

	assert := assert.New(t)

	vals := NoServiceChecker{
		Namespace:    "test",
		IstioDetails: fakeIstioDetails(),
		WorkloadList: data.CreateWorkloadList("test",
			data.CreateWorkloadListItem("reviewsv1", appVersionLabel("reviews", "v1")),
			data.CreateWorkloadListItem("reviewsv2", appVersionLabel("reviews", "v2")),
			data.CreateWorkloadListItem("detailsv1", appVersionLabel("details", "v1")),
			data.CreateWorkloadListItem("detailsv2", appVersionLabel("details", "v2")),
			data.CreateWorkloadListItem("productv1", appVersionLabel("product", "v1")),
			data.CreateWorkloadListItem("productv2", appVersionLabel("product", "v2")),
		),
		Services:             fakeServiceDetails([]string{"reviews", "details", "product"}),
		AuthorizationDetails: &kubernetes.RBACDetails{},
	}.Check()

	assert.NotEmpty(vals)
	assert.True(vals[models.IstioValidationKey{ObjectType: "virtualservice", Namespace: "test", Name: "product-vs"}].Valid)
	customerDr := vals[models.IstioValidationKey{ObjectType: "destinationrule", Namespace: "test", Name: "customer-dr"}]
	assert.False(customerDr.Valid)
	assert.Equal(1, len(customerDr.Checks))
	assert.Equal("spec/host", customerDr.Checks[0].Path)
	assert.NoError(validations.ConfirmIstioCheckMessage("destinationrules.nodest.matchingregistry", customerDr.Checks[0]))

	vals = NoServiceChecker{
		Namespace: "test",
		WorkloadList: data.CreateWorkloadList("test",
			data.CreateWorkloadListItem("reviewsv1", appVersionLabel("reviews", "v1")),
			data.CreateWorkloadListItem("reviewsv2", appVersionLabel("reviews", "v2")),
			data.CreateWorkloadListItem("detailsv1", appVersionLabel("details", "v1")),
			data.CreateWorkloadListItem("detailsv2", appVersionLabel("details", "v2")),
			data.CreateWorkloadListItem("customerv1", appVersionLabel("customer", "v1")),
			data.CreateWorkloadListItem("customerv2", appVersionLabel("customer", "v2")),
		),
		IstioDetails:         fakeIstioDetails(),
		Services:             fakeServiceDetails([]string{"reviews", "details", "customer"}),
		AuthorizationDetails: &kubernetes.RBACDetails{},
	}.Check()

	assert.NotEmpty(vals)
	assert.True(vals[models.IstioValidationKey{ObjectType: "destinationrule", Namespace: "test", Name: "customer-dr"}].Valid)
	productVs := vals[models.IstioValidationKey{ObjectType: "virtualservice", Namespace: "test", Name: "product-vs"}]
	assert.False(productVs.Valid)
	assert.Equal(2, len(productVs.Checks))
	assert.Equal("spec/http[0]/route[0]/destination/host", productVs.Checks[0].Path)
	assert.NoError(validations.ConfirmIstioCheckMessage("virtualservices.nohost.hostnotfound", productVs.Checks[0]))
	assert.Equal("spec/tcp[0]/route[0]/destination/host", productVs.Checks[1].Path)
	assert.NoError(validations.ConfirmIstioCheckMessage("virtualservices.nohost.hostnotfound", productVs.Checks[1]))

	vals = NoServiceChecker{
		Namespace: "test",
		WorkloadList: data.CreateWorkloadList("test",
			data.CreateWorkloadListItem("reviewsv1", appVersionLabel("reviews", "v1")),
			data.CreateWorkloadListItem("reviewsv2", appVersionLabel("reviews", "v2")),
			data.CreateWorkloadListItem("productv1", appVersionLabel("product", "v1")),
			data.CreateWorkloadListItem("productv2", appVersionLabel("product", "v2")),
			data.CreateWorkloadListItem("customerv1", appVersionLabel("customer", "v1")),
			data.CreateWorkloadListItem("customerv2", appVersionLabel("customer", "v2")),
		),
		IstioDetails:         fakeIstioDetails(),
		Services:             fakeServiceDetails([]string{"reviews", "product", "customer"}),
		AuthorizationDetails: &kubernetes.RBACDetails{},
	}.Check()

	assert.NotEmpty(vals)
	assert.True(vals[models.IstioValidationKey{ObjectType: "destinationrule", Namespace: "test", Name: "customer-dr"}].Valid)

	vals = NoServiceChecker{
		Namespace: "test",
		WorkloadList: data.CreateWorkloadList("test",
			data.CreateWorkloadListItem("productv1", appVersionLabel("product", "v1")),
			data.CreateWorkloadListItem("productv2", appVersionLabel("product", "v2")),
			data.CreateWorkloadListItem("detailsv1", appVersionLabel("details", "v1")),
			data.CreateWorkloadListItem("detailsv2", appVersionLabel("details", "v2")),
			data.CreateWorkloadListItem("customerv1", appVersionLabel("customer", "v1")),
			data.CreateWorkloadListItem("customerv2", appVersionLabel("customer", "v2")),
		),
		IstioDetails:         fakeIstioDetails(),
		Services:             fakeServiceDetails([]string{"details", "product", "customer"}),
		AuthorizationDetails: &kubernetes.RBACDetails{},
	}.Check()

	assert.NotEmpty(vals)
	assert.True(vals[models.IstioValidationKey{ObjectType: "destinationrule", Namespace: "test", Name: "customer-dr"}].Valid)
}

func TestObjectWithoutGateway(t *testing.T) {
	conf := config.NewConfig()
	config.Set(conf)
	assert := assert.New(t)

	istioDetails := fakeIstioDetails()
	gateways := make([]interface{}, 1)
	gateways = append(gateways, "non-existant-gateway")

	istioDetails.VirtualServices[0].GetSpec()["gateways"] = gateways
	vals := NoServiceChecker{
		Namespace:            "test",
		IstioDetails:         istioDetails,
		Services:             fakeServiceDetails([]string{"reviews", "product", "customer"}),
		AuthorizationDetails: &kubernetes.RBACDetails{},
	}.Check()

	assert.NotEmpty(vals)

	productVs := vals[models.IstioValidationKey{ObjectType: "virtualservice", Namespace: "test", Name: "product-vs"}]
	assert.False(productVs.Valid)
	assert.NoError(validations.ConfirmIstioCheckMessage("virtualservices.nogateway", productVs.Checks[0]))
}

func fakeIstioDetails() *kubernetes.IstioDetails {
	istioDetails := kubernetes.IstioDetails{}

	istioDetails.VirtualServices = []kubernetes.IstioObject{
		data.AddRoutesToVirtualService("http", data.CreateRoute("product", "v1", -1),
			data.AddRoutesToVirtualService("tcp", data.CreateRoute("product", "v1", -1),
				data.CreateEmptyVirtualService("product-vs", "test", []string{"product"}),
			),
		)}

	istioDetails.DestinationRules = []kubernetes.IstioObject{
		data.CreateEmptyDestinationRule("test", "customer-dr", "customer"),
	}

	return &istioDetails
}

func fakeServiceDetails(services []string) []core_v1.Service {
	items := []core_v1.Service{}
	for _, service := range services {
		items = append(items, core_v1.Service{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: service,
			},
		})
	}
	return items
}

func appVersionLabel(app, version string) map[string]string {
	return map[string]string{
		"app":     app,
		"version": version,
	}
}
