package scanner

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// DiscoveredDevice represents a scanner found on the local network.
type DiscoveredDevice struct {
	IP            string `json:"ip"`
	Model         string `json:"model"`
	DisplayStatus string `json:"display_status"`
	Online        bool   `json:"online"`
	HasADF        bool   `json:"has_adf"`
	DocInADF      bool   `json:"doc_in_adf"`
	TonerPercent  int    `json:"toner_percent"`
	MACAddress    string `json:"mac_address"`
}

// DiscoverScanners probes the local subnet and WS-Discovery multicast to find scanners.
func DiscoverScanners(ctx context.Context, defaultIP string) []DiscoveredDevice {
	if defaultIP == "" {
		defaultIP = "192.168.1.50"
	}

	foundMap := make(map[string]DiscoveredDevice)
	var mu sync.Mutex

	probeHost := func(ip string) {
		driver := NewSamsungScannerDriver(ip, 9400)
		probeCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
		defer cancel()

		st, err := driver.GetStatus(probeCtx)
		if err == nil && st != nil && st.Online {
			mu.Lock()
			foundMap[ip] = DiscoveredDevice{
				IP:            st.IP,
				Model:         st.Model,
				DisplayStatus: st.DisplayStatus,
				Online:        st.Online,
				HasADF:        st.HasADF,
				DocInADF:      st.DocInADF,
				TonerPercent:  st.TonerPercent,
				MACAddress:    st.MACAddress,
			}
			mu.Unlock()
		}
	}

	// 1. Check default target IP first
	probeHost(defaultIP)

	mu.Lock()
	if len(foundMap) > 0 {
		var result []DiscoveredDevice
		for _, dev := range foundMap {
			result = append(result, dev)
		}
		mu.Unlock()
		return result
	}
	mu.Unlock()

	// 2. Discover local subnets
	subnets := getLocalSubnets()
	if len(subnets) == 0 {
		subnets = []string{"192.168.1"}
	}

	var candidateIPs []string
	for _, sub := range subnets {
		for i := 1; i <= 254; i++ {
			ip := fmt.Sprintf("%s.%d", sub, i)
			if ip != defaultIP {
				candidateIPs = append(candidateIPs, ip)
			}
		}
	}

	// Scan candidate IPs in parallel with worker pool
	sem := make(chan struct{}, 40)
	var wg sync.WaitGroup

	for _, ip := range candidateIPs {
		select {
		case <-ctx.Done():
			break
		default:
		}

		// Quick port check before full query
		targetIP := ip
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			if isPortOpen(targetIP, 9400, 300*time.Millisecond) || isPortOpen(targetIP, 8018, 300*time.Millisecond) {
				probeHost(targetIP)
			}
		}()
	}

	wg.Wait()

	var result []DiscoveredDevice
	mu.Lock()
	for _, dev := range foundMap {
		result = append(result, dev)
	}
	mu.Unlock()

	return result
}

func isPortOpen(ip string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func getLocalSubnets() []string {
	var subnets []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return subnets
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				ip := ipNet.IP.To4()
				subnet := fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
				subnets = append(subnets, subnet)
			}
		}
	}
	return subnets
}
