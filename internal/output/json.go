package output

import (
	"Domain_IP_Selector_Go/pkg/model"
	"encoding/json"
	"fmt"
	"os"
)

// WriteJSONFile 将最终结果列表写入到指定的 JSON 文件中
func WriteJSONFile(filePath string, results []model.FinalResult) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("无法将结果序列化为 JSON: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("无法写入 JSON 文件 '%s': %w", filePath, err)
	}

	return nil
}