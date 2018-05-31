package route_rules

import (
	"reflect"
	"strconv"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/services/models"
	"github.com/kiali/kiali/util/intutil"
)

type RouteChecker struct{ kubernetes.IstioObject }

// Check returns both an array of IstioCheck and a boolean indicating if the current route rule is valid.
// The array of IstioChecks contains the result of running the following validations:
// 1. All weights with a numeric number.
// 2. All weights have value between 0 and 100.
// 3. Sum of all weights are 100 (if only one weight, then it assumes that is 100).
// 4. All the route has to have weight label.
func (route RouteChecker) Check() ([]*models.IstioCheck, bool) {
	validations := make([]*models.IstioCheck, 0)

	var weightSum int
	var weightCount int
	var valid bool = true

	slice := reflect.ValueOf(route.GetSpec()["route"])
	if slice.Kind() != reflect.Slice {
		return validations, valid
	}

	for i := 0; i < slice.Len(); i++ {
		var weight int

		route := slice.Index(i).Interface().(map[string]interface{})
		if route["weight"] == nil {
			continue
		}

		weightCount = weightCount + 1
		weight, err := intutil.Convert(route["weight"])
		if err != nil {
			valid = false
			validation := models.BuildCheck("Weight must be a number",
				"error", "spec/route["+strconv.Itoa(i)+"]/weight/"+route["weight"].(string))
			validations = append(validations, &validation)
		}

		if weight > 100 || weight < 0 {
			valid = false
			validation := models.BuildCheck("Weight should be between 0 and 100",
				"error", "spec/route["+strconv.Itoa(i)+"]/weight/"+strconv.Itoa(weight))
			validations = append(validations, &validation)
		}

		weightSum = weightSum + weight
	}

	if weightCount > 0 && weightSum != 100 {
		valid = false
		validation := models.BuildCheck("Weight sum should be 100", "error", "")
		validations = append(validations, &validation)
	}

	if weightCount > 0 && weightCount != slice.Len() {
		valid = false
		validation := models.BuildCheck("All routes should have weight", "error", "")
		validations = append(validations, &validation)
	}

	return validations, valid
}
