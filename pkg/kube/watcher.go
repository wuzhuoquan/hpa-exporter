package kube

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"hpa-exportor/pkg/metrics"
	hpav1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"
	"strconv"

	//hpav2 "k8s.io/api/autoscaling/v2"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"os"
)

type HpaWatcher struct {
	informer           cache.SharedInformer
	stopper            chan struct{}
	metricsStore       *metrics.Store
}

type Action string

const  (
	DelHpa      Action = "del"
	UpdateHpa	Action = "update"
)


func NewHpaWatcher(config *rest.Config, metricsStore *metrics.Store) *HpaWatcher {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0)
	hpaInformer := factory.Autoscaling().V1().HorizontalPodAutoscalers().Informer()
	watcher := &HpaWatcher{
		informer:           hpaInformer,
		stopper:            make(chan struct{}),
		metricsStore:       metricsStore,
	}
	hpaInformer.AddEventHandler(watcher)
	return watcher
}


func (e *HpaWatcher) OnAdd(obj interface{}, isInInitialList bool) {
	hpaobj := obj.(*hpav1.HorizontalPodAutoscaler)
	e.processMetrics(hpaobj, UpdateHpa)
	//e.metricsStore.HpaStatusCurrentReplicas.WithLabelValues(hpa.Name).Set(float64(hpa.Status.CurrentReplicas))
}

func (e *HpaWatcher) OnUpdate(oldObj, newObj interface{}) {
	oldHpaObj := oldObj.(*hpav1.HorizontalPodAutoscaler)
	hpaobj := newObj.(*hpav1.HorizontalPodAutoscaler)
	e.processMetrics(oldHpaObj, DelHpa)
	e.processMetrics(hpaobj, UpdateHpa)

}

func (e *HpaWatcher) OnDelete(obj interface{}) {
	hpaobj := obj.(*hpav1.HorizontalPodAutoscaler)
	e.processMetrics(hpaobj, DelHpa)
}


func (e *HpaWatcher) processMetrics(hpaObject *hpav1.HorizontalPodAutoscaler, action Action)  {
	hpaName := hpaObject.GetName()
	hpaNamespace := hpaObject.GetNamespace()
	targetRefName := hpaObject.Spec.ScaleTargetRef.Name
	targetRefKind := hpaObject.Spec.ScaleTargetRef.Kind

	currentReplicas := hpaObject.Status.CurrentReplicas
	desireReplicas := hpaObject.Status.DesiredReplicas
	maxReplicas := hpaObject.Spec.MaxReplicas
	minReplicas := *hpaObject.Spec.MinReplicas
	switch action {
	case UpdateHpa:
		e.metricsStore.HpaStatusCurrentReplicas.WithLabelValues(hpaName, hpaNamespace, targetRefName, targetRefKind).Set(float64(currentReplicas))
		e.metricsStore.HpaStatusDesiredReplicas.WithLabelValues(hpaName, hpaNamespace, targetRefName, targetRefKind).Set(float64(desireReplicas))
		e.metricsStore.HpaSpecMaxReplicas.WithLabelValues(hpaName, hpaNamespace, targetRefName, targetRefKind).Set(float64(maxReplicas))
		e.metricsStore.HpaSpecMinReplicas.WithLabelValues(hpaName, hpaNamespace, targetRefName, targetRefKind).Set(float64(minReplicas))
	case DelHpa:
		delLabels := prometheus.Labels{"hpa_name": hpaName, "namespace": hpaNamespace}
		e.metricsStore.HpaStatusCurrentReplicas.DeletePartialMatch(delLabels)
		e.metricsStore.HpaStatusDesiredReplicas.DeletePartialMatch(delLabels)
		e.metricsStore.HpaSpecMaxReplicas.DeletePartialMatch(delLabels)
		e.metricsStore.HpaSpecMinReplicas.DeletePartialMatch(delLabels)
		e.metricsStore.HpaStatusCurrentMetrics.DeletePartialMatch(delLabels)
		e.metricsStore.HpaSpecTargetsMetrics.DeletePartialMatch(delLabels)
		return
	}


	annotations := hpaObject.Annotations
	currentMetrics := annotations["autoscaling.alpha.kubernetes.io/current-metrics"]
	var currentMetricsArray []hpav1.MetricStatus
	if err := json.Unmarshal([]byte(currentMetrics), &currentMetricsArray); err != nil {
		klog.V(4).ErrorS(err, "Can not transform annotation autoscaling.alpha.kubernetes.io/current-metrics: %v to []hpav1.MetricStatus for hpa %v in %v", currentMetrics, hpaName, hpaNamespace)
	}
	for _, metric := range currentMetricsArray {
		metricType := metric.Type
		var metricName string
		var metricValue int64
		switch metricType {
		case hpav1.ObjectMetricSourceType:
			metricName = metric.Object.MetricName
			metricValue = metric.Object.AverageValue.MilliValue()
		case hpav1.PodsMetricSourceType:
			metricName = metric.Pods.MetricName
			metricValue = metric.Pods.CurrentAverageValue.MilliValue()
		case hpav1.ExternalMetricSourceType:
			metricName = metric.External.MetricName
			metricValue = metric.External.CurrentAverageValue.MilliValue()
		default:
			klog.V(4).Info("metricType %v is not validated", metricType)
			continue
		}
		v, _ := strconv.ParseFloat(fmt.Sprintf("%.5f", float64(metricValue)/float64(1000)), 64)
		e.metricsStore.HpaStatusCurrentMetrics.WithLabelValues(hpaName, hpaNamespace, targetRefName, targetRefKind, metricName, string(metricType)).Set(v)
	}

	targetMetrics := annotations["autoscaling.alpha.kubernetes.io/metrics"]
	var targetMetricsArray []hpav1.MetricSpec
	if err := json.Unmarshal([]byte(targetMetrics), &targetMetricsArray); err != nil {
		klog.V(4).ErrorS(err, "Can not transform annotation autoscaling.alpha.kubernetes.io/conditions: %v to []hpav1.MetricSpec for hpa %v in %v", targetMetrics, hpaName, hpaNamespace)
	}
	for _, targetmetric := range targetMetricsArray {
		targetmetricType := targetmetric.Type
		var metricName string
		var targetValue int64
		switch targetmetricType {
		case hpav1.ObjectMetricSourceType:
			metricName = targetmetric.Object.MetricName
			targetValue = targetmetric.Object.TargetValue.MilliValue()
		case hpav1.PodsMetricSourceType:
			metricName = targetmetric.Pods.MetricName
			targetValue = targetmetric.Pods.TargetAverageValue.MilliValue()
		case hpav1.ExternalMetricSourceType:
			metricName = targetmetric.External.MetricName
			targetValue = targetmetric.External.TargetAverageValue.MilliValue()
		default:
			klog.V(4).Info("metricType %v is not validated", targetmetricType)
			continue
		}
		v, _ := strconv.ParseFloat(fmt.Sprintf("%.5f", float64(targetValue)/float64(1000)), 64)
		e.metricsStore.HpaSpecTargetsMetrics.WithLabelValues(hpaName, hpaNamespace, targetRefName, targetRefKind, metricName, string(targetmetricType)).Set(v)
	}
}


func (e *HpaWatcher) Start() {
	go e.informer.Run(e.stopper)
}

func (e *HpaWatcher) Stop() {
	e.stopper <- struct{}{}
	close(e.stopper)
}