package prometheus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/kiali/kiali/config"
)

// ClientInterface for mocks (only mocked function are necessary here)
type ClientInterface interface {
	GetServiceHealth(namespace, servicename string, ports []int32) (EnvoyHealth, error)
	GetNamespaceServicesRequestRates(namespace, ratesInterval string) (model.Vector, model.Vector, error)
	GetServiceRequestRates(namespace, service, ratesInterval string) (model.Vector, model.Vector, error)
	GetSourceServices(namespace, servicename string) (map[string][]string, error)
}

// Client for Prometheus API.
// It hides the way we query Prometheus offering a layer with a high level defined API.
type Client struct {
	ClientInterface
	p8s api.Client
	api v1.API
}

// NewClient creates a new client to the Prometheus API.
// It returns an error on any problem.
func NewClient() (*Client, error) {
	if config.Get() == nil {
		return nil, errors.New("config.Get() must be not null")
	}
	p8s, err := api.NewClient(api.Config{Address: config.Get().ExternalServices.PrometheusServiceURL})
	if err != nil {
		return nil, err
	}
	client := Client{p8s: p8s, api: v1.NewAPI(p8s)}
	return &client, nil
}

// Inject allows for replacing the API with a mock For testing
func (in *Client) Inject(api v1.API) {
	in.api = api
}

// GetSourceServices returns a map of list of source services for a given service identified by its namespace and service name.
// Returned map has a destination version as a key and a list of "<origin service>/<origin version>" pairs as values.
// Destination service is not included in the map as it is passed as argument.
// It returns an error on any problem.
func (in *Client) GetSourceServices(namespace string, servicename string) (map[string][]string, error) {
	query := fmt.Sprintf("istio_request_count{destination_service=\"%s.%s.%s\"}",
		servicename, namespace, config.Get().ExternalServices.Istio.IstioIdentityDomain)
	result, err := in.api.Query(context.Background(), query, time.Now())
	if err != nil {
		return nil, err
	}
	routes := make(map[string][]string)
	switch result.Type() {
	case model.ValVector:
		matrix := result.(model.Vector)
		for _, sample := range matrix {
			metric := sample.Metric
			index := fmt.Sprintf("%s", metric["destination_version"])
			sourceService := string(metric["source_service"])
			// sourceService is in the form "service.namespace.istio_identity_domain". We want to keep only "service.namespace".
			if i := strings.Index(sourceService, "."+config.Get().ExternalServices.Istio.IstioIdentityDomain); i > 0 {
				sourceService = sourceService[:i]
			}
			source := fmt.Sprintf("%s/%s", sourceService, metric["source_version"])
			if arr, ok := routes[index]; ok {
				found := false
				for _, s := range arr {
					if s == source {
						found = true
						break
					}
				}
				if !found {
					routes[index] = append(arr, source)
				}
			} else {
				routes[index] = []string{source}
			}
		}
	}
	return routes, nil
}

// GetServiceMetrics returns the Metrics related to the provided service identified by its namespace and service name.
func (in *Client) GetServiceMetrics(query *ServiceMetricsQuery) Metrics {
	return getServiceMetrics(in.api, query)
}

// GetServiceHealth returns the Health related to the provided service identified by its namespace and service name.
// It reads Envoy metrics, inbound and outbound
// When the health is unavailable, total number of members will be 0.
func (in *Client) GetServiceHealth(namespace, servicename string, ports []int32) (EnvoyHealth, error) {
	return getServiceHealth(in.api, namespace, servicename, ports)
}

// GetNamespaceMetrics returns the Metrics described by the optional service pattern ("" for all), and optional
// version, for the given namespace. Use GetServiceMetrics if you don't need pattern matching.
func (in *Client) GetNamespaceMetrics(query *NamespaceMetricsQuery) Metrics {
	return getNamespaceMetrics(in.api, query)
}

// GetNamespaceServicesRequestRates queries Prometheus to fetch request counters rates over a time interval
// for each service, both in and out.
// Returns (in, out, error)
func (in *Client) GetNamespaceServicesRequestRates(namespace string, ratesInterval string) (model.Vector, model.Vector, error) {
	return getNamespaceServicesRequestRates(in.api, namespace, ratesInterval)
}

// GetServiceRequestRates queries Prometheus to fetch request counters rates over a time interval
// for a given service, both in and out.
// Returns (in, out, error)
func (in *Client) GetServiceRequestRates(namespace, service string, ratesInterval string) (model.Vector, model.Vector, error) {
	return getServiceRequestRates(in.api, namespace, service, ratesInterval)
}

// API returns the Prometheus V1 HTTP API for performing calls not supported natively by this client
func (in *Client) API() v1.API {
	return in.api
}

// Address return the configured Prometheus service URL
func (in *Client) Address() string {
	return config.Get().ExternalServices.PrometheusServiceURL
}
