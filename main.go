package main

import (
	"flag"
	"fmt"
	"fs-access-api/internal/adapters/out/metrics"
	"fs-access-api/internal/app"
	"fs-access-api/internal/app/config"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var ProgramVersion = "dev"

const (
	ProgramName = "fs-access-api"
)

func main() {
	configFileFlag := flag.String("config", "config.yml", "Path to configuration YAML")
	pidFileFlag := flag.String("pidfile", "", "Path to PID file (optional)")
	bootstrapFlag := flag.Bool("bootstrap", false, "If the instance is the first instance of its group")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFileFlag)
	if err != nil {
		panic(fmt.Errorf("cannot load --config=%s: %v", *configFileFlag, err))
	}

	var pidCleanup func()
	if *pidFileFlag != "" {
		pidCleanup, err = app.CreatePIDFile(*pidFileFlag)
		if err != nil {
			log.Fatalf("pidfile: %v", err)
		}
		defer pidCleanup()
	}

	cfg.PrintHello(ProgramName, ProgramVersion, *pidFileFlag, *bootstrapFlag)

	reg := prometheus.NewRegistry()

	// add standard Go/process collectors (they are NOT in reg by default)
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	actionMetrics, err := metrics.NewAuthzActionMetrics(ProgramName, ProgramVersion, cfg.Metrics, reg)
	if err != nil {
		panic(err)
	}

	restServer, err := app.BuildRestServer(cfg, *bootstrapFlag, actionMetrics)
	if err != nil {
		panic(fmt.Errorf("cannot build rest server: %v", err))
	}

	router := app.BuildRouter(restServer)

	// Wrap router to expose /metrics alongside all existing routes.
	mux := http.NewServeMux()
	mux.Handle(cfg.HttpServer.TelemetryPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	// / is the root of the API
	mux.Handle("/", router)

	servers, err := app.NewMultiHTTPServer(cfg.HttpServer, mux)
	if err != nil {
		panic(err)
	}
	servers.Start()
	servers.WaitAndShutdown()
}
