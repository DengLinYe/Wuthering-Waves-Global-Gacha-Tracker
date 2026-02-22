package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"time"
)

type PullRecord struct {
	Name  string `json:"name"`
	Pulls int    `json:"pulls"`
	Time  string `json:"time"`
}

type PoolAnalysis struct {
	PoolName      string       `json:"poolName"`
	TotalPulls    int          `json:"totalPulls"`
	CurrentPity   int          `json:"currentPity"`
	Avg5StarPulls float64      `json:"avg5StarPulls"`
	FiveStarCount int          `json:"fiveStarCount"`
	Avg4StarPulls float64      `json:"avg4StarPulls"`
	FourStarCount int          `json:"fourStarCount"`
	History5Star  []PullRecord `json:"history5Star"`
}

type GlobalAnalysis struct {
	TotalPulls    int            `json:"totalPulls"`
	Avg5StarPulls float64        `json:"avg5StarPulls"`
	Avg4StarPulls float64        `json:"avg4StarPulls"`
	Items5Star    map[string]int `json:"items5Star"`
	Items4Star    map[string]int `json:"items4Star"`
}

// 分析结果结构体定义，包含玩家ID、各池分析结果和全局统计数据
type AnalysisResult struct {
	Uid         string                  `json:"uid"`
	Pools       map[string]PoolAnalysis `json:"pools"`
	GlobalStats GlobalAnalysis          `json:"globalStats"`
}

// 分析抽卡数据，生成统计结果并保存到本地JSON文件中
func analyseData(targetUid string) {
	fileData, err := os.ReadFile("Datas/gacha_storage.json")
	if err != nil {
		fmt.Println("读取本地数据失败:", err)
		return
	}

	var storage map[string]PlayerData
	json.Unmarshal(fileData, &storage)

	// 如果没有指定UID，默认分析第一个玩家的数据
	if targetUid == "" {
		for uid := range storage {
			targetUid = uid
			break
		}
	}

	if targetUid == "" || storage[targetUid].Uid == "" {
		fmt.Println("未找到任何玩家数据")
		return
	}

	playerData := storage[targetUid]

	poolNames := map[string]string{
		"1": "up角色池",
		"2": "up武器池",
		"3": "常驻角色池",
		"4": "常驻武器池",
		"5": "新手池",
		"6": "定向常驻池",
		"7": "感恩庆典池",
		"8": "新手自选池",
	}

	result := AnalysisResult{
		Uid:   targetUid,
		Pools: make(map[string]PoolAnalysis),
		GlobalStats: GlobalAnalysis{
			Items5Star: make(map[string]int),
			Items4Star: make(map[string]int),
		},
	}

	globalTotal5Pulls := 0
	globalCount5 := 0
	globalTotal4Pulls := 0
	globalCount4 := 0

	fmt.Printf("========== 玩家 %s 抽卡分析报告 ==========\n", targetUid)

	for i := 1; i <= 8; i++ {
		poolKey := fmt.Sprintf("%d", i)
		poolData := playerData.GachaData[poolKey]
		poolName := poolNames[poolKey]

		if len(poolData) == 0 {
			continue
		}

		sort.Slice(poolData, func(a, b int) bool {
			return poolData[a].Time < poolData[b].Time
		})

		current5Pity := 0
		current4Pity := 0
		total5Pulls := 0
		count5 := 0
		total4Pulls := 0
		count4 := 0

		var history []PullRecord

		for _, item := range poolData {
			result.GlobalStats.TotalPulls++
			current5Pity++
			current4Pity++

			switch item.QualityLevel {
			case 5:
				result.GlobalStats.Items5Star[item.Name]++
				total5Pulls += current5Pity
				count5++

				history = append(history, PullRecord{
					Name:  item.Name,
					Pulls: current5Pity,
					Time:  item.Time,
				})

				current5Pity = 0
				current4Pity = 0
			case 4:
				result.GlobalStats.Items4Star[item.Name]++
				total4Pulls += current4Pity
				count4++
				current4Pity = 0
			}
		}

		globalTotal5Pulls += total5Pulls
		globalCount5 += count5
		globalTotal4Pulls += total4Pulls
		globalCount4 += count4

		poolAnalysis := PoolAnalysis{
			PoolName:      poolName,
			TotalPulls:    len(poolData),
			CurrentPity:   current5Pity,
			FiveStarCount: count5,
			FourStarCount: count4,
			History5Star:  history,
		}

		if count5 > 0 {
			poolAnalysis.Avg5StarPulls = float64(total5Pulls) / float64(count5)
		}
		if count4 > 0 {
			poolAnalysis.Avg4StarPulls = float64(total4Pulls) / float64(count4)
		}

		result.Pools[poolKey] = poolAnalysis

		fmt.Printf("\n--- 【%s】 ---\n", poolName)
		fmt.Printf("总计花费: %d 抽\n", poolAnalysis.TotalPulls)

		for _, record := range history {
			fmt.Printf("  -> 经过 %2d 抽 获得五星: %s (%s)\n", record.Pulls, record.Name, record.Time)
		}

		fmt.Printf("  -> 当前已垫水位: %d 抽\n", poolAnalysis.CurrentPity)

		if count5 > 0 {
			fmt.Printf("五星平均出金: %.2f 抽 (共 %d 个)\n", poolAnalysis.Avg5StarPulls, count5)
		}
		if count4 > 0 {
			fmt.Printf("四星平均出紫: %.2f 抽 (共 %d 个)\n", poolAnalysis.Avg4StarPulls, count4)
		}
	}

	if globalCount5 > 0 {
		result.GlobalStats.Avg5StarPulls = float64(globalTotal5Pulls) / float64(globalCount5)
	}
	if globalCount4 > 0 {
		result.GlobalStats.Avg4StarPulls = float64(globalTotal4Pulls) / float64(globalCount4)
	}

	fmt.Printf("\n========== 跨池全局统计 ==========\n")
	fmt.Printf("全池累计花费: %d 抽\n", result.GlobalStats.TotalPulls)

	if globalCount5 > 0 {
		fmt.Printf("全局五星平均出金: %.2f 抽\n", result.GlobalStats.Avg5StarPulls)
	} else {
		fmt.Printf("全局五星平均出金: 暂无\n")
	}

	if globalCount4 > 0 {
		fmt.Printf("全局四星平均出紫: %.2f 抽\n", result.GlobalStats.Avg4StarPulls)
	} else {
		fmt.Printf("全局四星平均出紫: 暂无\n")
	}

	fmt.Printf("\n[获取五星物品汇总]\n")
	for name, count := range result.GlobalStats.Items5Star {
		fmt.Printf("%s : %d 个\n", name, count)
	}

	fmt.Printf("\n[获取四星物品汇总]\n")
	for name, count := range result.GlobalStats.Items4Star {
		fmt.Printf("%s : %d 个\n", name, count)
	}
	fmt.Printf("==========================================\n")

	finalJson, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile("Datas/analysis_result.json", finalJson, 0644)
}

//go:embed result.html
var staticFiles embed.FS

// 启动本地HTTP服务器，提供分析结果的可视化界面，并自动打开浏览器访问
func serveAndOpen() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		htmlContent, err := staticFiles.ReadFile("result.html")
		if err != nil {
			http.Error(w, "无法读取内置的 HTML 文件", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(htmlContent)
	})

	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, "./Datas/analysis_result.json")
	})

	fmt.Println("正在启动本地可视化服务...")
	fmt.Println("如果浏览器没有自动打开，请手动访问: http://localhost:8080")

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowserUrl("http://localhost:8080")
	}()

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("服务启动失败:", err)
	}
}

// 打开默认浏览器访问指定URL
func openBrowserUrl(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", "", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
