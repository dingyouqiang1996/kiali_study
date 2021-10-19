package checkers

import (
	networking_v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	"github.com/kiali/kiali/business/checkers/common"
	"github.com/kiali/kiali/models"
)

const ServiceEntryCheckerType = "serviceentry"

type ServiceEntryChecker struct {
	ServiceEntries         []networking_v1alpha3.ServiceEntry
	ExportedServiceEntries []networking_v1alpha3.ServiceEntry
	Namespaces             models.Namespaces
}

func (s ServiceEntryChecker) Check() models.IstioValidations {
	validations := models.IstioValidations{}

	for _, se := range s.ServiceEntries {
		validations.MergeValidations(s.runSingleChecks(se))
	}

	return validations
}

func (s ServiceEntryChecker) runSingleChecks(se networking_v1alpha3.ServiceEntry) models.IstioValidations {
	key, validations := EmptyValidValidation(se.Name, se.Namespace, ServiceEntryCheckerType)

	enabledCheckers := []Checker{
		common.ExportToNamespaceChecker{ExportTo: se.Spec.ExportTo, Namespaces: s.Namespaces},
	}

	for _, checker := range enabledCheckers {
		checks, validChecker := checker.Check()
		validations.Checks = append(validations.Checks, checks...)
		validations.Valid = validations.Valid && validChecker
	}

	return models.IstioValidations{key: validations}
}
