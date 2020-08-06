package handlers

import (
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus"
)

func NamespaceList(w http.ResponseWriter, r *http.Request) {
	business, err := getBusiness(r)

	if err != nil {
		log.Error(err)
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	namespaces, err := business.Namespace.GetNamespaces()
	if err != nil {
		log.Error(err)
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, namespaces)
}

// NamespaceMetrics is the API handler to fetch metrics to be displayed, related to all
// services in the namespace
func NamespaceMetrics(w http.ResponseWriter, r *http.Request) {
	getNamespaceMetrics(w, r, defaultPromClientSupplier)
}

// getServiceMetrics (mock-friendly version)
func getNamespaceMetrics(w http.ResponseWriter, r *http.Request, promSupplier promClientSupplier) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	prom, namespaceInfo := initClientsForMetrics(w, r, promSupplier, namespace)
	if prom == nil {
		// any returned value nil means error & response already written
		return
	}

	params := prometheus.IstioMetricsQuery{Namespace: namespace}
	err := extractIstioMetricsQueryParams(r, &params, namespaceInfo)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	metrics := prom.GetMetrics(&params)
	RespondWithJSON(w, http.StatusOK, metrics)
}

// NamespaceValidationSummary is the API handler to fetch validations summary to be displayed.
// It is related to all the Istio Objects within the namespace
func NamespaceValidationSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	business, err := getBusiness(r)
	if err != nil {
		log.Error(err)
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var validationSummary models.IstioValidationSummary

	istioConfigValidationResults, errValidations := business.Validations.GetValidations(namespace, "")
	if errValidations != nil {
		log.Error(errValidations)
		RespondWithError(w, http.StatusInternalServerError, errValidations.Error())
	} else {
		validationSummary = istioConfigValidationResults.SummarizeValidation(namespace)
	}

	RespondWithJSON(w, http.StatusOK, validationSummary)
}

// NamespaceUpdate is the API to perform a patch on a Namespace configuration
func NamespaceUpdate(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	business, err := getBusiness(r)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Namespace initialization error: "+err.Error())
		return
	}
	namespace := params["namespace"]
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Update request with bad update patch: "+err.Error())
	}
	jsonPatch := string(body)

	ns, err := business.Namespace.UpdateNamespace(namespace, jsonPatch)
	if err != nil {
		handleErrorResponse(w, err)
		return
	}
	audit(r, "UPDATE on Namespace: "+namespace+" Patch: "+jsonPatch)
	RespondWithJSON(w, http.StatusOK, ns)
}
