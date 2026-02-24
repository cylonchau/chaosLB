package config

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Backend struct {
	IP     string `json:"ip" yaml:"ip"`
	Port   int    `json:"port" yaml:"port"`
	Weight int    `json:"weight" yaml:"weight"`
}

type Service struct {
	VIP       string    `json:"vip" yaml:"vip"`
	LocalPort int       `json:"local_port" yaml:"local_port"`
	Backends  []Backend `json:"backends" yaml:"backends"`
	Protocol  string    `json:"protocol" yaml:"protocol"`
	Interface string    `json:"interface" yaml:"interface"`
	Business  string    `json:"business" yaml:"business"`
}

type Config struct {
	Services    []Service `json:"services" yaml:"services"`
	MetricsPort int       `json:"metrics_port" yaml:"metrics_port"`
}

func LoadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		// Try YAML if JSON fails
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("parse config (JSON/YAML): %w", err)
		}
	}

	return &config, nil
}
