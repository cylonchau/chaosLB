package metrics

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	ActiveConnections   *prometheus.GaugeVec
	InactiveConnections *prometheus.GaugeVec
	BackendWeight       *prometheus.GaugeVec
	BackendStatus       *prometheus.GaugeVec
	VipStatus           *prometheus.GaugeVec
	SnatRulesCount      *prometheus.GaugeVec
	SysctlStatus        *prometheus.GaugeVec
	BackendConnections  *prometheus.GaugeVec
	BackendInPackets    *prometheus.GaugeVec
	BackendOutPackets   *prometheus.GaugeVec
	BackendInBytes      *prometheus.GaugeVec
	BackendOutBytes     *prometheus.GaugeVec
	ConnectionsTotal    *prometheus.GaugeVec
	PacketsInTotal      *prometheus.GaugeVec
	PacketsOutTotal     *prometheus.GaugeVec
	BytesInTotal        *prometheus.GaugeVec
	BytesOutTotal       *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		ActiveConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_active_connections", Help: "Active IPVS connections"},
			[]string{"vip", "backend", "business"},
		),
		InactiveConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_inactive_connections", Help: "Inactive IPVS connections"},
			[]string{"vip", "backend", "business"},
		),
		// ... (Add all other metrics here similarly)
		BackendWeight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_backend_weight", Help: "Backend weight in IPVS"},
			[]string{"vip", "backend", "business"},
		),
		BackendStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_backend_status", Help: "Backend status (1=up, 0=down)"},
			[]string{"vip", "backend", "business"},
		),
		VipStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_vip_status", Help: "VIP status (1=active, 0=inactive)"},
			[]string{"vip", "interface", "business"},
		),
		SnatRulesCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_snat_rules_count", Help: "Number of SNAT rules created"},
			[]string{"business"},
		),
		SysctlStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_sysctl_status", Help: "Sysctl configuration status"},
			[]string{"parameter"},
		),
		BackendConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_backend_connections_total", Help: "Total connections to backend"},
			[]string{"vip", "backend", "business"},
		),
		BackendInPackets: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_backend_in_packets_total", Help: "Total incoming packets to backend"},
			[]string{"vip", "backend", "business"},
		),
		BackendOutPackets: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_backend_out_packets_total", Help: "Total outgoing packets from backend"},
			[]string{"vip", "backend", "business"},
		),
		BackendInBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_backend_in_bytes_total", Help: "Total incoming bytes to backend"},
			[]string{"vip", "backend", "business"},
		),
		BackendOutBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_backend_out_bytes_total", Help: "Total outgoing bytes from backend"},
			[]string{"vip", "backend", "business"},
		),
		ConnectionsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_connections_total", Help: "Total connections to virtual service"},
			[]string{"service", "business"},
		),
		PacketsInTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_packets_in_total", Help: "Total incoming packets to virtual service"},
			[]string{"service", "business"},
		),
		PacketsOutTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_packets_out_total", Help: "Total outgoing packets from virtual service"},
			[]string{"service", "business"},
		),
		BytesInTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_bytes_in_total", Help: "Total incoming bytes to virtual service"},
			[]string{"service", "business"},
		),
		BytesOutTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "ipvs_bytes_out_total", Help: "Total outgoing bytes from virtual service"},
			[]string{"service", "business"},
		),
	}

	prometheus.MustRegister(m.ActiveConnections)
	prometheus.MustRegister(m.InactiveConnections)
	prometheus.MustRegister(m.BackendWeight)
	prometheus.MustRegister(m.BackendStatus)
	prometheus.MustRegister(m.VipStatus)
	prometheus.MustRegister(m.SnatRulesCount)
	prometheus.MustRegister(m.SysctlStatus)
	prometheus.MustRegister(m.BackendConnections)
	prometheus.MustRegister(m.BackendInPackets)
	prometheus.MustRegister(m.BackendOutPackets)
	prometheus.MustRegister(m.BackendInBytes)
	prometheus.MustRegister(m.BackendOutBytes)
	prometheus.MustRegister(m.ConnectionsTotal)
	prometheus.MustRegister(m.PacketsInTotal)
	prometheus.MustRegister(m.PacketsOutTotal)
	prometheus.MustRegister(m.BytesInTotal)
	prometheus.MustRegister(m.BytesOutTotal)

	return m
}

func StartServer(ctx context.Context, port int) {
	http.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Printf("Metrics server starting on port %d", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Metrics server error: %v", err)
	}
}
