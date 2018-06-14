package prometheus

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/kiali/kiali/config"
)

var (
	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

// MetricsQuery is a common struct for ServiceMetricsQuery and NamespaceMetricsQuery
type MetricsQuery struct {
	v1.Range
	Version      string
	RateInterval string
	RateFunc     string
	Filters      []string
	ByLabelsIn   []string
	ByLabelsOut  []string
	IncludeIstio bool
}

// FillDefaults fills the struct with default parameters
func (q *MetricsQuery) FillDefaults() {
	q.End = time.Now()
	q.Start = q.End.Add(-30 * time.Minute)
	q.Step = 15 * time.Second
	q.RateInterval = "1m"
	q.RateFunc = "rate"
	q.IncludeIstio = false
}

// ServiceMetricsQuery contains fields used for querying a service metrics
type ServiceMetricsQuery struct {
	MetricsQuery
	Namespace string
	Service   string
}

// NamespaceMetricsQuery contains fields used for querying namespace metrics
type NamespaceMetricsQuery struct {
	MetricsQuery
	Namespace      string
	ServicePattern string
}

// Metrics contains health, all simple metrics and histograms data
type Metrics struct {
	Metrics    map[string]*Metric   `json:"metrics"`
	Histograms map[string]Histogram `json:"histograms"`
}

// Metric holds the Prometheus Matrix model, which contains one or more time series (depending on grouping)
type Metric struct {
	Matrix model.Matrix `json:"matrix"`
	err    error
}

// Histogram contains Metric objects for several histogram-kind statistics
type Histogram struct {
	Average      *Metric `json:"average"`
	Median       *Metric `json:"median"`
	Percentile95 *Metric `json:"percentile95"`
	Percentile99 *Metric `json:"percentile99"`
}

// EnvoyHealth is the number of healthy versus total membership (ie. replicas) inside envoy cluster (ie. service)
type EnvoyHealth struct {
	Inbound  EnvoyRatio `json:"inbound"`
	Outbound EnvoyRatio `json:"outbound"`
}

// EnvoyRatio is the number of healthy members versus total members
type EnvoyRatio struct {
	Healthy int `json:"healthy"`
	Total   int `json:"total"`
}

func getServiceHealth(api v1.API, namespace string, servicename string) (EnvoyHealth, error) {
	envoyClustername := strings.Replace(config.Get().ExternalServices.Istio.IstioIdentityDomain, ".", "_", -1)
	queryPart := replaceInvalidCharacters(fmt.Sprintf("%s_%s_%s", servicename, namespace, envoyClustername))
	now := time.Now()
	ret := EnvoyHealth{}

	// Note: metric names below probably depend on some istio configuration.
	// They should anyway change soon in a more prometheus-friendly way,
	// see https://github.com/istio/istio/issues/4854 and https://github.com/istio/istio/pull/5069

	var healthyErrIn, totalErrIn, healthyErrOut, totalErrOut error
	var wg sync.WaitGroup
	wg.Add(4)
	// Inbound
	go func() {
		defer wg.Done()
		vec, err := fetchTimestamp(api, fmt.Sprintf("envoy_cluster_inbound_9080__%s_membership_healthy", queryPart), now)
		healthyErrIn = err
		if len(vec) > 0 {
			ret.Inbound.Healthy = int(vec[0].Value)
		}
	}()
	go func() {
		defer wg.Done()
		vec, err := fetchTimestamp(api, fmt.Sprintf("envoy_cluster_inbound_9080__%s_membership_total", queryPart), now)
		totalErrIn = err
		if len(vec) > 0 {
			ret.Inbound.Total = int(vec[0].Value)
		}
	}()
	go func() {
		defer wg.Done()
		vec, err := fetchTimestamp(api, fmt.Sprintf("envoy_cluster_outbound_9080__%s_membership_healthy", queryPart), now)
		healthyErrOut = err
		if len(vec) > 0 {
			ret.Outbound.Healthy = int(vec[0].Value)
		}
	}()
	go func() {
		defer wg.Done()
		vec, err := fetchTimestamp(api, fmt.Sprintf("envoy_cluster_outbound_9080__%s_membership_total", queryPart), now)
		totalErrOut = err
		if len(vec) > 0 {
			ret.Outbound.Total = int(vec[0].Value)
		}
	}()
	wg.Wait()
	if healthyErrIn != nil {
		return ret, healthyErrIn
	} else if totalErrIn != nil {
		return ret, totalErrIn
	} else if healthyErrOut != nil {
		return ret, healthyErrOut
	} else if totalErrOut != nil {
		return ret, totalErrOut
	}
	return ret, nil
}

func getServiceMetrics(api v1.API, q *ServiceMetricsQuery) Metrics {
	clustername := config.Get().ExternalServices.Istio.IstioIdentityDomain
	destService := fmt.Sprintf("destination_service=\"%s.%s.%s\"", q.Service, q.Namespace, clustername)
	srcService := fmt.Sprintf("source_service=\"%s.%s.%s\"", q.Service, q.Namespace, clustername)
	labelsIn, labelsOut, labelsErrorIn, labelsErrorOut := buildLabelStrings(destService, srcService, q.Version, q.IncludeIstio)
	groupingIn := joinLabels(q.ByLabelsIn)
	groupingOut := joinLabels(q.ByLabelsOut)

	return fetchAllMetrics(api, &q.MetricsQuery, labelsIn, labelsOut, labelsErrorIn, labelsErrorOut, groupingIn, groupingOut)
}

func getNamespaceMetrics(api v1.API, q *NamespaceMetricsQuery) Metrics {
	svc := q.ServicePattern
	if "" == svc {
		svc = ".*"
	}
	destService := fmt.Sprintf("destination_service=~\"%s\\\\.%s\\\\..*\"", svc, q.Namespace)
	srcService := fmt.Sprintf("source_service=~\"%s\\\\.%s\\\\..*\"", svc, q.Namespace)
	labelsIn, labelsOut, labelsErrorIn, labelsErrorOut := buildLabelStrings(destService, srcService, q.Version, q.IncludeIstio)
	groupingIn := joinLabels(q.ByLabelsIn)
	groupingOut := joinLabels(q.ByLabelsOut)

	return fetchAllMetrics(api, &q.MetricsQuery, labelsIn, labelsOut, labelsErrorIn, labelsErrorOut, groupingIn, groupingOut)
}

func buildLabelStrings(destServiceLabel, srcServiceLabel, version string, includeIstio bool) (string, string, string, string) {
	versionLabelIn := ""
	versionLabelOut := ""
	if len(version) > 0 {
		versionLabelIn = fmt.Sprintf(",destination_version=\"%s\"", version)
		versionLabelOut = fmt.Sprintf(",source_version=\"%s\"", version)
	}

	// when filtering we still keep incoming istio traffic, it's typically ingressgateway. We
	// only want to filter outgoing traffic to the istio infra services.
	istioFilterOut := ""
	if !includeIstio {
		istioFilterOut = ",destination_service!~\".*\\\\.istio-system\\\\..*\""
	}
	labelsIn := fmt.Sprintf("{%s%s}", destServiceLabel, versionLabelIn)
	labelsOut := fmt.Sprintf("{%s%s%s}", srcServiceLabel, versionLabelOut, istioFilterOut)
	labelsErrorIn := fmt.Sprintf("{%s%s,response_code=~\"[5|4].*\"}", destServiceLabel, versionLabelIn)
	labelsErrorOut := fmt.Sprintf("{%s%s%s,response_code=~\"[5|4].*\"}", srcServiceLabel, versionLabelOut, istioFilterOut)

	return labelsIn, labelsOut, labelsErrorIn, labelsErrorOut
}

func joinLabels(labels []string) string {
	str := ""
	if len(labels) > 0 {
		sep := ""
		for _, lbl := range labels {
			str = str + sep + lbl
			sep = ","
		}
	}
	return str
}

func fetchAllMetrics(api v1.API, q *MetricsQuery, labelsIn, labelsOut, labelsErrorIn, labelsErrorOut, groupingIn, groupingOut string) Metrics {
	var wg sync.WaitGroup
	fetchRateInOut := func(p8sFamilyName string, metricIn **Metric, metricOut **Metric, lblIn string, lblOut string) {
		defer wg.Done()
		m := fetchRateRange(api, p8sFamilyName, lblIn, groupingIn, q)
		*metricIn = m
		m = fetchRateRange(api, p8sFamilyName, lblOut, groupingOut, q)
		*metricOut = m
	}

	fetchHistoInOut := func(p8sFamilyName string, hIn *Histogram, hOut *Histogram) {
		defer wg.Done()
		h := fetchHistogramRange(api, p8sFamilyName, labelsIn, groupingIn, q)
		*hIn = h
		h = fetchHistogramRange(api, p8sFamilyName, labelsOut, groupingOut, q)
		*hOut = h
	}

	type resultHolder struct {
		metricIn   *Metric
		metricOut  *Metric
		histoIn    Histogram
		histoOut   Histogram
		definition kialiMetric
	}
	maxResults := len(kialiMetrics)
	results := make([]*resultHolder, maxResults, maxResults)

	for i, metric := range kialiMetrics {
		// if filters is empty, fetch all anyway
		doFetch := len(q.Filters) == 0
		if !doFetch {
			for _, filter := range q.Filters {
				if filter == metric.name {
					doFetch = true
					break
				}
			}
		}
		if doFetch {
			wg.Add(1)
			result := resultHolder{definition: metric}
			results[i] = &result
			if metric.isHisto {
				go fetchHistoInOut(metric.istioName, &result.histoIn, &result.histoOut)
			} else {
				labelsInToUse, labelsOutToUse := metric.labelsToUse(labelsIn, labelsOut, labelsErrorIn, labelsErrorOut)
				go fetchRateInOut(metric.istioName, &result.metricIn, &result.metricOut, labelsInToUse, labelsOutToUse)
			}
		}
	}
	wg.Wait()

	// Return results as two maps
	metrics := make(map[string]*Metric)
	histograms := make(map[string]Histogram)
	for _, result := range results {
		if result != nil {
			if result.definition.isHisto {
				histograms[result.definition.name+"_in"] = result.histoIn
				histograms[result.definition.name+"_out"] = result.histoOut
			} else {
				metrics[result.definition.name+"_in"] = result.metricIn
				metrics[result.definition.name+"_out"] = result.metricOut
			}
		}
	}
	return Metrics{
		Metrics:    metrics,
		Histograms: histograms}
}

func fetchRateRange(api v1.API, metricName string, labels string, grouping string, q *MetricsQuery) *Metric {
	var query string
	// Example: round(sum(rate(my_counter{foo=bar}[5m])) by (baz), 0.001)
	if grouping == "" {
		query = fmt.Sprintf("round(sum(%s(%s%s[%s])), 0.001)", q.RateFunc, metricName, labels, q.RateInterval)
	} else {
		query = fmt.Sprintf("round(sum(%s(%s%s[%s])) by (%s), 0.001)", q.RateFunc, metricName, labels, q.RateInterval, grouping)
	}
	return fetchRange(api, query, q.Range)
}

func fetchHistogramRange(api v1.API, metricName string, labels string, grouping string, q *MetricsQuery) Histogram {
	// Note: we may want to make returned stats configurable in the future
	// Note 2: the p8s queries are not run in parallel here, but they are at the caller's place.
	//	This is because we may not want to create too many threads in the lowest layer
	groupingAvg := ""
	groupingQuantile := ""
	if grouping != "" {
		groupingAvg = fmt.Sprintf(" by (%s)", grouping)
		groupingQuantile = fmt.Sprintf(",%s", grouping)
	}

	// Average
	// Example: sum(rate(my_histogram_sum{foo=bar}[5m])) by (baz) / sum(rate(my_histogram_count{foo=bar}[5m])) by (baz)
	query := fmt.Sprintf(
		"round(sum(rate(%s_sum%s[%s]))%s / sum(rate(%s_count%s[%s]))%s, 0.001)", metricName, labels, q.RateInterval, groupingAvg,
		metricName, labels, q.RateInterval, groupingAvg)
	avg := fetchRange(api, query, q.Range)

	// Median
	// Example: round(histogram_quantile(0.5, sum(rate(my_histogram_bucket{foo=bar}[5m])) by (le,baz)), 0.001)
	query = fmt.Sprintf(
		"round(histogram_quantile(0.5, sum(rate(%s_bucket%s[%s])) by (le%s)), 0.001)", metricName, labels, q.RateInterval, groupingQuantile)
	med := fetchRange(api, query, q.Range)

	// Quantile 95
	query = fmt.Sprintf(
		"round(histogram_quantile(0.95, sum(rate(%s_bucket%s[%s])) by (le%s)), 0.001)", metricName, labels, q.RateInterval, groupingQuantile)
	p95 := fetchRange(api, query, q.Range)

	// Quantile 99
	query = fmt.Sprintf(
		"round(histogram_quantile(0.99, sum(rate(%s_bucket%s[%s])) by (le%s)), 0.001)", metricName, labels, q.RateInterval, groupingQuantile)
	p99 := fetchRange(api, query, q.Range)

	return Histogram{
		Average:      avg,
		Median:       med,
		Percentile95: p95,
		Percentile99: p99}
}

func fetchTimestamp(api v1.API, query string, t time.Time) (model.Vector, error) {
	result, err := api.Query(context.Background(), query, t)
	if err != nil {
		return nil, err
	}
	switch result.Type() {
	case model.ValVector:
		return result.(model.Vector), nil
	}
	return nil, fmt.Errorf("Invalid query, vector expected: %s", query)
}

func fetchRange(api v1.API, query string, bounds v1.Range) *Metric {
	result, err := api.QueryRange(context.Background(), query, bounds)
	if err != nil {
		return &Metric{err: err}
	}
	switch result.Type() {
	case model.ValMatrix:
		return &Metric{Matrix: result.(model.Matrix)}
	}
	return &Metric{err: fmt.Errorf("Invalid query, matrix expected: %s", query)}
}

func replaceInvalidCharacters(metricName string) string {
	// See https://github.com/prometheus/prometheus/blob/master/util/strutil/strconv.go#L43
	return invalidLabelCharRE.ReplaceAllString(metricName, "_")
}

func getNamespaceServicesRequestRates(api v1.API, namespace string, ratesInterval string) (model.Vector, model.Vector, error) {
	lblIn := fmt.Sprintf(`destination_service=~".*\\.%s\\..*"`, namespace)
	in, err := getRequestRatesForLabel(api, time.Now(), lblIn, ratesInterval)
	if err != nil {
		return model.Vector{}, model.Vector{}, err
	}
	lblOut := fmt.Sprintf(`source_service=~".*\\.%s\\..*"`, namespace)
	out, err := getRequestRatesForLabel(api, time.Now(), lblOut, ratesInterval)
	if err != nil {
		return model.Vector{}, model.Vector{}, err
	}
	return in, out, nil
}

func getServiceRequestRates(api v1.API, namespace, service string, ratesInterval string) (model.Vector, model.Vector, error) {
	lblIn := fmt.Sprintf(`destination_service=~"%s\\.%s\\..*"`, service, namespace)
	in, err := getRequestRatesForLabel(api, time.Now(), lblIn, ratesInterval)
	if err != nil {
		return model.Vector{}, model.Vector{}, err
	}
	lblOut := fmt.Sprintf(`source_service=~"%s\\.%s\\..*"`, service, namespace)
	out, err := getRequestRatesForLabel(api, time.Now(), lblOut, ratesInterval)
	if err != nil {
		return model.Vector{}, model.Vector{}, err
	}
	return in, out, nil
}

func getRequestRatesForLabel(api v1.API, time time.Time, labels, ratesInterval string) (model.Vector, error) {
	query := fmt.Sprintf("rate(istio_request_count{%s}[%s])", labels, ratesInterval)
	result, err := api.Query(context.Background(), query, time)
	if err != nil {
		return model.Vector{}, err
	}
	return result.(model.Vector), nil
}
