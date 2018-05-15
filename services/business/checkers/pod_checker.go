package checkers

import (
	"github.com/kiali/kiali/services/business/checkers/pods"
	"github.com/kiali/kiali/services/models"
	"k8s.io/api/core/v1"
)

type PodChecker struct {
	Pods []v1.Pod
}

const podsCheckerType = "pod"

func (checker PodChecker) Check() *models.IstioValidations {
	// The only available checker test for missing
	// sidecars in a service. Only individual checks makes sense for now.
	return checker.runIndividualChecks()
}

func (checker PodChecker) runIndividualChecks() *models.IstioValidations {
	validations := models.IstioValidations{}
	if len(checker.Pods) == 0 {
		return &validations
	}

	for _, pod := range checker.Pods {
		validation := models.IstioValidation{
			Name:       pod.ObjectMeta.Name,
			ObjectType: podsCheckerType,
			Valid:      true,
		}
		validations[pod.ObjectMeta.Name] = &validation

		checkers := checker.enabledCheckersFor(&pod)

		for _, podChecker := range checkers {
			checks, isValid := podChecker.Check()
			validation.Checks = append(validation.Checks, checks...)
			validation.Valid = validation.Valid && isValid
		}
	}

	return &validations
}

func (checker *PodChecker) enabledCheckersFor(object *v1.Pod) []Checker {
	return []Checker{
		pods.SidecarPresenceChecker{Pod: object},
	}
}
