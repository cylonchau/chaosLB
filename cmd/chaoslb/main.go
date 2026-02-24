package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cylonchau/chaosLB/pkg/config"
	"github.com/cylonchau/chaosLB/pkg/manager"
	"github.com/cylonchau/chaosLB/pkg/metrics"
	"github.com/cylonchau/chaosLB/pkg/utils"

	"github.com/spf13/cobra"
)

var (
	version    = "dev"
	configFile string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "chaoslb",
		Short: "Chaos Load Balancer - IPVS Manager with Prometheus Metrics",
		Run:   run,
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "config file path")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of chaoslb",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("chaoslb version %s\n", version)
		},
	}

	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	conf, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := utils.CheckPortAvailable(conf.MetricsPort); err != nil {
		log.Fatalf("Metrics port %d occupied: %v", conf.MetricsPort, err)
	}

	m := metrics.NewMetrics()
	mgr := manager.NewIPVSManager(conf, m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Setup(); err != nil {
		log.Fatalf("Setup failed: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down...")
		mgr.Cleanup()
		cancel()
	}()

	log.Printf("Chaos Load Balancer %s started", version)
	go mgr.Monitor(ctx)
	metrics.StartServer(ctx, conf.MetricsPort)
}
