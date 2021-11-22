package checkers

import (
	networking_v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	"github.com/kiali/kiali/business/checkers/common"
	"github.com/kiali/kiali/business/checkers/virtualservices"
	"github.com/kiali/kiali/models"
)

const VirtualCheckerType = "virtualservice"

type VirtualServiceChecker struct {
	Namespace                string
	Namespaces               models.Namespaces
	DestinationRules         []networking_v1alpha3.DestinationRule
	VirtualServices          []networking_v1alpha3.VirtualService
	ExportedVirtualServices  []networking_v1alpha3.VirtualService
	ExportedDestinationRules []networking_v1alpha3.DestinationRule
}

// An Object Checker runs all checkers for an specific object type (i.e.: pod, route rule,...)
// It run two kinds of checkers:
// 1. Individual checks: validating individual objects.
// 2. Group checks: validating behaviour between configurations.
func (in VirtualServiceChecker) Check() models.IstioValidations {
	validations := models.IstioValidations{}

	validations = validations.MergeValidations(in.runIndividualChecks())
	validations = validations.MergeValidations(in.runGroupChecks())

	return validations
}

// Runs individual checks for each virtual service
func (in VirtualServiceChecker) runIndividualChecks() models.IstioValidations {
	validations := models.IstioValidations{}

	for _, virtualService := range in.VirtualServices {
		validations.MergeValidations(in.runChecks(virtualService))
	}

	return validations
}

// runGroupChecks runs group checks for all virtual services
func (in VirtualServiceChecker) runGroupChecks() models.IstioValidations {
	validations := models.IstioValidations{}

	enabledCheckers := []GroupChecker{
		virtualservices.SingleHostChecker{Namespace: in.Namespace, Namespaces: in.Namespaces, VirtualServices: in.VirtualServices, ExportedVirtualServices: in.ExportedVirtualServices},
	}

	for _, checker := range enabledCheckers {
		validations = validations.MergeValidations(checker.Check())
	}

	return validations
}

// runChecks runs all the individual checks for a single virtual service and appends the result into validations.
func (in VirtualServiceChecker) runChecks(virtualService networking_v1alpha3.VirtualService) models.IstioValidations {
	virtualServiceName := virtualService.Name
	key, rrValidation := EmptyValidValidation(virtualServiceName, virtualService.Namespace, VirtualCheckerType)

	enabledCheckers := []Checker{
		virtualservices.RouteChecker{VirtualService: virtualService, Namespace: in.Namespace, Namespaces: in.Namespaces.GetNames()},
		virtualservices.SubsetPresenceChecker{Namespace: in.Namespace, Namespaces: in.Namespaces.GetNames(), DestinationRules: in.DestinationRules, VirtualService: virtualService, ExportedDestinationRules: in.ExportedDestinationRules},
		common.ExportToNamespaceChecker{ExportTo: virtualService.Spec.ExportTo, Namespaces: in.Namespaces},
	}

	for _, checker := range enabledCheckers {
		checks, validChecker := checker.Check()
		rrValidation.Checks = append(rrValidation.Checks, checks...)
		rrValidation.Valid = rrValidation.Valid && validChecker
	}

	return models.IstioValidations{key: rrValidation}
}
