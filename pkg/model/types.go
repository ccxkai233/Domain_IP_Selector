package model

import (
	"net"
	"time"
)

// IPInfo 包含从域名解析出的初始 IP 信息
type IPInfo struct {
	Address      net.IP
	SourceDomain string // 从哪个域名解析出来的
}

// LatencyResult 包含 HTTPing 延迟测试后的结果
type LatencyResult struct {
	IPInfo
	Delay    time.Duration
	LossRate float64
	Colo     string // e.g., "SJC"
	Region   string // e.g., "North America"
}

// FinalResult 包含所有信息的最终结果
type FinalResult struct {
	LatencyResult
	DownloadSpeed float64 // in MB/s
}