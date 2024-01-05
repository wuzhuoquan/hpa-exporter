package main

import (

	"flag"
	"fmt"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/rest"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/tools/clientcmd"
	"hpa-exportor/pkg/metrics"
	"hpa-exportor/pkg/kube"
)

func main() {

	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")

	flag.Parse()

	var config *rest.Config
	//var err error
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println("error")
	}
	//if strings.Compare(*kubeconfig, "") == 0 {
	//	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	//	if err != nil {
	//		fmt.Printf("Error building kubeconfig: %v\n", err)
	//		os.Exit(1)
	//	}
	//} else {
	//	if config, err = rest.InClusterConfig(); err != nil {
	//		if config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig); err != nil {
	//			panic(any(err))
	//		}
	//	}
	//}
	metricsStore := metrics.NewMetricsStore("kube_")
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

