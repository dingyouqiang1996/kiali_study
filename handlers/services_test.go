package handlers

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/prometheus/prometheustest"
)

// TestServiceMetricsDefault is unit test (testing request handling, not the prometheus client behaviour)
func TestServiceMetricsDefault(t *testing.T) {
	ts, api := setupServiceMetricsEndpoint(t)
	defer ts.Close()

	url := ts.URL + "/api/namespaces/ns/services/svc/metrics"
	now := time.Now()
	delta := 2 * time.Second
	coveredPath := 0

	api.SpyArgumentsAndReturnEmpty(func(args mock.Arguments) {
		query := args[1].(string)
		assert.IsType(t, v1.Range{}, args[2])
		r := args[2].(v1.Range)
		assert.Contains(t, query, "svc.ns.svc.cluster.local")
		assert.Contains(t, query, "[1m]")
		if strings.Contains(query, "histogram_quantile") {
			// Histogram specific queries
			assert.Contains(t, query, " by (le)")
			coveredPath |= 1
		} else {
			assert.NotContains(t, query, " by ")
			coveredPath |= 2
		}
		assert.Equal(t, 15*time.Second, r.Step)
		assert.WithinDuration(t, now, r.End, delta)
		assert.WithinDuration(t, now.Add(-30*time.Minute), r.Start, delta)
	})

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)

	assert.NotEmpty(t, actual)
	assert.Equal(t, 200, resp.StatusCode, string(actual))
	// Assert branch coverage
	assert.Equal(t, coveredPath, 3)
}

func TestServiceMetricsWithParams(t *testing.T) {
	ts, api := setupServiceMetricsEndpoint(t)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/api/namespaces/ns/services/svc/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("rateInterval", "5h")
	q.Add("step", "99")
	q.Add("duration", "1000")
	q.Add("byLabelsIn[]", "response_code")
	q.Add("byLabelsOut[]", "response_code")
	q.Add("filters[]", "request_count")
	q.Add("filters[]", "request_size")
	req.URL.RawQuery = q.Encode()

	now := time.Now()
	delta := 2 * time.Second
	coveredPath := 0

	api.SpyArgumentsAndReturnEmpty(func(args mock.Arguments) {
		query := args[1].(string)
		assert.IsType(t, v1.Range{}, args[2])
		r := args[2].(v1.Range)
		assert.Contains(t, query, "[5h]")
		if strings.Contains(query, "histogram_quantile") {
			// Histogram specific queries
			assert.Contains(t, query, " by (le,response_code)")
			assert.Contains(t, query, "istio_request_size")
			coveredPath |= 1
		} else {
			assert.Contains(t, query, " by (response_code)")
			coveredPath |= 2
		}
		assert.Equal(t, 99*time.Second, r.Step)
		assert.WithinDuration(t, now, r.End, delta)
		assert.WithinDuration(t, now.Add(-1000*time.Second), r.Start, delta)
	})

	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)

	assert.NotEmpty(t, actual)
	assert.Equal(t, 200, resp.StatusCode, string(actual))
	// Assert branch coverage
	assert.Equal(t, coveredPath, 3)
}

func TestServiceMetricsBadDuration(t *testing.T) {
	ts, api := setupServiceMetricsEndpoint(t)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/api/namespaces/ns/services/svc/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("rateInterval", "5h")
	q.Add("step", "99")
	q.Add("duration", "abc")
	req.URL.RawQuery = q.Encode()

	api.SpyArgumentsAndReturnEmpty(func(args mock.Arguments) {
		// Make sure there's no client call and we fail fast
		t.Error("Unexpected call to client while having bad request")
	})

	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, 400, resp.StatusCode)
	assert.Contains(t, string(actual), "cannot parse query parameter 'duration'")
}

func TestServiceMetricsBadStep(t *testing.T) {
	ts, api := setupServiceMetricsEndpoint(t)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/api/namespaces/ns/services/svc/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("rateInterval", "5h")
	q.Add("step", "abc")
	q.Add("duration", "1000")
	req.URL.RawQuery = q.Encode()

	api.SpyArgumentsAndReturnEmpty(func(args mock.Arguments) {
		// Make sure there's no client call and we fail fast
		t.Error("Unexpected call to client while having bad request")
	})

	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, 400, resp.StatusCode)
	assert.Contains(t, string(actual), "cannot parse query parameter 'step'")
}

func setupServiceMetricsEndpoint(t *testing.T) (*httptest.Server, *prometheustest.PromAPIMock) {
	client, api, err := setupMocked()
	if err != nil {
		t.Fatal(err)
	}

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/{namespace}/services/{service}/metrics", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			getServiceMetrics(w, r, func() (*prometheus.Client, error) {
				return client, nil
			})
		}))

	ts := httptest.NewServer(mr)
	return ts, api
}

// TestServiceHealth is unit test (testing request handling, not the prometheus client behaviour)
func TestServiceHealth(t *testing.T) {
	ts, api := setupServiceHealthEndpoint(t)
	defer ts.Close()

	url := ts.URL + "/api/namespaces/ns/services/svc/health"
	now := time.Now()
	delta := 2 * time.Second

	api.SpyArgumentsAndReturnEmpty(func(args mock.Arguments) {
		query := args[1].(string)
		assert.IsType(t, time.Time{}, args[2])
		timestamp := args[2].(time.Time)
		// Health = envoy metrics
		assert.Contains(t, query, "svc_ns_svc_cluster_local")
		assert.WithinDuration(t, now, timestamp, delta)
	})

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)

	assert.NotEmpty(t, actual)
	assert.Equal(t, 200, resp.StatusCode, string(actual))
}

func setupServiceHealthEndpoint(t *testing.T) (*httptest.Server, *prometheustest.PromAPIMock) {
	client, api, err := setupMocked()
	if err != nil {
		t.Fatal(err)
	}

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/{namespace}/services/{service}/health", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			getServiceHealth(w, r, func() (*prometheus.Client, error) {
				return client, nil
			})
		}))

	ts := httptest.NewServer(mr)
	return ts, api
}
