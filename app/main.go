package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/hashicorp/consul/api"
)

const (
	name = "mytest"
	port = 9001
)

var (
	srvId  string
	client *api.Client
	server *http.Server
)

var (
	rpcDurations = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "rpc_durations_seconds",
			Help:       "RPC latency distributions.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"service"},
	)
)

func register() {
	var err error
	cfg := api.DefaultConfig()
	cfg.Address = "consul:8500"
	client, err = api.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	var address string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if addrIP, ok := addr.(*net.IPNet); ok {
				address = addrIP.IP.String()

			}
		}

	}
	if address == "" {
		panic("no iface")
	}

	endpoint := fmt.Sprintf("http://%s:%d/health", address, port)

	checkCfg := &api.AgentServiceCheck{
		Interval: "5s",
		HTTP:     endpoint,
		Timeout:  "10s",
		DeregisterCriticalServiceAfter: "1m",
	}
	srvId = fmt.Sprintf("%s-%d", name, time.Now().Unix())
	if err := client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:      srvId,
		Name:    name,
		Address: address,
		Port:    int(port),
		Check:   checkCfg,
	}); err != nil {
		panic(err)
	}
}

func startServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rpcDurations.WithLabelValues("test").Observe(rand.NormFloat64())
		io.WriteString(w, "hello world")
	})
	mux.Handle("/metrics", promhttp.Handler())
	server = &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
}

func grace() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	client.Agent().ServiceDeregister(srvId)
	server.Shutdown(context.Background())
}

func main() {
	register()
	startServer()
	grace()

}
