package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// 全局静态变量定义
var (
	configFilePath      = "config.json"                // 配置文件路径
	gacha_storagePath   = "Datas/gacha_storage.json"   // 抽卡数据存储路径
	analyse_storagePath = "Datas/analysis_result.json" // 分析结果存储路径
)

// 响应总结构体定义
type GachaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []GachaItem `json:"data"`
}

// 响应Data字段的结构体定义，同存储结构体一致
type GachaItem struct {
	CardPoolType string `json:"cardPoolType"`
	ResourceId   int    `json:"resourceId"`
	QualityLevel int    `json:"qualityLevel"`
	ResourceType string `json:"resourceType"`
	Name         string `json:"name"`
	Count        int    `json:"count"`
	Time         string `json:"time"`
}

// 本地存储结构体定义，包含玩家ID和对应的抽卡数据
type PlayerData struct {
	Uid       string                 `json:"uid"`
	GachaData map[string][]GachaItem `json:"gachaData"`
}

// config.json配置结构体定义
type AppConfig struct {
	LogPath string `json:"logPath"`
	LastUid string `json:"lastUid"`
}

func main() {
	cfg := loadConfig()
	reader := bufio.NewReader(os.Stdin)

	dataDir := "Datas"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		err := os.MkdirAll(dataDir, 0755)
		if err != nil {
			fmt.Printf("无法创建目录 %s: %v\n", dataDir, err)
			return
		}
	}

	for {
		fmt.Println("\n==================================")
		fmt.Println("       鸣潮抽卡分析工具")
		fmt.Println("==================================")
		fmt.Println("  [1] 同步最新抽卡记录")
		fmt.Println("  [2] 生成可视化分析报告")
		fmt.Println("  [0] 退出程序")
		fmt.Println("==================================")
		fmt.Print("请输入序号选择功能: ")

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			logPath := searchLogFile(&cfg)
			if logPath == "" {
				fmt.Println("未获得日志文件地址，同步取消。")
				continue
			}

			rawUrl, err := getLatestGachaUrl(logPath)
			if err != nil {
				fmt.Println("提取抽卡链接失败:", err)
				continue
			}

			if err := getGachaData(rawUrl); err != nil {
				fmt.Println("获取抽卡数据失败:", err)
			}

		case "2":
			uidPrompt := "请输入要分析的玩家 ID"
			if cfg.LastUid != "" {
				uidPrompt += fmt.Sprintf(" (回车默认使用 %s)", cfg.LastUid)
			} else {
				uidPrompt += " (回车自动分析第一个账号)"
			}
			uidPrompt += ": "

			fmt.Print(uidPrompt)
			uidInput, _ := reader.ReadString('\n')
			targetUid := strings.TrimSpace(uidInput)

			if targetUid == "" && cfg.LastUid != "" {
				targetUid = cfg.LastUid
				fmt.Println("使用缓存的 UID:", targetUid)
			} else if targetUid != "" {
				cfg.LastUid = targetUid
				saveConfig(cfg)
			}

			analyseData(targetUid)

			serveAndOpen()

		case "0":
			fmt.Println("程序已退出。")
			return

		default:
			fmt.Println("输入无效，请重新输入 0、1 或 2。")
		}
	}
}
