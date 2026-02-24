package manager

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cylonchau/chaosLB/pkg/config"
	"github.com/cylonchau/chaosLB/pkg/metrics"
	"github.com/cylonchau/chaosLB/pkg/utils"
)

type IPVSManager struct {
	config           *config.Config
	metrics          *metrics.Metrics
	createdRules     []string
	createdVIPs      []string
	createdIfaces    []string
	createdSNATRules []string
	originalSysctls  map[string]string
	mu               sync.RWMutex
}

func NewIPVSManager(conf *config.Config, m *metrics.Metrics) *IPVSManager {
	return &IPVSManager{
		config:          conf,
		metrics:         m,
		createdRules:    make([]string, 0),
		createdVIPs:     make([]string, 0),
		createdIfaces:   make([]string, 0),
		createdSNATRules: make([]string, 0),
		originalSysctls: make(map[string]string),
	}
}

func (m *IPVSManager) Setup() error {
	if err := m.configureSysctl(); err != nil {
		return fmt.Errorf("configure sysctl: %w", err)
	}

	exec.Command("modprobe", "ip_vs").Run()
	exec.Command("modprobe", "dummy").Run()

	for _, svc := range m.config.Services {
		if err := m.setupService(svc); err != nil {
			return fmt.Errorf("setup service %s:%d: %w", svc.VIP, svc.LocalPort, err)
		}
		if err := m.addSNATRules(svc); err != nil {
			return fmt.Errorf("add SNAT rules for %s:%d: %w", svc.VIP, svc.LocalPort, err)
		}
	}
	return nil
}

func (m *IPVSManager) configureSysctl() error {
	sysctlParams := map[string]string{
		"net.ipv4.vs.conntrack": "1",
		"net.ipv4.ip_forward":   "1",
	}

	for param, value := range sysctlParams {
		output, err := exec.Command("sysctl", "-n", param).Output()
		if err == nil {
			m.originalSysctls[param] = strings.TrimSpace(string(output))
		}

		if err := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", param, value)).Run(); err != nil {
			return fmt.Errorf("failed to set %s=%s: %w", param, value, err)
		}
		log.Printf("Set sysctl parameter: %s=%s", param, value)
		m.metrics.SysctlStatus.WithLabelValues(param).Set(1)
	}
	return nil
}

func (m *IPVSManager) setupService(svc config.Service) error {
	proto := "tcp"
	if svc.Protocol != "" { proto = svc.Protocol }
	iface := "ipvs0"
	if svc.Interface != "" { iface = svc.Interface }
	business := svc.Business
	if business == "" { business = "default" }

	if err := m.createDummyInterface(iface); err != nil {
		return err
	}

	// Add VIP to interface
	vipKey := fmt.Sprintf("%s/%s", svc.VIP, iface)
	output, _ := exec.Command("ip", "addr", "show", "dev", iface).Output()
	if !strings.Contains(string(output), svc.VIP+"/32") {
		if err := exec.Command("ip", "addr", "add", svc.VIP+"/32", "dev", iface).Run(); err != nil {
			return fmt.Errorf("add VIP to interface: %w", err)
		}
	} else {
		log.Printf("VIP %s already exists on interface %s, skipping", svc.VIP, iface)
	}

	m.mu.Lock()
	m.createdVIPs = append(m.createdVIPs, vipKey)
	m.mu.Unlock()

	// Add virtual service
	serviceKey := fmt.Sprintf("%s:%d:%s", svc.VIP, svc.LocalPort, proto)
	if err := exec.Command("ipvsadm", "-L", "-n", "-t", fmt.Sprintf("%s:%d", svc.VIP, svc.LocalPort)).Run(); err != nil {
		if err := exec.Command("ipvsadm", "-A", "-t", fmt.Sprintf("%s:%d", svc.VIP, svc.LocalPort), "-s", "rr").Run(); err != nil {
			return fmt.Errorf("add virtual service: %w", err)
		}
	} else {
		log.Printf("Virtual service %s:%d already exists, skipping creation", svc.VIP, svc.LocalPort)
	}

	m.mu.Lock()
	m.createdRules = append(m.createdRules, serviceKey)
	m.mu.Unlock()

	// Add real servers
	for _, backend := range svc.Backends {
		weight := backend.Weight
		if weight == 0 { weight = 1 }
		exec.Command("ipvsadm", "-a", "-t", fmt.Sprintf("%s:%d", svc.VIP, svc.LocalPort),
			"-r", fmt.Sprintf("%s:%d", backend.IP, backend.Port), "-m", "-w", strconv.Itoa(weight)).Run()

		vipLabel := fmt.Sprintf("%s:%d", svc.VIP, svc.LocalPort)
		backendLabel := fmt.Sprintf("%s:%d", backend.IP, backend.Port)
		m.metrics.BackendStatus.WithLabelValues(vipLabel, backendLabel, business).Set(1)
		m.metrics.BackendWeight.WithLabelValues(vipLabel, backendLabel, business).Set(float64(weight))
	}

	m.metrics.VipStatus.WithLabelValues(svc.VIP, iface, business).Set(1)
	return nil
}

func (m *IPVSManager) createDummyInterface(iface string) error {
	if _, err := exec.Command("ip", "link", "show", iface).Output(); err == nil {
		return nil
	}
	if err := exec.Command("ip", "link", "add", iface, "type", "dummy").Run(); err != nil {
		return fmt.Errorf("create dummy interface %s: %w", iface, err)
	}
	if err := exec.Command("ip", "link", "set", "dev", iface, "up").Run(); err != nil {
		return fmt.Errorf("bring up interface %s: %w", iface, err)
	}
	m.mu.Lock()
	m.createdIfaces = append(m.createdIfaces, iface)
	m.mu.Unlock()
	log.Printf("Created dummy interface: %s", iface)
	return nil
}

func (m *IPVSManager) addSNATRules(svc config.Service) error {
	localIP, err := utils.GetLocalIP()
	if err != nil { return err }

	rule := fmt.Sprintf("-t nat -A POSTROUTING -m ipvs --vaddr %s --vport %d -j SNAT --to-source %s",
		svc.VIP, svc.LocalPort, localIP)
	checkRule := fmt.Sprintf("-t nat -C POSTROUTING -m ipvs --vaddr %s --vport %d -j SNAT --to-source %s",
		svc.VIP, svc.LocalPort, localIP)

	if err := exec.Command("iptables", strings.Fields(checkRule)...).Run(); err != nil {
		if err := exec.Command("iptables", strings.Fields(rule)...).Run(); err != nil {
			return fmt.Errorf("add SNAT rule: %w", err)
		}
	}

	ruleKey := fmt.Sprintf("%s:%d->%s", svc.VIP, svc.LocalPort, localIP)
	m.mu.Lock()
	m.createdSNATRules = append(m.createdSNATRules, ruleKey)
	m.mu.Unlock()

	business := svc.Business
	if business == "" { business = "default" }
	m.metrics.SnatRulesCount.WithLabelValues(business).Set(float64(len(m.createdSNATRules)))
	return nil
}

func (m *IPVSManager) Monitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C: m.updateMetrics()
		}
	}
}

func (m *IPVSManager) updateMetrics() {
	// (Simplified logic for brevity, matches original main.go behavior)
	output, _ := exec.Command("ipvsadm", "-L", "-n", "--stats", "--exact").Output()
	m.parseIPVSStats(string(output))

	output, _ = exec.Command("ipvsadm", "-Ln").Output()
	m.parseIPVSConnections(string(output))

	m.updateVIPStatus()
}



func (m *IPVSManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Println("Cleaning up IPVS rules and interfaces...")
	for _, rule := range m.createdRules {
		parts := strings.Split(rule, ":")
		if len(parts) >= 2 {
			exec.Command("ipvsadm", "-D", "-t", fmt.Sprintf("%s:%s", parts[0], parts[1])).Run()
		}
	}

	for _, vipKey := range m.createdVIPs {
		parts := strings.Split(vipKey, "/")
		if len(parts) == 2 {
			exec.Command("ip", "addr", "del", parts[0]+"/32", "dev", parts[1]).Run()
		}
	}

	for _, iface := range m.createdIfaces {
		exec.Command("ip", "link", "del", iface).Run()
	}

	m.removeSNATRules()
	m.restoreSysctl()
}

func (m *IPVSManager) removeSNATRules() {
	localIP, _ := utils.GetLocalIP()
	for _, svc := range m.config.Services {
		rule := fmt.Sprintf("-t nat -D POSTROUTING -m ipvs --vaddr %s --vport %d -j SNAT --to-source %s",
			svc.VIP, svc.LocalPort, localIP)
		exec.Command("iptables", strings.Fields(rule)...).Run()
	}
}

func (m *IPVSManager) restoreSysctl() {
	for param, value := range m.originalSysctls {
		exec.Command("sysctl", "-w", fmt.Sprintf("%s=%s", param, value)).Run()
	}
}

func (m *IPVSManager) updateVIPStatus() {
	output, _ := exec.Command("ip", "addr", "show").Output()
	for _, svc := range m.config.Services {
		status := 0.0
		if strings.Contains(string(output), svc.VIP+"/32") { status = 1.0 }
		iface := svc.Interface
		if iface == "" { iface = "ipvs0" }
		business := svc.Business
		if business == "" { business = "default" }
		m.metrics.VipStatus.WithLabelValues(svc.VIP, iface, business).Set(status)
	}
}

func (m *IPVSManager) parseIPVSStats(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		// Match VIP:Port
		if strings.Contains(fields[0], ":") {
			service := fields[0]
			conns, _ := strconv.ParseFloat(fields[1], 64)
			inPkts, _ := strconv.ParseFloat(fields[2], 64)
			outPkts, _ := strconv.ParseFloat(fields[3], 64)
			inBytes, _ := strconv.ParseFloat(fields[4], 64)
			outBytes, _ := strconv.ParseFloat(fields[5], 64)

			business := m.getBusinessForService(service)
			m.metrics.ConnectionsTotal.WithLabelValues(service, business).Set(conns)
			m.metrics.PacketsInTotal.WithLabelValues(service, business).Set(inPkts)
			m.metrics.PacketsOutTotal.WithLabelValues(service, business).Set(outPkts)
			m.metrics.BytesInTotal.WithLabelValues(service, business).Set(inBytes)
			m.metrics.BytesOutTotal.WithLabelValues(service, business).Set(outBytes)
		}
	}
}

func (m *IPVSManager) parseIPVSConnections(output string) {
	lines := strings.Split(output, "\n")
	currentService := ""
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		if fields[0] == "Prot" || fields[0] == "RemoteAddress:Port" || fields[0] == "IP" {
			continue
		}

		if fields[0] == "TCP" || fields[0] == "UDP" {
			currentService = fields[1]
			continue
		}

		if strings.HasPrefix(line, "  ->") && currentService != "" {
			backend := fields[1]
			weight, _ := strconv.ParseFloat(fields[3], 64)
			active, _ := strconv.ParseFloat(fields[4], 64)
			inact, _ := strconv.ParseFloat(fields[5], 64)

			business := m.getBusinessForService(currentService)
			m.metrics.BackendConnections.WithLabelValues(currentService, backend, "active", business).Set(active)
			m.metrics.BackendConnections.WithLabelValues(currentService, backend, "inactive", business).Set(inact)
			m.metrics.BackendWeight.WithLabelValues(currentService, backend, business).Set(weight)

			// Update health status
			status := 1.0
			if err := m.checkBackendHealth(backend); err != nil {
				status = 0.0
			}
			m.metrics.BackendStatus.WithLabelValues(currentService, backend, business).Set(status)
		}
	}
}

func (m *IPVSManager) getBusinessForService(service string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// service is usually VIP:Port
	for _, svc := range m.config.Services {
		svcAddr := fmt.Sprintf("%s:%d", svc.VIP, svc.LocalPort)
		if svcAddr == service {
			if svc.Business != "" {
				return svc.Business
			}
			return "default"
		}
	}
	return "unknown"
}
func (m *IPVSManager) checkBackendHealth(backend string) error {
	conn, err := net.DialTimeout("tcp", backend, 2*time.Second)
	if err != nil { return err }
	conn.Close()
	return nil
}