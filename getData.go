package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 获取抽卡数据的函数，接受URL作为输入，解析参数并请求API获取数据，最后保存到本地JSON文件中
func getGachaData(rawUrl string) error {
	urlParts := strings.Split(rawUrl, "?")
	if len(urlParts) < 2 {
		return fmt.Errorf("URL格式错误")
	}

	params, err := url.ParseQuery(urlParts[1])
	if err != nil {
		return fmt.Errorf("参数解析失败: %v", err)
	}

	playerId := params.Get("player_id")
	recordId := params.Get("record_id")
	gachaId := params.Get("gacha_id")
	serverId := params.Get("svr_id")
	lang := params.Get("lang")

	localFileName := "Datas/gacha_storage.json"
	var storage map[string]PlayerData

	fileData, err := os.ReadFile(localFileName)
	if err == nil {
		json.Unmarshal(fileData, &storage)
	} else {
		storage = make(map[string]PlayerData)
	}

	if _, exists := storage[playerId]; !exists {
		storage[playerId] = PlayerData{
			Uid:       playerId,
			GachaData: make(map[string][]GachaItem),
		}
	}

	playerData := storage[playerId]
	if playerData.GachaData == nil {
		playerData.GachaData = make(map[string][]GachaItem)
	}

	apiUrl := "https://gmserver-api.aki-game2.net/gacha/record/query"
	poolTypes := []int{1, 2, 3, 4, 5, 6, 7, 8}

	for _, poolType := range poolTypes {
		poolKey := fmt.Sprintf("%d", poolType)

		payload := map[string]any{
			"playerId":     playerId,
			"recordId":     recordId,
			"cardPoolId":   gachaId,
			"cardPoolType": poolType,
			"serverId":     serverId,
			"languageCode": lang,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Printf("请求卡池 %d 失败: %v\n", poolType, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var gachaResp GachaResponse
		json.Unmarshal(body, &gachaResp)

		if gachaResp.Code != 0 || len(gachaResp.Data) == 0 {
			fmt.Printf("卡池 %d 无数据或请求失败: %s\n[请确保此前已在游戏内打开历史记录页面]\n", poolType, gachaResp.Message)
			continue
		}

		existingData := playerData.GachaData[poolKey]
		existingKeys := make(map[string]bool)
		for _, item := range existingData {
			key := item.Time + item.Name
			existingKeys[key] = true
		}

		newDataAdded := 0
		for _, item := range gachaResp.Data {
			key := item.Time + item.Name
			if !existingKeys[key] {
				existingData = append(existingData, item)
				newDataAdded++
			}
		}

		playerData.GachaData[poolKey] = existingData
		fmt.Printf("卡池 %d 同步完成，新增 %d 条记录\n", poolType, newDataAdded)
	}

	storage[playerId] = playerData

	finalJson, _ := json.MarshalIndent(storage, "", "  ")
	os.WriteFile(localFileName, finalJson, 0644)
	fmt.Println("所有数据已成功保存并合并到", localFileName)

	return nil
}

// 获取日志文件中最新的抽卡URL，使用正则表达式匹配并返回最后一个匹配项
func getLatestGachaUrl(logFilePath string) (string, error) {
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`https://[^\s"'<>]+gacha[^\s"'<>]+`)
	matches := re.FindAllString(string(content), -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("未在日志中找到抽卡链接，请确认是否在游戏内打开了历史记录页面")
	}

	return matches[len(matches)-1], nil
}

// 全局静态变量定义
func loadConfig() AppConfig {
	var cfg AppConfig
	data, err := os.ReadFile(configFilePath)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	return cfg
}

// 保存配置到本地JSON文件中
func saveConfig(cfg AppConfig) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile("config.json", data, 0644)
}

// 获取日志路径
func searchLogFile(cfg *AppConfig) string {
	if cfg.LogPath != "" {
		if _, err := os.Stat(cfg.LogPath); err == nil {
			fmt.Println("使用缓存的日志路径:", cfg.LogPath)
			return cfg.LogPath
		}
		fmt.Println("无效的缓存日志路径，重新搜索...")
	}

	var foundPaths []string
	targetSubPath := filepath.Join("Client", "Saved", "Logs", "Client.log")

	fmt.Println("正在全盘自动搜索游戏日志文件，请稍候...")

	drives := "CDEFGHIJKLMNOPQRSTUVWXYZ"
	for _, d := range drives {
		driveRoot := string(d) + ":\\"
		if _, err := os.Stat(driveRoot); err == nil {
			filepath.WalkDir(driveRoot, func(path string, info os.DirEntry, err error) error {
				if err != nil || !info.IsDir() {
					return nil
				}

				name := strings.ToLower(info.Name())
				if name == "windows" || name == "programdata" || name == "$recycle.bin" || name == "system volume information" {
					return filepath.SkipDir
				}

				if name == "wuthering waves game" {
					checkPath := filepath.Join(path, targetSubPath)
					if _, err := os.Stat(checkPath); err == nil {
						foundPaths = append(foundPaths, checkPath)
					}
					return filepath.SkipDir
				}
				return nil
			})
		}
	}

	var finalPath string
	if len(foundPaths) == 1 {
		finalPath = foundPaths[0]
		fmt.Println("已自动定位到日志文件:", finalPath)
	} else {
		reader := bufio.NewReader(os.Stdin)
		if len(foundPaths) > 1 {
			fmt.Println("\n检测到多个可能的日志文件：")
			for _, p := range foundPaths {
				fmt.Println("-", p)
			}
			fmt.Println("请手动复制并输入你要使用的准确路径：")
		} else {
			fmt.Println("\n全盘搜索完毕，未自动找到日志文件。")
			fmt.Println("请手动输入 Client.log 的完整绝对路径：")
		}

		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		finalPath = strings.Trim(strings.TrimSpace(input), "\"'")
	}

	if finalPath != "" {
		cfg.LogPath = finalPath
		saveConfig(*cfg)
	}

	return finalPath
}
