package main

import (
	"flag"
	"fmt"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/rest"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"hpa-exportor/pkg/kube"
	"hpa-exportor/pkg/metrics"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	metricPrefix := flag.String("prefix", "kube_", "metrics prefix, default kube_")

	flag.Parse()

	var config *rest.Config
	var err error
	//config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	//if err != nil {
	//	fmt.Println("error")
	//}
	if strings.Compare(*kubeconfig, "") != 0 {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			fmt.Printf("Error building kubeconfig: %v\n", err)
			os.Exit(1)
		}
	} else {
		if config, err = rest.InClusterConfig(); err != nil {
			fmt.Printf("Error building kubeclient: %v\n", err)
			os.Exit(1)
		}
	}

	metricsStore := metrics.NewMetricsStore(*metricPrefix)
	metrics.Init(":8080", "")
	hpawatcher := kube.NewHpaWatcher(config, metricsStore)

	hpawatcher.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)



	select {
	case sig := <-c:
		log.Info().Str("signal", sig.String()).Msg("Received signal to exit")
		hpawatcher.Stop()
	}
}

