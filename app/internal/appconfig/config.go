package appconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"fire-salamander-desktop/internal/runtimeenv"
)

const DefaultAppConfigJSON = `{
  "app": {
    "version": "1.0.0",
    "language": "zh-CN"
  },
  "execution": {
    "defaultPython": "python3",
    "maxHistory": 50,
    "remoteTimeoutSec": 30
  },
  "export": {
    "lastDirectory": "",
    "goMode": "binary",
    "autoOpenDir": true
  },
  "go": {
    "selectedBinary": "",
    "knownBinaries": [],
    "lastInstallDirectory": "",
    "disabled": false
  },
  "ui": {
    "theme": "dracula",
    "verboseShortcuts": false
  },
  "window": {
    "width": 0,
    "height": 0,
    "x": -1,
    "y": -1,
    "maximised": false,
    "fullscreen": false
  }
}
`

func DefaultDocument() map[string]json.RawMessage {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(DefaultAppConfigJSON), &cfg); err != nil || cfg == nil {
		return map[string]json.RawMessage{}
	}
	return cfg
}

func LoadDocument(configPath string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultDocument(), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil || cfg == nil {
		return DefaultDocument(), nil
	}
	return cfg, nil
}

func WriteDocument(configPath string, doc map[string]json.RawMessage) error {
	output, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("格式化配置文件失败: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

func ResolveConfigPath() (string, error) {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return "", fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := os.MkdirAll(layout.ConfigDir(), 0755); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}
	return filepath.Join(layout.ConfigDir(), "app.json"), nil
}
