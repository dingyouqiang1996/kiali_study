package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/services/business"
)

// AppList is the API handler to fetch all the apps to be displayed, related to a single namespace
func AppList(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	// Get business layer
	business, err := business.Get()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Apps initialization error: "+err.Error())
		return
	}
	namespace := params["namespace"]

	// Fetch and build apps
	appList, err := business.App.GetAppList(namespace)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, appList)
}

// AppDetails is the API handler to fetch all details to be displayed, related to a single app
func AppDetails(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	// Get business layer
	business, err := business.Get()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Services initialization error: "+err.Error())
		return
	}
	namespace := params["namespace"]
	app := params["app"]

	// Fetch and build app
	appDetails, err := business.App.GetApp(namespace, app)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, appDetails)
}

// AppMetrics is the API handler to fetch metrics to be displayed, related to an app-label grouping
func AppMetrics(w http.ResponseWriter, r *http.Request) {
	getAppMetrics(w, r, prometheus.NewClient)
}

// getAppMetrics (mock-friendly version)
func getAppMetrics(w http.ResponseWriter, r *http.Request, promClientSupplier func() (*prometheus.Client, error)) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	app := vars["app"]

	params := prometheus.MetricsQuery{Namespace: namespace, App: app}
	err := extractMetricsQueryParams(r, &params)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	prometheusClient, err := promClientSupplier()
	if err != nil {
		log.Error(err)
		RespondWithError(w, http.StatusServiceUnavailable, "Prometheus client error: "+err.Error())
		return
	}
	metrics := prometheusClient.GetMetrics(&params)
	RespondWithJSON(w, http.StatusOK, metrics)
}
