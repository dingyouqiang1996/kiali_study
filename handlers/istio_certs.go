package handlers

import "net/http"

// IstioCerts returns information about internal mTLS certificates used by Istio
func IstioCerts(w http.ResponseWriter, r *http.Request) {
	business, err := getBusiness(r)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Services initialization error: "+err.Error())
		return
	}
	certs, _ := business.IstioCerts.GetCertsInfo()
	RespondWithJSON(w, http.StatusOK, certs)
}
