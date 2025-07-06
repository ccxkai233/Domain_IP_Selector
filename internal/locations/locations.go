package locations

import (
	"encoding/json"
	"fmt"
	"os"
)

// RegionMap 用于存储 IATA 代码到区域的映射
type RegionMap map[string]string

// LoadLocationsFromFile 从指定的 JSON 文件加载位置数据
func LoadLocationsFromFile(filePath string) (RegionMap, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法读取位置文件 '%s': %w", filePath, err)
	}

	// 临时的结构，用于解析JSON数组中的每个对象
	type locationEntry struct {
		IATA   string `json:"iata"`
		Region string `json:"region"`
	}

	var entries []locationEntry
	err = json.Unmarshal(data, &entries)
	if err != nil {
		return nil, fmt.Errorf("解析位置文件 JSON 失败: %w", err)
	}

	// 将解析出的列表转换为map
	regionMap := make(RegionMap)
	for _, entry := range entries {
		if entry.IATA != "" && entry.Region != "" {
			regionMap[entry.IATA] = entry.Region
		}
	}

	return regionMap, nil
}

// GetRegion 根据 IATA 代码从映射中查找区域
func (rm RegionMap) GetRegion(iataCode string) (string, bool) {
	region, ok := rm[iataCode]
	return region, ok
}
