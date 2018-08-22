package prometheustest

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/prometheus"
)

func setupMocked() (*prometheus.Client, *PromAPIMock, error) {
	config.Set(config.NewConfig())
	api := new(PromAPIMock)
	client, err := prometheus.NewClient()
	if err != nil {
		return nil, nil, err
	}
	client.Inject(api)
	return client, api, nil
}

func TestGetSourceWorkloads(t *testing.T) {
	rqCustV1C200 := model.Metric{
		"__name__":                  "istio_requests_total",
		"instance":                  "172.17.0.6:42422",
		"job":                       "istio-mesh",
		"response_code":             "200",
		"source_workload_namespace": "istio-system",
		"source_app":                "customer",
		"source_workload":           "customer-v1",
		"source_version":            "v1",
		"destination_service":       "productpage.istio-system.svc.cluster.local",
		"destination_version":       "v1"}
	rqCustV1C404 := model.Metric{
		"__name__":                  "istio_requests_total",
		"instance":                  "172.17.0.6:42422",
		"job":                       "istio-mesh",
		"response_code":             "404",
		"source_workload_namespace": "istio-system",
		"source_app":                "customer",
		"source_workload":           "customer-v1",
		"source_version":            "v1",
		"destination_service":       "productpage.istio-system.svc.cluster.local",
		"destination_version":       "v1"}
	rqCustV2 := model.Metric{
		"__name__":                  "istio_requests_total",
		"instance":                  "172.17.0.6:42422",
		"job":                       "istio-mesh",
		"response_code":             "200",
		"source_workload_namespace": "istio-system",
		"source_app":                "customer",
		"source_workload":           "customer-v2",
		"source_version":            "v2",
		"destination_service":       "productpage.istio-system.svc.cluster.local",
		"destination_version":       "v1"}
	rqCustV2ToV2 := model.Metric{
		"__name__":                  "istio_requests_total",
		"instance":                  "172.17.0.6:42422",
		"job":                       "istio-mesh",
		"response_code":             "200",
		"source_workload_namespace": "istio-system",
		"source_app":                "customer",
		"source_workload":           "customer-v2",
		"source_version":            "v2",
		"destination_service":       "productpage.istio-system.svc.cluster.local",
		"destination_version":       "v2"}
	vector := model.Vector{
		&model.Sample{
			Metric: rqCustV1C200,
			Value:  4},
		&model.Sample{
			Metric: rqCustV1C404,
			Value:  4},
		&model.Sample{
			Metric: rqCustV2,
			Value:  1},
		&model.Sample{
			Metric: rqCustV2ToV2,
			Value:  2}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, "istio_requests_total{reporter=\"destination\",destination_service_name=\"productpage\",destination_service_namespace=\"istio-system\"}", &vector)
	sources, err := client.GetSourceWorkloads("istio-system", "productpage")
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, 2, len(sources), "Map should have 2 keys (versions)")
	assert.Equal(t, 2, len(sources["v1"]), "v1 should have 2 sources")
	assert.Equal(t, 1, len(sources["v2"]), "v2 should have 1 source")
	assert.Equal(t, "customer", sources["v1"][0].App)
	assert.Equal(t, "customer", sources["v1"][1].App)
	assert.Equal(t, "customer", sources["v2"][0].App)
	assert.Equal(t, "istio-system", sources["v1"][0].Namespace)
	assert.Equal(t, "istio-system", sources["v1"][1].Namespace)
	assert.Equal(t, "istio-system", sources["v2"][0].Namespace)
	assert.Equal(t, "customer-v1", sources["v1"][0].Workload)
	assert.Equal(t, "customer-v2", sources["v1"][1].Workload)
	assert.Equal(t, "customer-v2", sources["v2"][0].Workload)
	assert.Equal(t, "v1", sources["v1"][0].Version)
	assert.Equal(t, "v2", sources["v1"][1].Version)
	assert.Equal(t, "v2", sources["v2"][0].Version)
}

func TestGetServiceMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockRange(api, "round(sum(rate(istio_requests_total{destination_service_name=\"productpage\",destination_service_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 2.5)
	mockRange(api, "round(sum(rate(istio_requests_total{destination_service_name=\"productpage\",destination_service_namespace=\"bookinfo\",response_code=~\"[5|4].*\"}[5m])) by (reporter), 0.001)", 4.5)
	mockRange(api, "round(sum(rate(istio_tcp_received_bytes_total{destination_service_name=\"productpage\",destination_service_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 11)
	mockRange(api, "round(sum(rate(istio_tcp_sent_bytes_total{destination_service_name=\"productpage\",destination_service_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 13)
	mockHistogram(api, "istio_request_bytes", "{destination_service_name=\"productpage\",destination_service_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.7)
	mockHistogram(api, "istio_request_duration_seconds", "{destination_service_name=\"productpage\",destination_service_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.8)
	mockHistogram(api, "istio_response_bytes", "{destination_service_name=\"productpage\",destination_service_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.9)
	metrics := client.GetMetrics(&prometheus.MetricsQuery{
		Namespace: "bookinfo",
		Service:   "productpage",
		Range: v1.Range{
			Start: time.Unix(1000, 0),
			End:   time.Unix(2000, 0),
			Step:  10,
		},
		RateInterval: "5m",
		RateFunc:     "rate",
		ByLabelsIn:   []string{},
		ByLabelsOut:  []string{},
		Filters:      []string{}})

	assert.Equal(t, 4, len(metrics.Dest.Metrics), "Should have 4 simple metrics")
	assert.Equal(t, 3, len(metrics.Dest.Histograms), "Should have 3 histograms")
	rqCountIn := metrics.Dest.Metrics["request_count_in"]
	assert.NotNil(t, rqCountIn)
	rqErrorCountIn := metrics.Dest.Metrics["request_error_count_in"]
	assert.NotNil(t, rqCountIn)
	rqSizeIn := metrics.Dest.Histograms["request_size_in"]
	assert.NotNil(t, rqSizeIn)
	rqDurationIn := metrics.Dest.Histograms["request_duration_in"]
	assert.NotNil(t, rqDurationIn)
	rsSizeIn := metrics.Dest.Histograms["response_size_in"]
	assert.NotNil(t, rsSizeIn)
	tcpRecIn := metrics.Dest.Metrics["tcp_received_in"]
	assert.NotNil(t, tcpRecIn)
	tcpSentIn := metrics.Dest.Metrics["tcp_sent_in"]
	assert.NotNil(t, tcpSentIn)

	assert.Equal(t, 2.5, float64(rqCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 4.5, float64(rqErrorCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.7, float64(rqSizeIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.8, float64(rqDurationIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.9, float64(rsSizeIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 11.0, float64(tcpRecIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 13.0, float64(tcpSentIn.Matrix[0].Values[0].Value))
}

func TestGetAppMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockRange(api, "round(sum(rate(istio_requests_total{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 1.5)
	mockRange(api, "round(sum(rate(istio_requests_total{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 2.5)
	mockRange(api, "round(sum(rate(istio_requests_total{source_app=\"productpage\",source_workload_namespace=\"bookinfo\",response_code=~\"[5|4].*\"}[5m])) by (reporter), 0.001)", 3.5)
	mockRange(api, "round(sum(rate(istio_requests_total{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\",response_code=~\"[5|4].*\"}[5m])) by (reporter), 0.001)", 4.5)
	mockRange(api, "round(sum(rate(istio_tcp_received_bytes_total{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 10)
	mockRange(api, "round(sum(rate(istio_tcp_received_bytes_total{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 11)
	mockRange(api, "round(sum(rate(istio_tcp_sent_bytes_total{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 12)
	mockRange(api, "round(sum(rate(istio_tcp_sent_bytes_total{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 13)
	mockHistogram(api, "istio_request_bytes", "{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.4)
	mockHistogram(api, "istio_request_duration_seconds", "{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.5)
	mockHistogram(api, "istio_response_bytes", "{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.6)
	mockHistogram(api, "istio_request_bytes", "{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.7)
	mockHistogram(api, "istio_request_duration_seconds", "{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.8)
	mockHistogram(api, "istio_response_bytes", "{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.9)
	metrics := client.GetMetrics(&prometheus.MetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
		Range: v1.Range{
			Start: time.Unix(1000, 0),
			End:   time.Unix(2000, 0),
			Step:  10,
		},
		RateInterval: "5m",
		RateFunc:     "rate",
		ByLabelsIn:   []string{},
		ByLabelsOut:  []string{},
		Filters:      []string{}})

	assert.Equal(t, 8, len(metrics.Dest.Metrics), "Should have 8 simple metrics")
	assert.Equal(t, 6, len(metrics.Dest.Histograms), "Should have 6 histograms")
	rqCountIn := metrics.Dest.Metrics["request_count_in"]
	assert.NotNil(t, rqCountIn)
	rqCountOut := metrics.Dest.Metrics["request_count_out"]
	assert.NotNil(t, rqCountOut)
	rqErrorCountIn := metrics.Dest.Metrics["request_error_count_in"]
	assert.NotNil(t, rqCountIn)
	rqErrorCountOut := metrics.Dest.Metrics["request_error_count_out"]
	assert.NotNil(t, rqCountOut)
	rqSizeIn := metrics.Dest.Histograms["request_size_in"]
	assert.NotNil(t, rqSizeIn)
	rqSizeOut := metrics.Dest.Histograms["request_size_out"]
	assert.NotNil(t, rqSizeOut)
	rqDurationIn := metrics.Dest.Histograms["request_duration_in"]
	assert.NotNil(t, rqDurationIn)
	rqDurationOut := metrics.Dest.Histograms["request_duration_out"]
	assert.NotNil(t, rqDurationOut)
	rsSizeIn := metrics.Dest.Histograms["response_size_in"]
	assert.NotNil(t, rsSizeIn)
	rsSizeOut := metrics.Dest.Histograms["response_size_out"]
	assert.NotNil(t, rsSizeOut)
	tcpRecIn := metrics.Dest.Metrics["tcp_received_in"]
	assert.NotNil(t, tcpRecIn)
	tcpRecOut := metrics.Dest.Metrics["tcp_received_out"]
	assert.NotNil(t, tcpRecOut)
	tcpSentIn := metrics.Dest.Metrics["tcp_sent_in"]
	assert.NotNil(t, tcpSentIn)
	tcpSentOut := metrics.Dest.Metrics["tcp_sent_out"]
	assert.NotNil(t, tcpSentOut)

	assert.Equal(t, 2.5, float64(rqCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 1.5, float64(rqCountOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 4.5, float64(rqErrorCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 3.5, float64(rqErrorCountOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.35, float64(rqSizeOut.Average.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.2, float64(rqSizeOut.Median.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.3, float64(rqSizeOut.Percentile95.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.4, float64(rqSizeOut.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.5, float64(rqDurationOut.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.6, float64(rsSizeOut.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.7, float64(rqSizeIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.8, float64(rqDurationIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.9, float64(rsSizeIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 11.0, float64(tcpRecIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 10.0, float64(tcpRecOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 13.0, float64(tcpSentIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 12.0, float64(tcpSentOut.Matrix[0].Values[0].Value))
}

func TestGetFilteredAppMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockRange(api, "round(sum(rate(istio_requests_total{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 1.5)
	mockRange(api, "round(sum(rate(istio_requests_total{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 2.5)
	mockHistogram(api, "istio_request_bytes", "{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.4)
	mockHistogram(api, "istio_request_bytes", "{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.7)
	metrics := client.GetMetrics(&prometheus.MetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
		Range: v1.Range{
			Start: time.Unix(1000, 0),
			End:   time.Unix(2000, 0),
			Step:  10,
		},
		RateInterval: "5m",
		RateFunc:     "rate",
		ByLabelsIn:   []string{},
		ByLabelsOut:  []string{},
		Filters:      []string{"request_count", "request_size"}})

	assert.Equal(t, 2, len(metrics.Dest.Metrics), "Should have 2 simple metrics")
	assert.Equal(t, 2, len(metrics.Dest.Histograms), "Should have 2 histograms")
	rqCountIn := metrics.Dest.Metrics["request_count_in"]
	assert.NotNil(t, rqCountIn)
	rqCountOut := metrics.Dest.Metrics["request_count_out"]
	assert.NotNil(t, rqCountOut)
	rqSizeIn := metrics.Dest.Histograms["request_size_in"]
	assert.NotNil(t, rqSizeIn)
	rqSizeOut := metrics.Dest.Histograms["request_size_out"]
	assert.NotNil(t, rqSizeOut)
}

func TestGetAppMetricsInstantRates(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockRange(api, "round(sum(irate(istio_requests_total{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[1m])) by (reporter), 0.001)", 1.5)
	mockRange(api, "round(sum(irate(istio_requests_total{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[1m])) by (reporter), 0.001)", 2.5)
	metrics := client.GetMetrics(&prometheus.MetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
		Range: v1.Range{
			Start: time.Unix(1000, 0),
			End:   time.Unix(2000, 0),
			Step:  10,
		},
		RateInterval: "1m",
		RateFunc:     "irate",
		ByLabelsIn:   []string{},
		ByLabelsOut:  []string{},
		Filters:      []string{"request_count"},
	})

	assert.Equal(t, 2, len(metrics.Dest.Metrics), "Should have 2 simple metrics")
	assert.Equal(t, 0, len(metrics.Dest.Histograms), "Should have no histogram")
	rqCountIn := metrics.Dest.Metrics["request_count_in"]
	assert.NotNil(t, rqCountIn)
	rqCountOut := metrics.Dest.Metrics["request_count_out"]
	assert.NotNil(t, rqCountOut)
}

func TestGetServiceHealth(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockSingle(api, "envoy_cluster_inbound_9080__productpage_istio_system_svc_cluster_local_membership_healthy", 0)
	mockSingle(api, "envoy_cluster_inbound_9080__productpage_istio_system_svc_cluster_local_membership_total", 1)
	mockSingle(api, "envoy_cluster_outbound_9080__productpage_istio_system_svc_cluster_local_membership_healthy", 0)
	mockSingle(api, "envoy_cluster_outbound_9080__productpage_istio_system_svc_cluster_local_membership_total", 1)
	mockSingle(api, "envoy_cluster_inbound_8080__productpage_istio_system_svc_cluster_local_membership_healthy", 1)
	mockSingle(api, "envoy_cluster_inbound_8080__productpage_istio_system_svc_cluster_local_membership_total", 2)
	mockSingle(api, "envoy_cluster_outbound_8080__productpage_istio_system_svc_cluster_local_membership_healthy", 3)
	mockSingle(api, "envoy_cluster_outbound_8080__productpage_istio_system_svc_cluster_local_membership_total", 4)
	health, _ := client.GetServiceHealth("istio-system", "productpage", []int32{9080, 8080})

	// Check health
	assert.Equal(t, 1, health.Inbound.Healthy)
	assert.Equal(t, 3, health.Inbound.Total)
	assert.Equal(t, 3, health.Outbound.Healthy)
	assert.Equal(t, 5, health.Outbound.Total)
}

func TestGetAppMetricsUnavailable(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	// Mock everything to return empty data
	mockEmptyRange(api, "round(sum(rate(istio_requests_total{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)")
	mockEmptyRange(api, "round(sum(rate(istio_requests_total{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)")
	mockEmptyHistogram(api, "istio_request_bytes", "{source_app=\"productpage\",source_workload_namespace=\"bookinfo\"}[5m]")
	mockEmptyHistogram(api, "istio_request_bytes", "{destination_app=\"productpage\",destination_workload_namespace=\"bookinfo\"}[5m]")
	metrics := client.GetMetrics(&prometheus.MetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
		Range: v1.Range{
			Start: time.Unix(1000, 0),
			End:   time.Unix(2000, 0),
			Step:  10,
		},
		RateInterval: "5m",
		RateFunc:     "rate",
		ByLabelsIn:   []string{},
		ByLabelsOut:  []string{},
		Filters:      []string{"request_count", "request_size"},
	})

	assert.Equal(t, 2, len(metrics.Dest.Metrics), "Should have 2 simple metrics")
	assert.Equal(t, 2, len(metrics.Dest.Histograms), "Should have 2 histograms")
	rqCountIn := metrics.Dest.Metrics["request_count_in"]
	assert.NotNil(t, rqCountIn)
	rqSizeIn := metrics.Dest.Histograms["request_size_in"]
	assert.NotNil(t, rqSizeIn)

	// Simple metric & histogram are empty
	assert.Empty(t, rqCountIn.Matrix[0].Values)
	assert.Empty(t, rqSizeIn.Average.Matrix[0].Values)
	assert.Empty(t, rqSizeIn.Median.Matrix[0].Values)
	assert.Empty(t, rqSizeIn.Percentile95.Matrix[0].Values)
	assert.Empty(t, rqSizeIn.Percentile99.Matrix[0].Values)
}

func TestGetServiceHealthUnavailable(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	// Mock everything to return empty data
	mockQuery(api, "envoy_cluster_inbound_9080__productpage_istio_system_svc_cluster_local_membership_healthy", &model.Vector{})
	mockQuery(api, "envoy_cluster_inbound_9080__productpage_istio_system_svc_cluster_local_membership_total", &model.Vector{})
	mockQuery(api, "envoy_cluster_outbound_9080__productpage_istio_system_svc_cluster_local_membership_healthy", &model.Vector{})
	mockQuery(api, "envoy_cluster_outbound_9080__productpage_istio_system_svc_cluster_local_membership_total", &model.Vector{})
	h, err := client.GetServiceHealth("istio-system", "productpage", []int32{9080})

	// Check health unavailable
	assert.Nil(t, err)
	assert.Equal(t, 0, h.Inbound.Total)
	assert.Equal(t, 0, h.Outbound.Total)
}

func TestGetNamespaceMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockRange(api, "round(sum(rate(istio_requests_total{source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 1.5)
	mockRange(api, "round(sum(rate(istio_requests_total{destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 2.5)
	mockRange(api, "round(sum(rate(istio_requests_total{source_workload_namespace=\"bookinfo\",response_code=~\"[5|4].*\"}[5m])) by (reporter), 0.001)", 3.5)
	mockRange(api, "round(sum(rate(istio_requests_total{destination_workload_namespace=\"bookinfo\",response_code=~\"[5|4].*\"}[5m])) by (reporter), 0.001)", 4.5)
	mockRange(api, "round(sum(rate(istio_tcp_received_bytes_total{source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 10)
	mockRange(api, "round(sum(rate(istio_tcp_received_bytes_total{destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 11)
	mockRange(api, "round(sum(rate(istio_tcp_sent_bytes_total{source_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 12)
	mockRange(api, "round(sum(rate(istio_tcp_sent_bytes_total{destination_workload_namespace=\"bookinfo\"}[5m])) by (reporter), 0.001)", 13)
	mockHistogram(api, "istio_request_bytes", "{source_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.4)
	mockHistogram(api, "istio_request_duration_seconds", "{source_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.5)
	mockHistogram(api, "istio_response_bytes", "{source_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.6)
	mockHistogram(api, "istio_request_bytes", "{destination_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.7)
	mockHistogram(api, "istio_request_duration_seconds", "{destination_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.8)
	mockHistogram(api, "istio_response_bytes", "{destination_workload_namespace=\"bookinfo\"}[5m]", 0.35, 0.2, 0.3, 0.9)
	metrics := client.GetMetrics(&prometheus.MetricsQuery{
		Namespace: "bookinfo",
		Range: v1.Range{
			Start: time.Unix(1000, 0),
			End:   time.Unix(2000, 0),
			Step:  10,
		},
		RateInterval: "5m",
		RateFunc:     "rate",
		ByLabelsIn:   []string{},
		ByLabelsOut:  []string{},
		Filters:      []string{}})

	assert.Equal(t, 8, len(metrics.Dest.Metrics), "Should have 8 simple metrics")
	assert.Equal(t, 6, len(metrics.Dest.Histograms), "Should have 6 histograms")
	rqCountIn := metrics.Dest.Metrics["request_count_in"]
	assert.NotNil(t, rqCountIn)
	rqCountOut := metrics.Dest.Metrics["request_count_out"]
	assert.NotNil(t, rqCountOut)
	rqErrorCountIn := metrics.Dest.Metrics["request_error_count_in"]
	assert.NotNil(t, rqCountIn)
	rqErrorCountOut := metrics.Dest.Metrics["request_error_count_out"]
	assert.NotNil(t, rqCountOut)
	rqSizeIn := metrics.Dest.Histograms["request_size_in"]
	assert.NotNil(t, rqSizeIn)
	rqSizeOut := metrics.Dest.Histograms["request_size_out"]
	assert.NotNil(t, rqSizeOut)
	rqDurationIn := metrics.Dest.Histograms["request_duration_in"]
	assert.NotNil(t, rqDurationIn)
	rqDurationOut := metrics.Dest.Histograms["request_duration_out"]
	assert.NotNil(t, rqDurationOut)
	rsSizeIn := metrics.Dest.Histograms["response_size_in"]
	assert.NotNil(t, rsSizeIn)
	rsSizeOut := metrics.Dest.Histograms["response_size_out"]
	assert.NotNil(t, rsSizeOut)
	tcpRecIn := metrics.Dest.Metrics["tcp_received_in"]
	assert.NotNil(t, tcpRecIn)
	tcpRecOut := metrics.Dest.Metrics["tcp_received_out"]
	assert.NotNil(t, tcpRecOut)
	tcpSentIn := metrics.Dest.Metrics["tcp_sent_in"]
	assert.NotNil(t, tcpSentIn)
	tcpSentOut := metrics.Dest.Metrics["tcp_sent_out"]
	assert.NotNil(t, tcpSentOut)

	assert.Equal(t, 2.5, float64(rqCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 1.5, float64(rqCountOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 4.5, float64(rqErrorCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 3.5, float64(rqErrorCountOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.35, float64(rqSizeOut.Average.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.2, float64(rqSizeOut.Median.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.3, float64(rqSizeOut.Percentile95.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.4, float64(rqSizeOut.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.5, float64(rqDurationOut.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.6, float64(rsSizeOut.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.7, float64(rqSizeIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.8, float64(rqDurationIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.9, float64(rsSizeIn.Percentile99.Matrix[0].Values[0].Value))
	assert.Equal(t, 11.0, float64(tcpRecIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 10.0, float64(tcpRecOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 13.0, float64(tcpSentIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 12.0, float64(tcpSentOut.Matrix[0].Values[0].Value))
}

func TestGetAllRequestRates(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	vectorQ1 := model.Vector{
		&model.Sample{
			Timestamp: model.Now(),
			Value:     model.SampleValue(1),
			Metric:    model.Metric{"foo": "bar"},
		},
	}
	mockQuery(api, `rate(istio_requests_total{reporter="source",destination_service_namespace="ns",source_workload_namespace!="ns"}[5m])`, &vectorQ1)

	vectorQ2 := model.Vector{
		&model.Sample{
			Timestamp: model.Now(),
			Value:     model.SampleValue(2),
			Metric:    model.Metric{"foo": "bar"}},
	}
	mockQuery(api, `rate(istio_requests_total{reporter="source",source_workload_namespace="ns"}[5m])`, &vectorQ2)

	rates, err := client.GetAllRequestRates("ns", "5m")
	assert.Equal(t, 2, rates.Len())
	assert.Equal(t, vectorQ1[0], rates[0])
	assert.Equal(t, vectorQ2[0], rates[1])
}

func TestGetAllRequestRatesIstioSystem(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	vectorQ1 := model.Vector{
		&model.Sample{
			Timestamp: model.Now(),
			Value:     model.SampleValue(1),
			Metric:    model.Metric{"foo": "bar"},
		},
	}
	mockQuery(api, `rate(istio_requests_total{reporter="destination",destination_service_namespace="istio-system",source_workload_namespace!="istio-system"}[5m])`, &vectorQ1)

	vectorQ2 := model.Vector{
		&model.Sample{
			Timestamp: model.Now(),
			Value:     model.SampleValue(2),
			Metric:    model.Metric{"foo": "bar"}},
	}
	mockQuery(api, `rate(istio_requests_total{reporter="destination",source_workload_namespace="istio-system"}[5m])`, &vectorQ2)

	rates, err := client.GetAllRequestRates("istio-system", "5m")
	assert.Equal(t, 2, rates.Len())
	assert.Equal(t, vectorQ1[0], rates[0])
	assert.Equal(t, vectorQ2[0], rates[1])
}

func mockQuery(api *PromAPIMock, query string, ret *model.Vector) {
	api.On(
		"Query",
		mock.AnythingOfType("*context.emptyCtx"),
		query,
		mock.AnythingOfType("time.Time")).
		Return(*ret, nil)
}

func mockSingle(api *PromAPIMock, query string, ret model.SampleValue) {
	metric := model.Metric{
		"__name__": "whatever",
		"instance": "whatever",
		"job":      "whatever"}
	vector := model.Vector{
		&model.Sample{
			Metric: metric,
			Value:  ret}}
	mockQuery(api, query, &vector)
}

func mockQueryRange(api *PromAPIMock, query string, ret *model.Matrix) {
	api.On(
		"QueryRange",
		mock.AnythingOfType("*context.emptyCtx"),
		query,
		mock.AnythingOfType("v1.Range")).
		Return(*ret, nil)
}

func mockRange(api *PromAPIMock, query string, ret model.SampleValue) {
	metric := model.Metric{
		"reporter": "destination",
		"__name__": "whatever",
		"instance": "whatever",
		"job":      "whatever"}
	matrix := model.Matrix{
		&model.SampleStream{
			Metric: metric,
			Values: []model.SamplePair{model.SamplePair{Timestamp: 0, Value: ret}}}}
	mockQueryRange(api, query, &matrix)
}

func mockEmptyRange(api *PromAPIMock, query string) {
	metric := model.Metric{
		"reporter": "destination",
		"__name__": "whatever",
		"instance": "whatever",
		"job":      "whatever"}
	matrix := model.Matrix{
		&model.SampleStream{
			Metric: metric,
			Values: []model.SamplePair{}}}
	mockQueryRange(api, query, &matrix)
}

func mockHistogram(api *PromAPIMock, baseName string, suffix string, retAvg model.SampleValue, retMed model.SampleValue, ret95 model.SampleValue, ret99 model.SampleValue) {
	histMetric := "sum(rate(" + baseName + "_bucket" + suffix + ")) by (le,reporter)), 0.001)"
	mockRange(api, "round(histogram_quantile(0.5, "+histMetric, retMed)
	mockRange(api, "round(histogram_quantile(0.95, "+histMetric, ret95)
	mockRange(api, "round(histogram_quantile(0.99, "+histMetric, ret99)
	mockRange(api, "round(sum(rate("+baseName+"_sum"+suffix+")) by (reporter) / sum(rate("+baseName+"_count"+suffix+")) by (reporter), 0.001)", retAvg)
}

func mockEmptyHistogram(api *PromAPIMock, baseName string, suffix string) {
	histMetric := "sum(rate(" + baseName + "_bucket" + suffix + ")) by (le,reporter)), 0.001)"
	mockEmptyRange(api, "round(histogram_quantile(0.5, "+histMetric)
	mockEmptyRange(api, "round(histogram_quantile(0.95, "+histMetric)
	mockEmptyRange(api, "round(histogram_quantile(0.99, "+histMetric)
	mockEmptyRange(api, "round(sum(rate("+baseName+"_sum"+suffix+")) by (reporter) / sum(rate("+baseName+"_count"+suffix+")) by (reporter), 0.001)")
}

func setupExternal() (*prometheus.Client, error) {
	conf := config.NewConfig()
	conf.ExternalServices.PrometheusServiceURL = "http://prometheus-istio-system.127.0.0.1.nip.io"
	config.Set(conf)
	return prometheus.NewClient()
}
