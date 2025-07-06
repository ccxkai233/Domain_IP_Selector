package main

import (
	"Domain_IP_Selector_Go/internal/config"
	"Domain_IP_Selector_Go/internal/datasource"
	"Domain_IP_Selector_Go/internal/locations"
	"Domain_IP_Selector_Go/internal/output"
	"Domain_IP_Selector_Go/internal/tester"
	"Domain_IP_Selector_Go/pkg/model"
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

//go:embed default_config.yaml
var defaultConfigData []byte

//go:embed locations.json
var defaultLocationsData []byte

//go:embed reputation_domains.txt
var defaultDomainsData []byte

// ensureFile 检查文件是否存在于可执行文件目录，如果不存在，则使用提供的默认数据创建它。
// 返回最终的文件路径和错误。
func ensureFile(fileName string, defaultData []byte) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法获取可执行文件路径: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	filePath := filepath.Join(exeDir, fileName)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); err == nil {
		// 文件已存在，直接返回路径
		return filePath, nil
	} else if !os.IsNotExist(err) {
		// 其他类型的错误 (如权限问题)
		return "", fmt.Errorf("检查文件 %s 时出错: %w", fileName, err)
	}

	// 文件不存在，写入默认数据
	if err := os.WriteFile(filePath, defaultData, 0644); err != nil {
		return "", fmt.Errorf("无法写入默认文件 %s: %w", fileName, err)
	}

	log.Printf("首次运行，已在 %s 生成默认 %s 文件", exeDir, fileName)
	return filePath, nil
}

const (
	// 文件路径常量
	// locationsFile         = "locations.json" // 将由 ensureFile 动态确定
	// reputationDomainsFile = "reputation_domains.txt" // 将由 ensureFile 动态确定
	// configFile            = "config.yaml" // 将由 ensureConfig 动态确定

	// 测试参数
	latencyTestURL    = "https://www.cloudflare.com/cdn-cgi/trace"
	speedTestURL      = "https://speed.cloudflare.com/__down?bytes=200000000"
	pingTimes         = 4
	latencyThreshold  = 300 * time.Millisecond
	lossRateThreshold = 0.1 // 10%
	topNPerRegion     = 5
	speedTestTimeout  = 10 * time.Second
)

func main() {
	log.Println("--- 开始域名优选 IP 引擎 ---")

	// 0. 确保所有必需的文件都存在，如果不存在则从嵌入的数据创建
	log.Println("步骤 1/5: 检查并生成所需文件...")
	cfgPath, err := ensureFile("config.yaml", defaultConfigData)
	if err != nil {
		log.Fatalf("初始化配置文件失败: %v", err)
	}
	locationsPath, err := ensureFile("locations.json", defaultLocationsData)
	if err != nil {
		log.Fatalf("初始化 locations.json 失败: %v", err)
	}
	domainsPath, err := ensureFile("reputation_domains.txt", defaultDomainsData)
	if err != nil {
		log.Fatalf("初始化 reputation_domains.txt 失败: %v", err)
	}
	log.Println("文件检查完成。")

	// 1. 加载配置和初始化
	log.Println("步骤 2/5: 加载配置和初始化数据源...")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	log.Printf("配置加载成功：DNS并发数=%d, 延迟测试并发数=%d, 下载测速并发数=%d", cfg.DNSConcurrency, cfg.LatencyTestConcurrency, cfg.SpeedTestConcurrency)

	// 获取可执行文件所在目录，用于解析其他相对路径
	exeDir := filepath.Dir(cfgPath)

	regionMap, err := locations.LoadLocationsFromFile(locationsPath)
	if err != nil {
		log.Fatalf("加载 locations.json 失败: %v", err)
	}

	// 根据 IP 版本确定缓存文件名
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
		log.Fatalf("加载 Cloudflare IP 列表失败: %v", err)
	}
	log.Println("初始化完成。")

	// 3. IP 筛选与分组
	log.Println("步骤 3/5: IP 筛选与延迟测试...")
	domains, err := datasource.LoadDomainsFromFile(domainsPath)
	if err != nil {
		log.Fatalf("加载域名列表失败: %v", err)
	}

	// 并发 DNS 解析所有域名
	var (
		initialIPs []model.IPInfo
		wg         sync.WaitGroup
		mu         sync.Mutex
	)
	log.Printf("开始并发解析 %d 个域名...", len(domains))

	// 创建一个自定义的 DNS resolver，强制使用 1.1.1.1
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
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

			fmt.Printf("正在解析域名: %s\n", d)
			// 根据配置选择解析 "ip4" 或 "ip6"
			var lookupType string
			switch cfg.IPVersion {
			case "ipv4":
				lookupType = "ip4"
			case "ipv6":
				lookupType = "ip6"
			default:
				lookupType = "ip"
			}
			ips, err := resolver.LookupIP(context.Background(), lookupType, d)
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
	log.Println("所有域名解析完成。")

	// IP 去重
	uniqueIPsMap := make(map[string]model.IPInfo)
	for _, ipInfo := range initialIPs {
		ipStr := ipInfo.Address.String()
		if _, exists := uniqueIPsMap[ipStr]; !exists {
			uniqueIPsMap[ipStr] = ipInfo
		}
	}
	uniqueIPs := make([]model.IPInfo, 0, len(uniqueIPsMap))
	for _, ipInfo := range uniqueIPsMap {
		uniqueIPs = append(uniqueIPs, ipInfo)
	}
	log.Printf("从 %d 个解析结果中发现 %d 个独立 IP 地址。", len(initialIPs), len(uniqueIPs))

	// 过滤非 Cloudflare IP
	var cfIPs []model.IPInfo
	for _, ipInfo := range uniqueIPs {
		if cfIPSet.Contains(ipInfo.Address) {
			cfIPs = append(cfIPs, ipInfo)
		}
	}
	log.Printf("过滤后剩余 %d 个 Cloudflare IP 地址。", len(cfIPs))

	// 并发进行 HTTPing 延迟测试
	var (
		latencyResults []model.LatencyResult
		// wg 和 mu 已经在前面定义过，这里可以复用
	)
	log.Printf("开始对 %d 个 Cloudflare IP 进行并发延迟测试...", len(cfIPs))

	latencySemaphore := make(chan struct{}, cfg.LatencyTestConcurrency)

	for _, ipInfo := range cfIPs {
		wg.Add(1)
		go func(ipInfo model.IPInfo) {
			latencySemaphore <- struct{}{}
			defer func() {
				<-latencySemaphore
				wg.Done()
			}()

			res, err := tester.TestLatency(&net.IPAddr{IP: ipInfo.Address}, latencyTestURL, pingTimes)
			if err != nil {
				log.Printf("IP %s 延迟测试失败: %v", ipInfo.Address, err)
				return
			}

			// 在 goroutine 内部过滤掉不满足条件的 IP
			// 使用配置的最大延迟进行过滤
			if res.LossRate > lossRateThreshold || res.Delay > time.Duration(cfg.MaxLatency)*time.Millisecond {
				// log.Printf("IP %s 被过滤: 延迟=%.2fms, 丢包=%.0f%%", ipInfo.Address, float64(res.Delay.Milliseconds()), res.LossRate*100)
				return
			}

			// 标注区域
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
			log.Printf("IP %s: 延迟=%.2fms, 丢包=%.0f%%, Colo=%s, 区域=%s", ipInfo.Address, float64(res.Delay.Milliseconds()), res.LossRate*100, res.Colo, region)
		}(ipInfo)
	}

	wg.Wait()
	log.Println("延迟测试完成。")

	// 按 Region 分组
	resultsByRegion := make(map[string][]model.LatencyResult)
	for _, res := range latencyResults {
		resultsByRegion[res.Region] = append(resultsByRegion[res.Region], res)
	}

	// 4. 测速与输出
	log.Println("步骤 4/5: 下载速度测试...")
	var finalSpeedQueue []model.LatencyResult
	for _, regionResults := range resultsByRegion {
		// 按延迟排序
		sort.Slice(regionResults, func(i, j int) bool {
			return regionResults[i].Delay < regionResults[j].Delay
		})
		// 取 Top N
		// 取 Top N (根据配置)
		limit := cfg.TopNPerRegion
		if len(regionResults) < limit {
			limit = len(regionResults)
		}
		finalSpeedQueue = append(finalSpeedQueue, regionResults[:limit]...)
	}
	log.Printf("已汇集 %d 个 IP 进入最终测速队列。", len(finalSpeedQueue))

	// 并发进行下载速度测试（使用工作池模式限制并发）
	var finalResults []model.FinalResult
	semaphore := make(chan struct{}, cfg.SpeedTestConcurrency) // 创建一个信号量
	// wg 和 mu 已在前面定义和使用，这里直接复用

	for _, res := range finalSpeedQueue {
		wg.Add(1)
		go func(r model.LatencyResult) {
			semaphore <- struct{}{} // 获取一个令牌
			defer func() {
				<-semaphore // 释放令牌
				wg.Done()
			}()

			speedRes, err := tester.TestDownloadSpeed(&net.IPAddr{IP: r.Address}, speedTestURL, speedTestTimeout)
			if err != nil {
				log.Printf("IP %s 速度测试失败: %v", r.Address, err)
				return // 失败则不记录
			}

			result := model.FinalResult{
				LatencyResult: r,
				DownloadSpeed: speedRes.DownloadSpeed / 1024 / 1024, // B/s to MB/s
			}

			mu.Lock()
			finalResults = append(finalResults, result)
			mu.Unlock()
			log.Printf("IP %s: 下载速度=%.2f MB/s", r.Address, result.DownloadSpeed)
		}(res)
	}

	wg.Wait()

	// 按下载速度倒序排序
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].DownloadSpeed > finalResults[j].DownloadSpeed
	})
	log.Println("速度测试完成。")

	// 4. 写入结果
	log.Println("步骤 4/4: 写入结果文件...")
	resultJSONFile := filepath.Join(exeDir, fmt.Sprintf("result_%s.json", ipVersion))
	resultCSVFile := filepath.Join(exeDir, fmt.Sprintf("result_%s.csv", ipVersion))

	err = output.WriteJSONFile(resultJSONFile, finalResults)
	if err != nil {
		log.Fatalf("写入 result.json 失败: %v", err)
	}
	err = output.WriteCSVFile(resultCSVFile, finalResults)
	if err != nil {
		log.Fatalf("写入 result.csv 失败: %v", err)
	}
	log.Printf("结果已成功写入 %s 和 %s", resultJSONFile, resultCSVFile)

	log.Println("--- 所有任务已完成 ---")
}
