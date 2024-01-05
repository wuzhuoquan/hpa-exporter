- flag:
```
-kubeconfig: specify the kubeconfig,if not set,will use in cluster mode
-prefix: the prefix of metrics
```

- RUN
```
go build
./hpa-exporter -kubeconfig=xxxxx -prefix="kube_"
```

- Get the metrics
```
curl http://localhost:8080/metrics
```

- metrics
```
hpa_status_current_metrics
hpa_spec_targets_metrics
hpa_status_current_replicas
hpa_status_desired_replicas
hpa_spec_max_replicas
hpa_spec_min_replicas
```