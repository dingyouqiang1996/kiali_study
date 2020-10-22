package business

import (
	"fmt"
	"sort"

	kbus "github.com/kiali/k-charted/business"
	kconf "github.com/kiali/k-charted/config"
	kxconf "github.com/kiali/k-charted/config/extconfig"
	klog "github.com/kiali/k-charted/log"
	kmodel "github.com/kiali/k-charted/model"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/prometheus/internalmetrics"
	"github.com/kiali/kiali/status"
)

// DashboardsService deals with fetching dashboards from k8s client
type DashboardsService struct {
	delegate *kbus.DashboardsService
	prom     prometheus.ClientInterface
}

// NewDashboardsService initializes this business service
func NewDashboardsService(prom prometheus.ClientInterface) DashboardsService {
	cfg, lg, enabled := DashboardsConfig()
	if !enabled {
		return DashboardsService{delegate: nil, prom: prom}
	}
	delegate := kbus.NewDashboardsService(cfg, lg)
	return DashboardsService{delegate: &delegate, prom: prom}
}

func DashboardsConfig() (kconf.Config, klog.LogAdapter, bool) {
	cfg := config.Get()
	if !cfg.ExternalServices.CustomDashboards.Enabled {
		return kconf.Config{}, klog.LogAdapter{}, false
	}
	pURL := cfg.ExternalServices.Prometheus.URL
	pauth := cfg.ExternalServices.Prometheus.Auth
	if cfg.ExternalServices.CustomDashboards.Prometheus.URL != "" {
		pURL = cfg.ExternalServices.CustomDashboards.Prometheus.URL
		pauth = cfg.ExternalServices.CustomDashboards.Prometheus.Auth
	}
	gauth := cfg.ExternalServices.Grafana.Auth
	if pauth.UseKialiToken || (cfg.ExternalServices.Grafana.Enabled && gauth.UseKialiToken) {
		kialiToken, err := kubernetes.GetKialiToken()
		if err != nil {
			log.Errorf("Could not read the Kiali Service Account token: %v", err)
		}
		if pauth.UseKialiToken {
			pauth.Token = kialiToken
		}
		if gauth.UseKialiToken {
			gauth.Token = kialiToken
		}
	}
	var grafanaConfig kxconf.GrafanaConfig
	if cfg.ExternalServices.Grafana.Enabled {
		grafanaConfig = kxconf.GrafanaConfig{
			URL:          status.DiscoverGrafana(),
			InClusterURL: cfg.ExternalServices.Grafana.InClusterURL,
			Auth: kxconf.Auth{
				Type:               gauth.Type,
				Username:           gauth.Username,
				Password:           gauth.Password,
				Token:              gauth.Token,
				InsecureSkipVerify: gauth.InsecureSkipVerify,
				CAFile:             gauth.CAFile,
			},
		}
	}
	nsLabel := cfg.ExternalServices.CustomDashboards.NamespaceLabel
	if nsLabel == "" {
		nsLabel = "kubernetes_namespace"
	}
	return kconf.Config{
			GlobalNamespace: cfg.Deployment.Namespace,
			Prometheus: kxconf.PrometheusConfig{
				URL: pURL,
				Auth: kxconf.Auth{
					Type:               pauth.Type,
					Username:           pauth.Username,
					Password:           pauth.Password,
					Token:              pauth.Token,
					InsecureSkipVerify: pauth.InsecureSkipVerify,
					CAFile:             pauth.CAFile,
				},
			},
			Grafana:        grafanaConfig,
			NamespaceLabel: nsLabel,
		}, klog.LogAdapter{
			Errorf:   log.Errorf,
			Warningf: log.Warningf,
			Infof:    log.Infof,
			Tracef:   log.Tracef,
		}, true
}

type istioChart struct {
	kmodel.Chart
	refName string
	scale   float64
}

func getIstioCharts() []istioChart {
	istioCharts := []istioChart{
		{
			Chart: kmodel.Chart{
				Name:  "Request volume",
				Unit:  "ops",
				Spans: 6,
			},
			refName: "request_count",
		},
		{
			Chart: kmodel.Chart{
				Name:  "Request duration",
				Unit:  "seconds",
				Spans: 6,
			},
			refName: "request_duration_millis",
			scale:   0.001,
		},
		{
			Chart: kmodel.Chart{
				Name:  "Request throughput",
				Unit:  "bitrate",
				Spans: 6,
			},
			refName: "request_throughput",
			scale:   8, // Bps to bps
		},
		{
			Chart: kmodel.Chart{
				Name:  "Request size",
				Unit:  "bytes",
				Spans: 6,
			},
			refName: "request_size",
		},
		{
			Chart: kmodel.Chart{
				Name:  "Response throughput",
				Unit:  "bitrate",
				Spans: 6,
			},
			refName: "response_throughput",
			scale:   8, // Bps to bps
		},
		{
			Chart: kmodel.Chart{
				Name:  "Response size",
				Unit:  "bytes",
				Spans: 6,
			},
			refName: "response_size",
		},
		{
			Chart: kmodel.Chart{
				Name:  "TCP received",
				Unit:  "bitrate",
				Spans: 6,
			},
			refName: "tcp_received",
		},
		{
			Chart: kmodel.Chart{
				Name:  "TCP sent",
				Unit:  "bitrate",
				Spans: 6,
			},
			refName: "tcp_sent",
		},
	}
	return istioCharts
}

// GetIstioDashboard returns Istio dashboard (currently hard-coded) filled-in with metrics
func (in *DashboardsService) GetIstioDashboard(params prometheus.IstioMetricsQuery) (*kmodel.MonitoringDashboard, error) {
	var dashboard kmodel.MonitoringDashboard
	// Copy dashboard
	if params.Direction == "inbound" {
		dashboard = models.PrepareIstioDashboard("Inbound", "destination", "source")
	} else {
		dashboard = models.PrepareIstioDashboard("Outbound", "source", "destination")
	}

	metrics := in.prom.GetMetrics(&params)
	istioCharts := getIstioCharts()

	for _, chartTpl := range istioCharts {
		newChart := chartTpl.Chart
		unitScale := 1.0
		if chartTpl.scale != 0.0 {
			unitScale = chartTpl.scale
		}
		if metric, ok := metrics.Metrics[chartTpl.refName]; ok {
			fillMetric(&newChart, metric, unitScale)
		}
		if histo, ok := metrics.Histograms[chartTpl.refName]; ok {
			fillHistogram(&newChart, histo, unitScale)
		}
		dashboard.Charts = append(dashboard.Charts, newChart)
	}
	return &dashboard, nil
}

func fillHistogram(chart *kmodel.Chart, from prometheus.Histogram, scale float64) {
	chart.Metrics = []*kmodel.SampleStream{}
	// Extract and sort keys for consistent ordering
	stats := []string{}
	for k := range from {
		stats = append(stats, k)
	}
	sort.Strings(stats)
	for _, stat := range stats {
		promMetric := from[stat]
		if promMetric.Err != nil {
			chart.Error = fmt.Sprintf("error in metric %s/%s: %v", chart.Name, stat, promMetric.Err)
			return
		}
		metric := kmodel.ConvertMatrix(promMetric.Matrix, kmodel.BuildLabelsMap(chart.Name, stat), kmodel.ConversionParams{Scale: scale})
		chart.Metrics = append(chart.Metrics, metric...)
	}
}

func fillMetric(chart *kmodel.Chart, from *prometheus.Metric, scale float64) {
	if from.Err != nil {
		chart.Metrics = []*kmodel.SampleStream{}
		chart.Error = fmt.Sprintf("error in metric %s: %v", chart.Name, from.Err)
		return
	}
	chart.Metrics = kmodel.ConvertMatrix(from.Matrix, kmodel.BuildLabelsMap(chart.Name, ""), kmodel.ConversionParams{Scale: scale})
}

// GetCustomDashboardRefs finds all dashboard IDs and Titles associated to this app and add them to the model
func (in *DashboardsService) GetCustomDashboardRefs(namespace, app, version string, pods []*models.Pod) []kmodel.Runtime {
	if in.delegate == nil {
		// Custom dashboards are disabled
		return []kmodel.Runtime{}
	}

	var err error
	promtimer := internalmetrics.GetGoFunctionMetric("business", "DashboardsService", "GetCustomDashboardRefs")
	defer promtimer.ObserveNow(&err)

	// A better way to do?
	var podsCast []kmodel.Pod
	for _, p := range pods {
		podsCast = append(podsCast, p)
	}
	runtimes := in.delegate.SearchExplicitDashboards(namespace, podsCast)

	if len(runtimes) == 0 {
		cfg := config.Get()
		filters := make(map[string]string)
		if app != "" {
			filters[cfg.IstioLabels.AppLabelName] = app
		}
		if version != "" {
			filters[cfg.IstioLabels.VersionLabelName] = version
		}
		runtimes = in.delegate.DiscoverDashboards(namespace, filters)
	}
	return runtimes
}
