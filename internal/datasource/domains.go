package datasource

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDomainsFromFile 从指定路径的文件中读取域名列表。
// 它会忽略空行和以 '#' 开头的注释行。
func LoadDomainsFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开域名文件 '%s': %w", filePath, err)
	}
	defer file.Close()

	domainSet := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domainSet[line] = struct{}{} // 使用 map 自动去重
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取域名文件时出错: %w", err)
	}

	if len(domainSet) == 0 {
		return nil, fmt.Errorf("域名文件 '%s' 为空或未包含有效域名", filePath)
	}

	// 将 map 的 key 转换回 slice
	domains := make([]string, 0, len(domainSet))
	for domain := range domainSet {
		domains = append(domains, domain)
	}

	return domains, nil
}
