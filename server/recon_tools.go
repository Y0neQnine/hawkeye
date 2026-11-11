package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	actool "github.com/Autumn-27/norma/tool"
)

// 本文件实现「测绘引擎查询」host 工具：把 FOFA / Quake / Hunter / Shodan 统一成
// 一个 recon_search 入口。凭据存在 settings 表（系统配置 → 测绘 API），由服务端
// 代发请求——Agent 只看到查询结果，Key 不会出现在提示词、Bash 命令或日志里。
// 工具行 seed 进 tools 表，按 agent 绑定可见；默认给采集类 agent。

const reconToolKey = "recon_search"

const reconToolDesc = "查询已配置的互联网测绘引擎（FOFA / Quake / Hunter / Shodan），用于补充域名、子域名、IP、端口、服务与站点标题等攻击面资产。\n" +
	"仅做资产发现，不执行漏洞验证或攻击。查询会消耗对应平台的配额。\n" +
	"凭据在 系统配置 → 测绘 API 中配置；未配置的 provider 调用会返回明确提示。\n" +
	"结果需人工/Agent 判断后通过 insert_assets 等资产工具写回资产图，不要臆造字段。"

// reconToolAgents 是 recon_search 的默认 agent 绑定。可在「工具」页按需增删。
var reconToolAgents = []string{"worker", "planner", "mainagent", "pentest", "auto"}

func reconToolSchema() map[string]any {
	return objSchema(map[string]any{
		"provider": map[string]any{
			"type":        "string",
			"enum":        []string{"fofa", "quake", "hunter", "shodan"},
			"description": "测绘引擎：fofa / quake / hunter / shodan",
		},
		"query": strParam("引擎对应的查询语法，例如 fofa: domain=\"example.com\"；quake: domain: \"example.com\"；hunter: domain.suffix=\"example.com\"；shodan: hostname:example.com"),
		"page":  map[string]any{"type": "integer", "description": "页码，从 1 开始（默认 1）", "default": 1},
		"size": map[string]any{
			"type":        "integer",
			"description": "每页条数，1-100（默认 20）。Shodan 固定每页 100 条，此参数不生效",
			"default":     20,
		},
		"fields": strParam("可选，仅 FOFA 生效：返回字段，逗号分隔（默认 host,ip,port,domain,title,server）"),
	}, "provider", "query")
}

// reconTools 返回给 hostTools() 注入的测绘工具。
func (s *Server) reconTools() []actool.CoreTool {
	return []actool.CoreTool{s.toolReconSearch()}
}

// seedReconTools 把 recon_search 播种进 tools 表（仅首次插入，保留用户在「工具」页的改配）。
func (s *Server) seedReconTools() {
	schema, _ := json.Marshal(reconToolSchema())
	agents, _ := json.Marshal(reconToolAgents)
	if err := s.m.pg.SeedTool(reconToolKey, reconToolDesc, schema, agents); err != nil {
		log.Printf("[recon] seed %s 失败: %v", reconToolKey, err)
	}
}

func (s *Server) toolReconSearch() actool.CoreTool {
	return wrTool(reconToolKey, reconToolDesc, reconToolSchema(), s.runReconSearch)
}

func (s *Server) runReconSearch(ctx context.Context, in json.RawMessage) (actool.Result, error) {
	var a struct {
		Provider string `json:"provider"`
		Query    string `json:"query"`
		Page     int    `json:"page"`
		Size     int    `json:"size"`
		Fields   string `json:"fields"`
	}
	_ = json.Unmarshal(in, &a)
	provider := strings.ToLower(strings.TrimSpace(a.Provider))
	query := strings.TrimSpace(a.Query)
	if query == "" {
		return actool.Errorf("query 不能为空"), nil
	}
	if a.Page < 1 {
		a.Page = 1
	}
	if a.Size < 1 {
		a.Size = 20
	}
	if a.Size > 100 {
		a.Size = 100
	}

	email, fofaKey, quakeKey, hunterKey, shodanKey := s.m.ReconConfig()
	var (
		items any
		total int64
		note  string
		err   error
	)
	switch provider {
	case "fofa":
		items, total, note, err = reconFOFA(ctx, email, fofaKey, query, a.Fields, a.Page, a.Size)
	case "quake":
		items, total, note, err = reconQuake(ctx, quakeKey, query, a.Page, a.Size)
	case "hunter":
		items, total, note, err = reconHunter(ctx, hunterKey, query, a.Page, a.Size)
	case "shodan":
		items, total, note, err = reconShodan(ctx, shodanKey, query, a.Page)
	default:
		return actool.Errorf("provider 需为 fofa / quake / hunter / shodan"), nil
	}
	if err != nil {
		return actool.Errorf(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"provider": provider,
		"query":    query,
		"page":     a.Page,
		"size":     a.Size,
		"total":    total,
		"items":    items,
		"note":     note,
	})
}

var reconHTTPClient = &http.Client{Timeout: 30 * time.Second}

const reconMaxBody = 8 << 20 // 8 MiB，防止异常响应撑爆上下文

func reconFOFA(ctx context.Context, email, key, query, fields string, page, size int) (any, int64, string, error) {
	if strings.TrimSpace(email) == "" || strings.TrimSpace(key) == "" {
		return nil, 0, "", errors.New("FOFA 未配置：请在 系统配置 → 测绘 API 填写 FOFA 邮箱与 API Key")
	}
	if strings.TrimSpace(fields) == "" {
		fields = "host,ip,port,domain,title,server"
	}
	q := url.Values{}
	q.Set("email", email)
	q.Set("key", key)
	q.Set("qbase64", base64.StdEncoding.EncodeToString([]byte(query)))
	q.Set("page", strconv.Itoa(page))
	q.Set("size", strconv.Itoa(size))
	q.Set("fields", fields)
	body, err := reconGet(ctx, "https://fofa.info/api/v1/search/all?"+q.Encode())
	if err != nil {
		return nil, 0, "", err
	}
	var r struct {
		Error   bool   `json:"error"`
		Errmsg  string `json:"errmsg"`
		Total   int64  `json:"total"`
		Results any    `json:"results"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, 0, "", fmt.Errorf("FOFA 响应解析失败: %w", err)
	}
	if r.Error {
		return nil, 0, "", fmt.Errorf("FOFA 查询失败: %s", strings.TrimSpace(r.Errmsg))
	}
	return r.Results, r.Total, "返回字段: " + fields, nil
}

func reconQuake(ctx context.Context, key, query string, page, size int) (any, int64, string, error) {
	if strings.TrimSpace(key) == "" {
		return nil, 0, "", errors.New("Quake 未配置：请在 系统配置 → 测绘 API 填写 Quake API Key")
	}
	payload, _ := json.Marshal(map[string]any{
		"query":        query,
		"start":        (page - 1) * size,
		"size":         size,
		"ignore_cache": true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://quake.360.net/api/v3/search/quake_service", bytes.NewReader(payload))
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-QuakeToken", key)
	body, err := reconDo(req)
	if err != nil {
		return nil, 0, "", err
	}
	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
		Meta    struct {
			Pagination struct {
				Total int64 `json:"total"`
			} `json:"pagination"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, 0, "", fmt.Errorf("Quake 响应解析失败: %w", err)
	}
	if r.Code != 0 {
		return nil, 0, "", fmt.Errorf("Quake 查询失败(code=%d): %s", r.Code, strings.TrimSpace(r.Message))
	}
	return r.Data, r.Meta.Pagination.Total, "start=(page-1)*size，按 size 分页", nil
}

func reconHunter(ctx context.Context, key, query string, page, size int) (any, int64, string, error) {
	if strings.TrimSpace(key) == "" {
		return nil, 0, "", errors.New("Hunter 未配置：请在 系统配置 → 测绘 API 填写 Hunter API Key")
	}
	q := url.Values{}
	q.Set("api-key", key)
	q.Set("search", base64.StdEncoding.EncodeToString([]byte(query)))
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(size))
	q.Set("is_web", "3")
	body, err := reconGet(ctx, "https://hunter.qianxin.com/openApi/search?"+q.Encode())
	if err != nil {
		return nil, 0, "", err
	}
	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total int64 `json:"total"`
			Arr   any   `json:"arr"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, 0, "", fmt.Errorf("Hunter 响应解析失败: %w", err)
	}
	if r.Code != 200 {
		return nil, 0, "", fmt.Errorf("Hunter 查询失败(code=%d): %s", r.Code, strings.TrimSpace(r.Message))
	}
	return r.Data.Arr, r.Data.Total, "is_web=3（同时检索 Web 与非 Web 资产）", nil
}

func reconShodan(ctx context.Context, key, query string, page int) (any, int64, string, error) {
	if strings.TrimSpace(key) == "" {
		return nil, 0, "", errors.New("Shodan 未配置：请在 系统配置 → 测绘 API 填写 Shodan API Key")
	}
	q := url.Values{}
	q.Set("key", key)
	q.Set("query", query)
	q.Set("page", strconv.Itoa(page))
	body, err := reconGet(ctx, "https://api.shodan.io/shodan/host/search?"+q.Encode())
	if err != nil {
		return nil, 0, "", err
	}
	var r struct {
		Total   int64  `json:"total"`
		Matches any    `json:"matches"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, 0, "", fmt.Errorf("Shodan 响应解析失败: %w", err)
	}
	if strings.TrimSpace(r.Error) != "" {
		return nil, 0, "", fmt.Errorf("Shodan 查询失败: %s", strings.TrimSpace(r.Error))
	}
	return r.Matches, r.Total, "Shodan 每页固定 100 条，size 参数不生效", nil
}

func reconGet(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return reconDo(req)
}

func reconDo(req *http.Request) ([]byte, error) {
	resp, err := reconHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", req.URL.Host, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, reconMaxBody))
	if err != nil {
		return nil, fmt.Errorf("读取 %s 响应失败: %w", req.URL.Host, err)
	}
	if resp.StatusCode >= 400 {
		// 不回显请求 URL（含 Key），只截取响应体帮助定位配额/鉴权问题。
		msg := strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return nil, fmt.Errorf("%s 返回 HTTP %d: %s", req.URL.Host, resp.StatusCode, msg)
	}
	return body, nil
}
