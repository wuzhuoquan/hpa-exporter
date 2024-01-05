package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/rs/zerolog/log"
)


type Store struct {
	HpaStatusCurrentMetrics   *prometheus.GaugeVec
	HpaStatusCurrentReplicas  *prometheus.GaugeVec
	HpaSpecTargetsMetrics     *prometheus.GaugeVec
	HpaStatusDesiredReplicas  *prometheus.GaugeVec
	HpaSpecMaxReplicas  *prometheus.GaugeVec
	HpaSpecMinReplicas  *prometheus.GaugeVec
}

func NewMetricsStore(namePrefix string) *Store {
	return &Store{
		HpaStatusCurrentMetrics: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: namePrefix + "hpa_status_current_metrics",
			Help: "hpa_status_current_metrics",
		}, []string{"hpa_name", "namespace", "targetRef", "targetRefKind", "metric", "type"}),
		HpaSpecTargetsMetrics: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: namePrefix + "hpa_spec_targets_metrics",
			Help: "hpa_spec_targets_metrics",
		}, []string{"hpa_name", "namespace", "targetRef", "targetRefKind", "metric", "type"}),
		HpaStatusCurrentReplicas: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: namePrefix + "hpa_status_current_replicas",
			Help: "hpa_status_current_replicas",
		}, []string{"hpa_name", "namespace", "targetRef", "targetRefKind"}),
		HpaStatusDesiredReplicas: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: namePrefix + "hpa_status_desired_replicas",
			Help: "hpa_status_desired_replicas",
		}, []string{"hpa_name", "namespace", "targetRef", "targetRefKind"}),
		HpaSpecMaxReplicas: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: namePrefix + "hpa_spec_max_replicas",
			Help: "hpa_spec_max_replicas",
		}, []string{"hpa_name", "namespace", "targetRef", "targetRefKind"}),
		HpaSpecMinReplicas: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: namePrefix + "hpa_spec_min_replicas",
			Help: "hpa_spec_min_replicas",
		}, []string{"hpa_name", "namespace", "targetRef", "targetRefKind"}),
	}
}

// promLogger implements promhttp.Logger
type promLogger struct{}

func (pl promLogger) Println(v ...interface{}) {
	log.Logger.Error().Msg(fmt.Sprint(v...))
}

// promLogger implements the Logger interface
func (pl promLogger) Log(v ...interface{}) error {
	log.Logger.Info().Msg(fmt.Sprint(v...))
	return nil
}

func Init(addr string, tlsConf string) {
	// Setup the prometheus metrics machinery
	// Add Go module build info.
	prometheus.MustRegister(collectors.NewBuildInfoCollector())

	promLogger := promLogger{}
	metricsPath := "/metrics"

	// Expose the registered metrics via HTTP.
	http.Handle(metricsPath, promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))

	landingConfig := web.LandingConfig{
		Name:        "kubernetes-event-exporter",
		Description: "Export Kubernetes Events to multiple destinations with routing and filtering",
		Links: []web.LandingLinks{
			{
				Address: metricsPath,
				Text:    "Metrics",
			},
		},
	}
	landingPage, _ := web.NewLandingPage(landingConfig)
	http.Handle("/", landingPage)

	http.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})
	http.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	metricsServer := http.Server{
		ReadHeaderTimeout: 5 * time.Second}

	metricsFlags := web.FlagConfig{
		WebListenAddresses: &[]string{addr},
		WebSystemdSocket:   new(bool),
		WebConfigFile:      &tlsConf,
	}

	// start up the http listener to expose the metrics
	go web.ListenAndServe(&metricsServer, &metricsFlags, promLogger)
}
