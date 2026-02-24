package utils

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func CheckPortAvailable(port int) error {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	listener.Close()
	return nil
}

func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no valid local IP found")
}

func IsVIPExists(vip string) bool {
	output, err := exec.Command("ip", "addr", "show").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), vip+"/")
}
