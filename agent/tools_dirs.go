package agent

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/fingerprint"
	actool "github.com/Autumn-27/norma/tool"
)

// maxDirRecordedFindings caps how many discovered paths one probe_directories
// call registers as service assets. The model still sees every finding.
const maxDirRecordedFindings = 25

// probeDirectories lets an agent run a bounded directory scan against a site that
// only serves a default page, so the real backend entry (admin/login/console) or
// a fingerprint-revealing path (swagger/actuator/druid/.git) surfaces as assets.
func (t *ToolSet) probeDirectories() actool.CoreTool {
	return writeTool(
		"probe_directories",
		"对站点做【目录探测】：按内置的常见后台/控制台/敏感文件路径表逐条 GET（只读、同主机、限速、软 404 过滤），用于站点只返回默认页/空页/默认 404、看不到真实系统时，找出后台入口（admin/login/manage）或能表明指纹的目录（swagger、actuator、druid、nacos、phpmyadmin、.git、.env 等）。\n"+
			"命中（非软 404 的 2xx/3xx/401/403/405）会作为 HTTP 服务资产登记到当前任务（自动带上企业归属），并把识别到的技术栈回写到被探测站点；返回 {findings:[{kind,path,url,status_code,page_title,technologies}], recorded_assets, scanned}。\n"+
			"用法：url 传站点根地址（如 https://example.com）；paths 可补充自定义路径（会先于内置表探测）；max_paths 默认 150、上限 300。同一主机不要反复调用；任务为『仅资产』采集模式时不可用。",
		obj(map[string]any{
			"url":       str("起始站点 URL（http/https），如 https://example.com"),
			"paths":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "额外探测的路径（可选，不必带前导 /），会优先于内置路径表"},
			"max_paths": intp("本次最多探测的路径数，默认 150，上限 300"),
		}, "url"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			if t.assetsOnly {
				return actool.Errorf("当前任务为『仅资产』采集模式，禁止目录探测/路径请求"), nil
			}
			if t.as != nil && t.taskID > 0 && t.as.AssetsOnly(t.taskID) {
				return actool.Errorf("当前任务为『仅资产』采集模式，禁止目录探测/路径请求"), nil
			}
			if t.enrich == nil {
				return actool.Errorf("probe_directories 未启用：探测引擎未初始化"), nil
			}
			var a struct {
				URL      string   `json:"url"`
				Paths    []string `json:"paths"`
				MaxPaths int      `json:"max_paths"`
			}
			_ = json.Unmarshal(in, &a)
			raw := strings.TrimSpace(a.URL)
			if raw == "" {
				return actool.Errorf("url 不能为空"), nil
			}
			if u, err := url.Parse(raw); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return actool.Errorf("无效的站点 URL：" + raw), nil
			}
			res, err := t.enrich.ScanDirectories(ctx, raw, a.Paths, a.MaxPaths)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			recorded := 0
			if t.as != nil {
				recorded = t.recordDirFindings(raw, res)
			}
			return jsonResult(map[string]any{
				"base_url":        res.BaseURL,
				"scanned":         res.Scanned,
				"findings":        res.Findings,
				"technologies":    res.Technologies,
				"recorded_assets": recorded,
				"truncated":       res.Truncated,
				"aborted":         res.Aborted,
			})
		},
	)
}

// recordDirFindings registers discovered paths as HTTP service assets attributed to
// this run's task, anchors them to the owner node, and merges the discovered
// technologies back onto the probed site.
func (t *ToolSet) recordDirFindings(parentURL string, res *fingerprint.DirScanResult) int {
	if t.as == nil || res == nil {
		return 0
	}
	added := 0
	for i, f := range res.Findings {
		if i >= maxDirRecordedFindings {
			break
		}
		status := f.StatusCode
		length := f.ContentLength
		id, err := t.as.UpsertHTTPService(db.UpsertHTTPServiceReq{
			URL:           f.URL,
			Technologies:  f.Technologies,
			StatusCode:    &status,
			ContentLength: &length,
			PageTitle:     f.PageTitle,
			TaskID:        t.taskID,
		})
		if err != nil {
			continue
		}
		added++
		t.anchorOwner(id)
		if t.taskID > 0 {
			var sourceNodeID *int64
			if t.ownerNode > 0 {
				nodeID := t.ownerNode
				sourceNodeID = &nodeID
			}
			_ = t.as.SetTaskAssetSource(t.taskID, id, "agent", "目录探测命中", sourceNodeID)
		}
	}
	if len(res.Technologies) > 0 {
		_, _ = t.as.UpsertHTTPService(db.UpsertHTTPServiceReq{
			URL:          parentURL,
			Technologies: res.Technologies,
			TaskID:       t.taskID,
		})
	}
	return added
}
