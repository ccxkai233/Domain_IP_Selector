package datasource

import (
	"Domain_IP_Selector_Go/internal/config"
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	// CFIPsV4URL Cloudflare IPv4 地址列表 URL
	CFIPsV4URL = "https://www.cloudflare.com/ips-v4"
	// CFIPsV6URL Cloudflare IPv6 地址列表 URL
	CFIPsV6URL = "https://www.cloudflare.com/ips-v6"
)

// IPNetSet 用于高效地检查 IP 是否属于某个范围
type IPNetSet struct {
	nets []*net.IPNet
}

// Contains 检查给定的 IP 是否在集合中
func (s *IPNetSet) Contains(ip net.IP) bool {
	for _, n := range s.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// LoadCFIPs 确保 Cloudflare IP 列表可用，并在必要时下载
func LoadCFIPs(cachePath string, cfg *config.Config) (*IPNetSet, error) {
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		fmt.Printf("本地缓存 '%s' 不存在，正在从 Cloudflare 官网下载...\n", cachePath)
		err := downloadAndCacheCFIPs(cachePath, cfg)
		if err != nil {
			return nil, fmt.Errorf("下载和缓存 Cloudflare IP 失败: %w", err)
		}
		fmt.Println("下载并缓存成功。")
	}

	return loadIPsFromFile(cachePath)
}

func downloadAndCacheCFIPs(filePath string, cfg *config.Config) error {
	var data []byte
	var err error

	ipVersion := cfg.IPVersion
	if ipVersion == "" {
		ipVersion = "ipv4" // 默认为 ipv4
	}

	switch ipVersion {
	case "ipv6":
		fmt.Println("正在下载 Cloudflare IPv6 地址...")
		data, err = downloadURL(CFIPsV6URL)
		if err != nil {
			return fmt.Errorf("下载 IPv6 列表失败: %w", err)
		}
	case "ipv4":
		fmt.Println("正在下载 Cloudflare IPv4 地址...")
		data, err = downloadURL(CFIPsV4URL)
		if err != nil {
			return fmt.Errorf("下载 IPv4 列表失败: %w", err)
		}
	default:
		return fmt.Errorf("无效的 IPVersion 配置: %s", cfg.IPVersion)
	}

	// 创建并写入文件
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建缓存文件失败: %w", err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("写入 IP 数据失败: %w", err)
	}

	return nil
}

func downloadURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func loadIPsFromFile(filePath string) (*IPNetSet, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开 IP 文件 '%s': %w", filePath, err)
	}
	defer file.Close()

	ipNetSet := &IPNetSet{nets: []*net.IPNet{}}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(line)
		if err != nil {
			// 忽略无法解析的行
			continue
		}
		ipNetSet.nets = append(ipNetSet.nets, ipNet)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取 IP 文件时出错: %w", err)
	}

	if len(ipNetSet.nets) == 0 {
		return nil, fmt.Errorf("IP 文件 '%s' 中未找到有效的 CIDR", filePath)
	}

	return ipNetSet, nil
}
