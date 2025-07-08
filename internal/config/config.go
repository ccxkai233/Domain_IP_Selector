package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 结构用于映射 config.yaml 文件的内容
type Config struct {
	DNSConcurrency         int      `yaml:"dns_concurrency"`
	LatencyTestConcurrency int      `yaml:"latency_test_concurrency"`
	SpeedTestConcurrency   int      `yaml:"speedtest_concurrency"`
	MaxLatency             int      `yaml:"max_latency"`
	TopNPerGroup           int      `yaml:"top_n_per_group"`
	IPVersion              string   `yaml:"ip_version"`
	SpeedTestRateLimitMB   float64  `yaml:"speedtest_rate_limit_mb"`
	GroupBy                string   `yaml:"group_by"`
	FilterRegions          []string `yaml:"filter_regions"`
	FilterColos            []string `yaml:"filter_colos"`
}

// LoadConfig 从指定路径加载和解析 YAML 配置文件
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
