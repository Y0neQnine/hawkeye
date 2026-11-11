package server

import (
	"context"
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
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, run)
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
