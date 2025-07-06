package tester

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
)

const (
	// DefaultTCPPort 默认测速端口
	DefaultTCPPort = 443
)

var (
	// ColoRegexp 用于从 cf-ray 中提取数据中心代码
	ColoRegexp = regexp.MustCompile(`[A-Z]{3}`)
)

// isIPv4 检查 IP 地址是否为 IPv4
func isIPv4(ip string) bool {
	return strings.Contains(ip, ".")
}

// getDialContext 创建一个自定义的拨号上下文，强制通过指定的 IP 地址进行连接
func getDialContext(ip *net.IPAddr, port int) func(ctx context.Context, network, address string) (net.Conn, error) {
	var fakeSourceAddr string
	if isIPv4(ip.String()) {
		fakeSourceAddr = fmt.Sprintf("%s:%d", ip.String(), port)
	} else {
		fakeSourceAddr = fmt.Sprintf("[%s]:%d", ip.String(), port)
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, fakeSourceAddr)
	}
}

// getHeaderColo 从响应头中获取数据中心（Colo）代码
func getHeaderColo(header http.Header) (colo string) {
	// 如果是 Cloudflare 的服务器，则获取 cf-ray 头部
	if header.Get("Server") == "cloudflare" {
		colo = header.Get("cf-ray") // 示例 cf-ray: 7bd32409eda7b020-SJC
	} else { // 其他 CDN 的逻辑可以根据需要添加
		colo = header.Get("x-amz-cf-pop") // AWS CloudFront
	}

	if colo == "" {
		return ""
	}
	// 正则匹配并返回机场地区码
	return ColoRegexp.FindString(colo)
}