package checkers

import (
	networking_v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	"github.com/kiali/kiali/business/checkers/destinationrules"
	"github.com/kiali/kiali/business/checkers/virtualservices"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
)

const ServiceRoleCheckerType = "servicerole"

type NoServiceChecker struct {
	Namespace            string
	Namespaces           models.Namespaces
	ExportedResources    *kubernetes.ExportedResources
	WorkloadList         models.WorkloadList
	AuthorizationDetails *kubernetes.RBACDetails
	RegistryServices     []*kubernetes.RegistryService
}

func (in NoServiceChecker) Check() models.IstioValidations {
	validations := models.IstioValidations{}

	if len(in.RegistryServices) == 0 {
		return validations
	}

	serviceHosts := kubernetes.ServiceEntryHostnames(in.ExportedResources.ServiceEntries)
	gatewayNames := kubernetes.GatewayNames(in.ExportedResources.Gateways)

	for _, virtualService := range in.ExportedResources.VirtualServices {
		validations.MergeValidations(runVirtualServiceCheck(virtualService, in.Namespace, serviceHosts, in.Namespaces, in.RegistryServices))

		validations.MergeValidations(runGatewayCheck(virtualService, gatewayNames))
	}
	for _, destinationRule := range in.ExportedResources.DestinationRules {
		validations.MergeValidations(runDestinationRuleCheck(destinationRule, in.Namespace, in.WorkloadList, serviceHosts, in.Namespaces, in.RegistryServices, in.ExportedResources.VirtualServices))
	}
	return validations
}

func runVirtualServiceCheck(virtualService networking_v1alpha3.VirtualService, namespace string, serviceHosts map[string][]string, clusterNamespaces models.Namespaces, registryStatus []*kubernetes.RegistryService) models.IstioValidations {
	key, validations := EmptyValidValidation(virtualService.Name, virtualService.Namespace, VirtualCheckerType)

	result, valid := virtualservices.NoHostChecker{
		Namespace:         namespace,
		Namespaces:        clusterNamespaces,
		VirtualService:    virtualService,
		ServiceEntryHosts: serviceHosts,
		RegistryServices:  registryStatus,
	}.Check()

	validations.Valid = valid
	validations.Checks = result

	return models.IstioValidations{key: validations}
}

func runGatewayCheck(virtualService networking_v1alpha3.VirtualService, gatewayNames map[string]struct{}) models.IstioValidations {
	key, validations := EmptyValidValidation(virtualService.Name, virtualService.Namespace, VirtualCheckerType)

	result, valid := virtualservices.NoGatewayChecker{
		VirtualService: virtualService,
		GatewayNames:   gatewayNames,
	}.Check()

	validations.Valid = valid
	validations.Checks = result

	return models.IstioValidations{key: validations}
}

func runDestinationRuleCheck(destinationRule networking_v1alpha3.DestinationRule, namespace string, workloads models.WorkloadList,
	serviceHosts map[string][]string, clusterNamespaces models.Namespaces, registryStatus []*kubernetes.RegistryService, virtualServices []networking_v1alpha3.VirtualService) models.IstioValidations {
	key, validations := EmptyValidValidation(destinationRule.Name, destinationRule.Namespace, DestinationRuleCheckerType)

	result, valid := destinationrules.NoDestinationChecker{
		Namespace:        namespace,
		Namespaces:       clusterNamespaces,
		WorkloadList:     workloads,
		DestinationRule:  destinationRule,
		VirtualServices:  virtualServices,
		ServiceEntries:   serviceHosts,
		RegistryServices: registryStatus,
	}.Check()

	validations.Valid = valid
	validations.Checks = result

	return models.IstioValidations{key: validations}
}
