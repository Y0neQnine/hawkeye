package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
)

// 攻击面持续监控 HTTP 接口。快照与 diff 的存储逻辑在 db.MonitorStore。

func (s *Server) monitorStore() *db.MonitorStore {
	if s.m.pg == nil {
		return nil
	}
	return s.m.pg.Monitor()
}

func (s *Server) monitorOverview(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	out, err := ms.Overview(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) monitorRuns(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	runs, total, err := ms.ListRuns(r.Context(), limit, offset)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"runs": runs, "total": total})
}

func (s *Server) monitorDeleteRun(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "无效的快照批次")
		return
	}
	deleted, err := ms.DeleteRun(r.Context(), id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": deleted})
}

func (s *Server) monitorChanges(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	q := r.URL.Query()
	runID, _ := strconv.ParseInt(q.Get("run_id"), 10, 64)
	companyID, _ := strconv.ParseInt(q.Get("company_id"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	changes, total, err := ms.ListChanges(r.Context(), db.MonitorChangeFilter{
		RunID:      runID,
		CompanyID:  companyID,
		ChangeType: strings.TrimSpace(q.Get("change_type")),
		AssetType:  strings.TrimSpace(q.Get("asset_type")),
		Query:      strings.TrimSpace(q.Get("q")),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"changes": changes, "total": total})
}

func (s *Server) monitorTrend(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	points, err := ms.Trend(r.Context(), days)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"points": points})
}

func (s *Server) monitorSnapshot(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	run, err := ms.CreateSnapshot(ctx, "manual", "manual")
	if err != nil {
		if errors.Is(err, db.ErrMonitorSnapshotBusy) {
			writeErr(w, 409, err.Error())
			return
		}
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, run)
}

// ---- 自动监控：全局开关 / 监控目标 ----

type monitorAutoReq struct {
	Enabled *bool `json:"enabled"`
}

type monitorTargetReq struct {
	CompanyID       int64  `json:"company_id"`
	RootDomain      string `json:"root_domain"`
	IntervalMinutes int    `json:"interval_minutes"`
	AssetsOnly      *bool  `json:"assets_only"`
	Enabled         *bool  `json:"enabled"`
}

func (s *Server) monitorAutoGet(w http.ResponseWriter, r *http.Request) {
	pg := s.m.pg
	if pg == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": pg.GetBool(db.MonitorAutoSettingKey, false)})
}

func (s *Server) monitorAutoSet(w http.ResponseWriter, r *http.Request) {
	pg := s.m.pg
	if pg == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	var req monitorAutoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeErr(w, 400, "缺少 enabled")
		return
	}
	if err := pg.SetBool(db.MonitorAutoSettingKey, *req.Enabled); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": *req.Enabled})
}

func writeMonitorTargetErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrMonitorTargetNotFound):
		writeErr(w, 404, err.Error())
	case errors.Is(err, db.ErrMonitorTargetInvalid),
		errors.Is(err, db.ErrMonitorTargetDomainRequired),
		errors.Is(err, db.ErrMonitorTargetInterval),
		errors.Is(err, db.ErrMonitorTargetCompanyNotFound):
		writeErr(w, 400, err.Error())
	default:
		writeErr(w, 500, err.Error())
	}
}

func (s *Server) monitorTargets(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	targets, err := ms.ListTargets(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"targets": targets,
		"enabled": s.m.pg.GetBool(db.MonitorAutoSettingKey, false),
	})
}

func (s *Server) monitorCreateTarget(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	var req monitorTargetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	assetsOnly, enabled := true, true
	if req.AssetsOnly != nil {
		assetsOnly = *req.AssetsOnly
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	tgt, err := ms.UpsertTarget(r.Context(), db.MonitorTargetInput{
		CompanyID:       req.CompanyID,
		RootDomain:      req.RootDomain,
		IntervalMinutes: req.IntervalMinutes,
		AssetsOnly:      assetsOnly,
		Enabled:         enabled,
	})
	if err != nil {
		writeMonitorTargetErr(w, err)
		return
	}
	writeJSON(w, 200, tgt)
}

func (s *Server) monitorUpdateTarget(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "无效的监控目标")
		return
	}
	var patch db.MonitorTargetPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	tgt, err := ms.UpdateTarget(r.Context(), id, patch)
	if err != nil {
		writeMonitorTargetErr(w, err)
		return
	}
	writeJSON(w, 200, tgt)
}

func (s *Server) monitorDeleteTarget(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "无效的监控目标")
		return
	}
	deleted, err := ms.DeleteTarget(r.Context(), id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": deleted})
}

func (s *Server) monitorRunTarget(w http.ResponseWriter, r *http.Request) {
	ms := s.monitorStore()
	if ms == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "无效的监控目标")
		return
	}
	tgt, err := ms.GetTarget(r.Context(), id)
	if err != nil {
		writeMonitorTargetErr(w, err)
		return
	}
	if tgt.LastTaskID != nil && !s.monitorTaskDone(*tgt.LastTaskID) {
		writeErr(w, 409, "该目标上一轮监控仍在执行，请等待完成")
		return
	}
	task, err := s.launchMonitorRun(tgt)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"task_id": task.ID})
}

// launchMonitorRun creates and starts the incremental collection task for one
// target. New assets are attributed to the target company via task scope and by
// root-domain company resolution on the asset write path.
func (s *Server) launchMonitorRun(tgt *db.MonitorTarget) (*Task, error) {
	company := strings.TrimSpace(tgt.CompanyName)
	if company == "" {
		company = fmt.Sprintf("#%d", tgt.CompanyID)
	}
	desc := fmt.Sprintf("持续监控 · %s · %s", company, tgt.RootDomain)
	goal := monitorGoalText(tgt.RootDomain, company)
	assetsOnly := tgt.AssetsOnly
	task, err := s.m.CreateTaskWithOptions(desc, goal, db.TaskCreateOptions{
		Name:       "持续监控 · " + tgt.RootDomain,
		CompanyIDs: []int64{tgt.CompanyID},
		AssetsOnly: &assetsOnly,
	})
	if err != nil {
		return nil, err
	}
	taskID, _ := strconv.ParseInt(task.ID, 10, 64)
	if err := s.m.pg.Monitor().MarkTargetLaunched(context.Background(), tgt.ID, taskID); err != nil {
		log.Printf("[monitor] 目标 #%d 记录任务失败: %v", tgt.ID, err)
	}
	s.launchTask(task, desc+" "+goal, false)
	return task, nil
}

func (s *Server) monitorTaskDone(id int64) bool {
	task, ok := s.m.Task(strconv.FormatInt(id, 10))
	if !ok {
		return true // deleted/archived
	}
	lifecycle := task.lifecycleSnapshot()
	return !lifecycle.Queued && isTerminalStatus(lifecycle.Status)
}

func monitorGoalText(rootDomain, company string) string {
	return fmt.Sprintf(`对%s名下的根域名 %s 做一轮增量采集（持续监控）：
1. 先用 list_assets 读取该企业/该根域名的已有资产作为基线，只补新发现或属性缺失的部分，已有资产不重复登记。
2. 用证书日志（crt.sh）、被动 DNS、测绘 API（recon_search，如已配置）发现新增子域/主机，并对候选子域做解析校验。
3. 对尚未登记的主机补端口与服务；站点以 service 类型登记 URL，系统会自动只读探测并识别系统/技术栈。
4. 只做只读采集，不越出该根域名的授权范围；不重复扫描、不高并发。
5. 新资产按根域名归属自动挂到「%s」下；在 record_fact 中说明本轮新增了什么、依据是什么。`, company, rootDomain, company)
}

// snapshotMonitor records a snapshot without failing the caller: monitoring is
// auxiliary and must never break an asset sync.
func (s *Server) snapshotMonitor(ctx context.Context, trigger, source string) {
	ms := s.monitorStore()
	if ms == nil {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	run, err := ms.CreateSnapshot(cctx, trigger, source)
	if err != nil {
		log.Printf("[monitor] snapshot (%s/%s) failed: %v", trigger, source, err)
		return
	}
	log.Printf("[monitor] snapshot #%d (%s/%s): total=%d new=%d gone=%d changed=%d",
		run.ID, trigger, source, run.AssetTotal, run.NewCount, run.GoneCount, run.ChangedCount)
}

// runMonitorScheduler takes a periodic inventory snapshot. Collection itself can
// happen through ScopeSentry sync or Agent tasks; the snapshot turns whatever is
// currently known into a comparable baseline and computes the diff.
func (s *Server) runMonitorScheduler(ctx context.Context) {
	min := 360
	if raw := strings.TrimSpace(os.Getenv("HAWKEYE_MONITOR_INTERVAL_MIN")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			min = n
		}
	}
	if min <= 0 {
		log.Printf("[monitor] periodic snapshots disabled")
		return
	}
	ticker := time.NewTicker(time.Duration(min) * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.snapshotMonitor(ctx, "schedule", "")
		}
	}
}
