package tester

import (
	"fmt"
	//"crypto/tls"
	//"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// HttpingResult 包含一次 HTTPing 测试的结果
type HttpingResult struct {
	Delay    time.Duration
	LossRate float64
	Colo     string
}

// TestLatency 通过 HTTPing 测试单个 IP 的延迟
func TestLatency(ip *net.IPAddr, testURL string, pingTimes int) (*HttpingResult, error) {
	hc := http.Client{
		Timeout: time.Second * 2,
		Transport: &http.Transport{
			DialContext: getDialContext(ip, DefaultTCPPort),
			//TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 跳过证书验证
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 阻止重定向
		},
	}

	// 先访问一次获得 HTTP 状态码 及 Cloudflare Colo
	var colo string
	{
		request, err := http.NewRequest(http.MethodHead, testURL, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.80 Safari/537.36")
		response, err := hc.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		// 默认只认为 200, 301, 302 才算 HTTPing 通过
		if response.StatusCode != 200 && response.StatusCode != 301 && response.StatusCode != 302 {
			return nil, fmt.Errorf("invalid status code: %d", response.StatusCode)
		}

		io.Copy(io.Discard, response.Body)

		// 通过头部 Server 值判断是 Cloudflare 还是 AWS CloudFront 并设置 cfRay 为各自的机场地区码完整内容
		colo = getHeaderColo(response.Header)
	}

	// 循环测速计算延迟
	success := 0
	var totalDelay time.Duration
	for i := 0; i < pingTimes; i++ {
		request, err := http.NewRequest(http.MethodHead, testURL, nil)
		if err != nil {
			log.Printf("创建请求失败: %v", err) // 使用 log 记录非致命错误
			continue
		}
		request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/98.0.4758.80 Safari/537.36")
		if i == pingTimes-1 {
			request.Header.Set("Connection", "close")
		}
		startTime := time.Now()
		response, err := hc.Do(request)
		if err != nil {
			continue
		}
		success++
		io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		duration := time.Since(startTime)
		totalDelay += duration
	}

	if success == 0 {
		return nil, fmt.Errorf("all pings failed")
	}

	result := &HttpingResult{
		Delay:    totalDelay / time.Duration(success),
		LossRate: float64(pingTimes-success) / float64(pingTimes),
		Colo:     colo,
	}

	return result, nil
}
