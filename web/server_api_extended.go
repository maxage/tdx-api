package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/injoyai/tdx/protocol"
)

// 扩展API接口

// 获取股票代码列表
func handleGetCodes(w http.ResponseWriter, r *http.Request) {
	exchange := r.URL.Query().Get("exchange")

	type CodesResponse struct {
		Total     int                 `json:"total"`
		Exchanges map[string]int      `json:"exchanges"`
		Codes     []map[string]string `json:"codes"`
	}

	resp := &CodesResponse{
		Exchanges: make(map[string]int),
		Codes:     []map[string]string{},
	}

	// 确定要查询的交易所
	exchanges := []protocol.Exchange{}
	switch strings.ToLower(exchange) {
	case "sh":
		exchanges = []protocol.Exchange{protocol.ExchangeSH}
	case "sz":
		exchanges = []protocol.Exchange{protocol.ExchangeSZ}
	case "bj":
		exchanges = []protocol.Exchange{protocol.ExchangeBJ}
	default: // all
		exchanges = []protocol.Exchange{protocol.ExchangeSH, protocol.ExchangeSZ, protocol.ExchangeBJ}
	}

	// 获取所有交易所的代码
	for _, ex := range exchanges {
		codeResp, err := client.GetCodeAll(ex)
		if err != nil {
			continue
		}

		exName := ""
		switch ex {
		case protocol.ExchangeSH:
			exName = "sh"
		case protocol.ExchangeSZ:
			exName = "sz"
		case protocol.ExchangeBJ:
			exName = "bj"
		}

		count := 0
		for _, v := range codeResp.List {
			// 只返回股票
			if protocol.IsStock(v.Code) {
				resp.Codes = append(resp.Codes, map[string]string{
					"code":     v.Code,
					"name":     v.Name,
					"exchange": exName,
				})
				count++
			}
		}
		resp.Exchanges[exName] = count
		resp.Total += count
	}

	successResponse(w, resp)
}

// 批量获取行情
func handleBatchQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}

	var req struct {
		Codes []string `json:"codes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}

	if len(req.Codes) == 0 {
		errorResponse(w, "股票代码列表不能为空")
		return
	}

	// 限制最多50只
	if len(req.Codes) > 50 {
		errorResponse(w, "一次最多查询50只股票")
		return
	}

	quotes, err := client.GetQuote(req.Codes...)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取行情失败: %v", err))
		return
	}

	successResponse(w, quotes)
}

// 获取历史K线（指定范围，日/周/月K线使用前复权）
func handleGetKlineHistory(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	klineType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")

	if code == "" {
		errorResponse(w, "股票代码不能为空")
		return
	}

	// 解析limit，默认100，最大800
	limit := uint16(100)
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > 800 {
				l = 800
			}
			limit = uint16(l)
		}
	}

	var resp *protocol.KlineResp
	var err error

	switch klineType {
	case "minute1":
		resp, err = client.GetKlineMinute(code, 0, limit)
	case "minute5":
		resp, err = client.GetKline5Minute(code, 0, limit)
	case "minute15":
		resp, err = client.GetKline15Minute(code, 0, limit)
	case "minute30":
		resp, err = client.GetKline30Minute(code, 0, limit)
	case "hour":
		resp, err = client.GetKlineHour(code, 0, limit)
	case "week":
		// 周K线使用前复权
		resp, err = getQfqKlineDay(code)
		if err == nil {
			resp = convertToWeekKline(resp)
			// 限制返回数量
			if len(resp.List) > int(limit) {
				resp.List = resp.List[len(resp.List)-int(limit):]
				resp.Count = limit
			}
		}
	case "month":
		// 月K线使用前复权
		resp, err = getQfqKlineDay(code)
		if err == nil {
			resp = convertToMonthKline(resp)
			// 限制返回数量
			if len(resp.List) > int(limit) {
				resp.List = resp.List[len(resp.List)-int(limit):]
				resp.Count = limit
			}
		}
	case "day":
		fallthrough
	default:
		// 日K线使用前复权
		resp, err = getQfqKlineDay(code)
		if err == nil && len(resp.List) > int(limit) {
			// 只返回最近limit条
			resp.List = resp.List[len(resp.List)-int(limit):]
			resp.Count = limit
		}
	}

	if err != nil {
		errorResponse(w, fmt.Sprintf("获取K线失败: %v", err))
		return
	}

	successResponse(w, resp)
}

// 获取指数数据
func handleGetIndex(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	klineType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")

	if code == "" {
		errorResponse(w, "指数代码不能为空")
		return
	}

	// 解析limit，默认100，最大800
	limit := uint16(100)
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > 800 {
				l = 800
			}
			limit = uint16(l)
		}
	}

	var resp *protocol.KlineResp
	var err error

	// 根据类型选择对应的指数接口
	switch klineType {
	case "minute1":
		resp, err = client.GetIndex(protocol.TypeKlineMinute, code, 0, limit)
	case "minute5":
		resp, err = client.GetIndex(protocol.TypeKline5Minute, code, 0, limit)
	case "minute15":
		resp, err = client.GetIndex(protocol.TypeKline15Minute, code, 0, limit)
	case "minute30":
		resp, err = client.GetIndex(protocol.TypeKline30Minute, code, 0, limit)
	case "hour":
		resp, err = client.GetIndex(protocol.TypeKline60Minute, code, 0, limit)
	case "week":
		resp, err = client.GetIndexWeekAll(code)
		if resp != nil && len(resp.List) > int(limit) {
			resp.List = resp.List[:limit]
			resp.Count = limit
		}
	case "month":
		resp, err = client.GetIndexMonthAll(code)
		if resp != nil && len(resp.List) > int(limit) {
			resp.List = resp.List[:limit]
			resp.Count = limit
		}
	case "day":
		fallthrough
	default:
		resp, err = client.GetIndexDay(code, 0, limit)
	}

	if err != nil {
		errorResponse(w, fmt.Sprintf("获取指数数据失败: %v", err))
		return
	}

	successResponse(w, resp)
}

// 获取市场统计
func handleGetMarketStats(w http.ResponseWriter, r *http.Request) {
	type MarketStats struct {
		SH struct {
			Total int `json:"total"`
			Up    int `json:"up"`
			Down  int `json:"down"`
			Flat  int `json:"flat"`
		} `json:"sh"`
		SZ struct {
			Total int `json:"total"`
			Up    int `json:"up"`
			Down  int `json:"down"`
			Flat  int `json:"flat"`
		} `json:"sz"`
		BJ struct {
			Total int `json:"total"`
			Up    int `json:"up"`
			Down  int `json:"down"`
			Flat  int `json:"flat"`
		} `json:"bj"`
		UpdateTime string `json:"update_time"`
	}

	stats := &MarketStats{}

	// 获取各市场数据
	for _, ex := range []protocol.Exchange{protocol.ExchangeSH, protocol.ExchangeSZ, protocol.ExchangeBJ} {
		codeResp, err := client.GetCodeAll(ex)
		if err != nil {
			continue
		}

		// 统计涨跌情况
		total, up, down, flat := 0, 0, 0, 0
		for _, code := range codeResp.List {
			if !protocol.IsStock(code.Code) {
				continue
			}
			total++

			// 根据LastPrice判断涨跌（简化版）
			if code.LastPrice > 0 {
				up++
			} else if code.LastPrice < 0 {
				down++
			} else {
				flat++
			}
		}

		switch ex {
		case protocol.ExchangeSH:
			stats.SH.Total = total
			stats.SH.Up = up
			stats.SH.Down = down
			stats.SH.Flat = flat
		case protocol.ExchangeSZ:
			stats.SZ.Total = total
			stats.SZ.Up = up
			stats.SZ.Down = down
			stats.SZ.Flat = flat
		case protocol.ExchangeBJ:
			stats.BJ.Total = total
			stats.BJ.Up = up
			stats.BJ.Down = down
			stats.BJ.Flat = flat
		}
	}

	successResponse(w, stats)
}

// 获取服务器状态
func handleGetServerStatus(w http.ResponseWriter, r *http.Request) {
	type ServerStatus struct {
		Status    string `json:"status"`
		Connected bool   `json:"connected"`
		Version   string `json:"version"`
		Uptime    string `json:"uptime"`
	}

	status := &ServerStatus{
		Status:    "running",
		Connected: true,
		Version:   "1.0.0",
		Uptime:    "unknown",
	}

	successResponse(w, status)
}

// 健康检查
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"time":   fmt.Sprintf("%d", 1730617200),
	})
}
