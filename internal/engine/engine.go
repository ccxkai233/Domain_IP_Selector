package engine

import (
	"Domain_IP_Selector_Go/internal/config"
	"Domain_IP_Selector_Go/internal/datasource"
	"Domain_IP_Selector_Go/internal/locations"
	"Domain_IP_Selector_Go/internal/tester"
	"Domain_IP_Selector_Go/pkg/model"
	"context"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ProgressCallback 是一个用于报告进度的回调函数类型
type ProgressCallback func(message string)

// Run 启动 IP 优选引擎
// SimplifiedResult 定义了最终输出的扁平化数据结构
type SimplifiedResult struct {
	Address       string  `json:"Address"`
	SourceDomain  string  `json:"SourceDomain"`
	Delay         int64   `json:"Delay"` // 纳秒
	LossRate      float64 `json:"LossRate"`
	Colo          string  `json:"Colo"`
	Region        string  `json:"Region"`
	DownloadSpeed int     `json:"DownloadSpeed"` // MB/s
}

func Run(cfg *config.Config, locationsPath, domainsPath, exeDir string, progressCb ProgressCallback) ([]SimplifiedResult, error) {
	// --- 1. 初始化 ---
	progressCb("步骤 1/5: 初始化数据源...")
	regionMap, err := locations.LoadLocationsFromFile(locationsPath)
	if err != nil {
		return nil, fmt.Errorf("加载 locations.json 失败: %w", err)
	}

	var cfIPsCacheFile string
	ipVersion := cfg.IPVersion
	if ipVersion == "" {
		ipVersion = "ipv4" // 默认为 ipv4
	}
	if ipVersion == "ipv6" {
		cfIPsCacheFile = filepath.Join(exeDir, "cf-ips-ipv6.txt")
	} else {
		cfIPsCacheFile = filepath.Join(exeDir, "cf-ips-ipv4.txt")
	}

	cfIPSet, err := datasource.LoadCFIPs(cfIPsCacheFile, cfg)
	if err != nil {
		return nil, fmt.Errorf("加载 Cloudflare IP 列表失败: %w", err)
	}
	progressCb("初始化完成。")

	// --- 2. DNS 解析与 IP 筛选 ---
	progressCb("步骤 2/5: DNS 解析与 IP 筛选...")
	domains, err := datasource.LoadDomainsFromFile(domainsPath)
	if err != nil {
		return nil, fmt.Errorf("加载域名列表失败: %w", err)
	}

	initialIPs := resolveDomains(domains, cfg, progressCb)
	uniqueIPs := deduplicateIPs(initialIPs)
	cfIPs := filterCloudflareIPs(uniqueIPs, cfIPSet)
	progressCb(fmt.Sprintf("筛选出 %d 个 Cloudflare IP 地址。", len(cfIPs)))

	// --- 3. 延迟测试 ---
	progressCb("步骤 3/5: 延迟测试...")
	latencyResults := testLatencies(cfIPs, cfg, regionMap, progressCb)
	progressCb("延迟测试完成。")

	// --- 4. 过滤与分组 ---
	progressCb("步骤 4/5: 过滤与分组...")
	filteredResults := filterResults(latencyResults, cfg)
	groupedResults := groupResults(filteredResults, cfg.GroupBy)
	progressCb(fmt.Sprintf("已将 IP 按 '%s' 分为 %d 组。", cfg.GroupBy, len(groupedResults)))

	// --- 5. 下载速度测试 (带补充逻辑) ---
	progressCb("步骤 5/5: 下载速度测试...")
	finalResults := testSpeedsWithRetry(groupedResults, cfg, progressCb)
	progressCb("速度测试完成。")

	// 按下载速度倒序排序
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].DownloadSpeed > finalResults[j].DownloadSpeed
	})

	return finalResults, nil
}

// --- 各阶段的具体实现 ---

func resolveDomains(domains []string, cfg *config.Config, progressCb ProgressCallback) []model.IPInfo {
	var (
		initialIPs []model.IPInfo
		wg         sync.WaitGroup
		mu         sync.Mutex
	)
	progressCb(fmt.Sprintf("开始并发解析 %d 个域名...", len(domains)))

	// 增加对 DNSConcurrency 的检查
	if cfg.DNSConcurrency <= 0 {
		log.Printf("警告: DNSConcurrency 被设置为 %d，可能导致死锁。自动调整为默认值 10。", cfg.DNSConcurrency)
		cfg.DNSConcurrency = 10
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second} // 为拨号本身也增加超时
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}
	dnsSemaphore := make(chan struct{}, cfg.DNSConcurrency)

	for _, domain := range domains {
		wg.Add(1)
		go func(d string) {
			dnsSemaphore <- struct{}{}
			defer func() {
				<-dnsSemaphore
				wg.Done()
			}()

			var lookupType string
			switch cfg.IPVersion {
			case "ipv4":
				lookupType = "ip4"
			case "ipv6":
				lookupType = "ip6"
			default:
				lookupType = "ip"
			}

			// 为 DNS 查询添加超时
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			ips, err := resolver.LookupIP(ctx, lookupType, d)
			if err != nil {
				log.Printf("域名 %s 解析失败: %v", d, err)
				return
			}
			mu.Lock()
			for _, ip := range ips {
				initialIPs = append(initialIPs, model.IPInfo{Address: ip, SourceDomain: d})
			}
			mu.Unlock()
		}(domain)
	}
	wg.Wait()
	progressCb("所有域名解析完成。")
	return initialIPs
}

func deduplicateIPs(ips []model.IPInfo) []model.IPInfo {
	uniqueIPsMap := make(map[string]model.IPInfo)
	for _, ipInfo := range ips {
		ipStr := ipInfo.Address.String()
		if _, exists := uniqueIPsMap[ipStr]; !exists {
			uniqueIPsMap[ipStr] = ipInfo
		}
	}
	uniqueIPs := make([]model.IPInfo, 0, len(uniqueIPsMap))
	for _, ipInfo := range uniqueIPsMap {
		uniqueIPs = append(uniqueIPs, ipInfo)
	}
	return uniqueIPs
}

func filterCloudflareIPs(ips []model.IPInfo, cfIPSet *datasource.CFIPSet) []model.IPInfo {
	var cfIPs []model.IPInfo
	for _, ipInfo := range ips {
		if cfIPSet.Contains(ipInfo.Address) {
			cfIPs = append(cfIPs, ipInfo)
		}
	}
	return cfIPs
}

func testLatencies(ips []model.IPInfo, cfg *config.Config, regionMap locations.RegionMap, progressCb ProgressCallback) []model.LatencyResult {
	var (
		latencyResults []model.LatencyResult
		wg             sync.WaitGroup
		mu             sync.Mutex
	)
	progressCb(fmt.Sprintf("开始对 %d 个 Cloudflare IP 进行并发延迟测试...", len(ips)))

	// 增加对 LatencyTestConcurrency 的检查
	if cfg.LatencyTestConcurrency <= 0 {
		log.Printf("警告: LatencyTestConcurrency 被设置为 %d，可能导致死锁。自动调整为默认值 10。", cfg.LatencyTestConcurrency)
		cfg.LatencyTestConcurrency = 10
	}
	latencySemaphore := make(chan struct{}, cfg.LatencyTestConcurrency)

	for _, ipInfo := range ips {
		wg.Add(1)
		go func(ipInfo model.IPInfo) {
			latencySemaphore <- struct{}{}
			defer func() {
				<-latencySemaphore
				wg.Done()
			}()

			res, err := tester.TestLatency(&net.IPAddr{IP: ipInfo.Address}, "https://www.cloudflare.com/cdn-cgi/trace", 4)
			if err != nil {
				// log.Printf("IP %s 延迟测试失败: %v", ipInfo.Address, err)
				return
			}

			if res.LossRate > 0.1 || res.Delay > time.Duration(cfg.MaxLatency)*time.Millisecond {
				return
			}

			region, ok := regionMap.GetRegion(res.Colo)
			if !ok {
				region = "Unknown"
			}

			result := model.LatencyResult{
				IPInfo:   ipInfo,
				Delay:    res.Delay,
				LossRate: res.LossRate,
				Colo:     res.Colo,
				Region:   region,
			}

			mu.Lock()
			latencyResults = append(latencyResults, result)
			mu.Unlock()
			progressCb(fmt.Sprintf("IP %s: 延迟=%.2fms, 丢包=%.0f%%, Colo=%s, 区域=%s", ipInfo.Address, float64(res.Delay.Milliseconds()), res.LossRate*100, res.Colo, region))
		}(ipInfo)
	}
	wg.Wait()
	return latencyResults
}

func filterResults(results []model.LatencyResult, cfg *config.Config) []model.LatencyResult {
	var filtered []model.LatencyResult

	// 创建用于快速查找的 map
	regionFilter := make(map[string]bool)
	if len(cfg.FilterRegions) > 0 {
		for _, r := range cfg.FilterRegions {
			regionFilter[r] = true
		}
	}
	coloFilter := make(map[string]bool)
	if len(cfg.FilterColos) > 0 {
		for _, c := range cfg.FilterColos {
			coloFilter[c] = true
		}
	}

	for _, res := range results {
		pass := true
		if len(regionFilter) > 0 && !regionFilter[res.Region] {
			pass = false
		}
		if len(coloFilter) > 0 && !coloFilter[res.Colo] {
			pass = false
		}
		if pass {
			filtered = append(filtered, res)
		}
	}
	return filtered
}

func groupResults(results []model.LatencyResult, groupBy string) map[string][]model.LatencyResult {
	grouped := make(map[string][]model.LatencyResult)
	for _, res := range results {
		var key string
		switch groupBy {
		case "colo":
			key = res.Colo
		case "region":
			fallthrough
		default:
			key = res.Region
		}
		grouped[key] = append(grouped[key], res)
	}

	// 对每个分组按延迟排序
	for key := range grouped {
		sort.Slice(grouped[key], func(i, j int) bool {
			return grouped[key][i].Delay < grouped[key][j].Delay
		})
	}
	return grouped
}

func testSpeedsWithRetry(groupedResults map[string][]model.LatencyResult, cfg *config.Config, progressCb ProgressCallback) []SimplifiedResult {
	var (
		finalResults []SimplifiedResult
		wg           sync.WaitGroup
		mu           sync.Mutex
	)

	const testURL = "https://speed.cloudflare.com/__down?bytes=200000000"

	speedTestSemaphore := make(chan struct{}, cfg.SpeedTestConcurrency)

	for groupName, candidates := range groupedResults {
		wg.Add(1)
		go func(groupName string, candidates []model.LatencyResult) {
			defer wg.Done()
			var successfulTests []SimplifiedResult

			progressCb(fmt.Sprintf("开始测试分组 '%s'，目标 %d 个，候选 %d 个...", groupName, cfg.TopNPerGroup, len(candidates)))

			for _, candidate := range candidates {
				// 如果已经收集到足够的结果，则停止该分组的测试
				if len(successfulTests) >= cfg.TopNPerGroup {
					break
				}

				speedTestSemaphore <- struct{}{}

				speedRes, err := tester.TestDownloadSpeed(&net.IPAddr{IP: candidate.IPInfo.Address}, testURL, 10*time.Second, cfg.SpeedTestRateLimitMB)

				<-speedTestSemaphore

				if err != nil {
					progressCb(fmt.Sprintf("IP %s 速度测试失败: %v", candidate.IPInfo.Address, err))
					continue // 失败，继续下一个候选
				}

				// 检查速度是否低于最低要求
				speedInMBps := speedRes.DownloadSpeed / 1024 / 1024
				if cfg.MinSpeed > 0 && speedInMBps < cfg.MinSpeed {
					progressCb(fmt.Sprintf("IP %s 速度 %.2f MB/s 低于最低要求 %.2f MB/s, 已舍弃", candidate.IPInfo.Address, speedInMBps, cfg.MinSpeed))
					continue // 速度不达标，继续下一个候选
				}

				result := SimplifiedResult{
					Address:       candidate.IPInfo.Address.String(),
					SourceDomain:  candidate.IPInfo.SourceDomain,
					Delay:         candidate.Delay.Nanoseconds(),
					LossRate:      candidate.LossRate,
					Colo:          candidate.Colo,
					Region:        candidate.Region,
					DownloadSpeed: int(speedRes.DownloadSpeed / 1024), // B/s to KB/s, then to int
				}

				mu.Lock()
				successfulTests = append(successfulTests, result)
				finalResults = append(finalResults, result)
				mu.Unlock()

				progressCb(fmt.Sprintf("IP %s: 下载速度=%.2f MB/s (分组: %s)", candidate.IPInfo.Address, float64(result.DownloadSpeed)/1024.0, groupName))
			}
			progressCb(fmt.Sprintf("分组 '%s' 测试完成，成功获取 %d 个结果。", groupName, len(successfulTests)))

		}(groupName, candidates)
	}

	wg.Wait()
	return finalResults
}
