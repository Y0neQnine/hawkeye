package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"time"
)

// =====================================================================
// 攻击面持续监控：采集快照 + 资产变更
//
// 每次快照把当前 assets 表"拍平"进 monitor_snapshots（asset_id + 指纹），
// 与上一次成功快照做 diff，把新增/消失/变更写入 monitor_changes。这样无论
// 资产是 ScopeSentry 同步、Agent 采集还是人工录入进来的，监控都能统一感知，
// 不需要侵入各个 Upsert 路径。
// =====================================================================

const monitorSnapshotLockKey int64 = 7337741002

// ErrMonitorSnapshotBusy means another snapshot batch is currently running.
var ErrMonitorSnapshotBusy = errors.New("另一批快照正在执行，请稍后重试")

// MonitorRun 是一次采集快照批次。
type MonitorRun struct {
	ID           int64      `json:"id"`
	Trigger      string     `json:"trigger"` // manual | sync | schedule
	Source       string     `json:"source"`
	Baseline     bool       `json:"baseline"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	DurationMS   int64      `json:"duration_ms"`
	AssetTotal   int        `json:"asset_total"`
	NewCount     int        `json:"new_count"`
	GoneCount    int        `json:"gone_count"`
	ChangedCount int        `json:"changed_count"`
	Error        string     `json:"error,omitempty"`
}

// MonitorChange 是一条资产变更记录。
type MonitorChange struct {
	ID         int64          `json:"id"`
	RunID      int64          `json:"run_id"`
	AssetID    int64          `json:"asset_id"`
	ChangeType string         `json:"change_type"` // new | gone | changed
	AssetType  string         `json:"asset_type"`
	CompanyID  *int64         `json:"company_id,omitempty"`
	Company    string         `json:"company,omitempty"`
	Label      string         `json:"label"`
	Detail     map[string]any `json:"detail,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type MonitorChangeCounts struct {
	New     int `json:"new"`
	Gone    int `json:"gone"`
	Changed int `json:"changed"`
}

type MonitorOverview struct {
	AssetTotal   int                 `json:"asset_total"`
	AssetCounts  map[string]int      `json:"asset_counts"`
	CompanyTotal int                 `json:"company_total"`
	RunsTotal    int                 `json:"runs_total"`
	LatestRun    *MonitorRun         `json:"latest_run,omitempty"`
	Last24h      MonitorChangeCounts `json:"last_24h"`
	Last7d       MonitorChangeCounts `json:"last_7d"`
}

type MonitorTrendPoint struct {
	Date    string `json:"date"`
	New     int    `json:"new"`
	Gone    int    `json:"gone"`
	Changed int    `json:"changed"`
}

type MonitorChangeFilter struct {
	RunID      int64
	CompanyID  int64
	ChangeType string
	AssetType  string
	Query      string
	Limit      int
	Offset     int
}

// MonitorStore operates on monitor_runs / monitor_snapshots / monitor_changes.
type MonitorStore struct{ db *DB }

func (d *DB) Monitor() *MonitorStore { return &MonitorStore{db: d} }

const monitorFingerprintSQL = `md5(concat_ws('|',
		type,
		coalesce(domain,''), coalesce(root_domain,''), coalesce(ip,''), coalesce(c_segment::text,''),
		coalesce(port::text,''), coalesce(icp,''),
		coalesce(bound_domains::text,''), coalesce(open_ports::text,''),
		coalesce(record_type,''), coalesce(record_value::text,''),
		coalesce(bundle_id,''), coalesce(app_name,''), coalesce(category,''),
		coalesce(app_description,''), coalesce(app_icp,''),
		coalesce(url,''), coalesce(evidence_url,''), coalesce(service_type,''), coalesce(service_name,''), coalesce(favicon_mmh3,''),
		coalesce(status_code::text,''), coalesce(content_length::text,''), coalesce(page_title,''),
		coalesce(technologies::text,''), coalesce(method,''), coalesce(params::text,''),
		coalesce(company_id::text,'')))`

const monitorLabelSQL = `coalesce(nullif(url,''), nullif(domain,''), nullif(app_name,''), nullif(ip,''), nullif(service_name,''), id::text)`

// monitorAttrsSQL 抽出参与变更对比的稳定属性；变更记录里用它算字段级 diff，
// 避免只看到"整行指纹变了"却不知道变的是什么。
const monitorAttrsSQL = `jsonb_build_object(
		'domain', coalesce(domain,''),
		'root_domain', coalesce(root_domain,''),
		'ip', coalesce(ip,''),
		'port', coalesce(port,0),
		'url', coalesce(url,''),
		'evidence_url', coalesce(evidence_url,''),
		'service_name', coalesce(service_name,''),
		'service_type', coalesce(service_type,''),
		'status_code', coalesce(status_code,0),
		'content_length', coalesce(content_length,0),
		'page_title', coalesce(page_title,''),
		'technologies', coalesce(to_jsonb(technologies),'[]'::jsonb),
		'favicon_mmh3', coalesce(favicon_mmh3,''),
		'open_ports', coalesce(to_jsonb(open_ports),'[]'::jsonb),
		'record_type', coalesce(record_type,''),
		'record_value', coalesce(to_jsonb(record_value),'[]'::jsonb),
		'bundle_id', coalesce(bundle_id,''),
		'app_name', coalesce(app_name,''),
		'category', coalesce(category,''),
		'icp', coalesce(icp,''),
		'company_id', coalesce(company_id,0))`

// CreateSnapshot records the current asset inventory and diffs it against the
// previous successful run. The first run is a baseline: it stores the snapshot
// but reports no changes, so a fresh install is not flooded with "new" rows.
func (s *MonitorStore) CreateSnapshot(ctx context.Context, trigger, source string) (*MonitorRun, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("monitor: database unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize snapshots so two concurrent runs cannot both diff against the
	// same predecessor and produce duplicated change rows. Peek first without
	// blocking so a routine tick never piles up behind a long-running snapshot.
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock($1)`, monitorSnapshotLockKey).Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrMonitorSnapshotBusy
	}

	var prev sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id FROM monitor_runs
		WHERE finished_at IS NOT NULL AND error = ''
		ORDER BY id DESC LIMIT 1`).Scan(&prev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var runID int64
	baseline := !prev.Valid
	if err := tx.QueryRowContext(ctx, `INSERT INTO monitor_runs (trigger_kind, source, baseline)
		VALUES ($1, $2, $3) RETURNING id`, trigger, source, baseline).Scan(&runID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO monitor_snapshots
		(run_id, asset_id, fingerprint, asset_type, company_id, label, attrs)
		SELECT $1, a.id, `+monitorFingerprintSQL+`, a.type, a.company_id, `+monitorLabelSQL+`, `+monitorAttrsSQL+`
		FROM assets a`, runID); err != nil {
		return nil, err
	}

	if prev.Valid {
		prevID := prev.Int64
		if _, err := tx.ExecContext(ctx, `INSERT INTO monitor_changes
			(run_id, asset_id, change_type, asset_type, company_id, label, detail)
			SELECT $1, c.asset_id, 'new', c.asset_type, c.company_id, c.label, '{}'::jsonb
			FROM monitor_snapshots c
			LEFT JOIN monitor_snapshots p ON p.run_id = $2 AND p.asset_id = c.asset_id
			WHERE c.run_id = $1 AND p.asset_id IS NULL`, runID, prevID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO monitor_changes
			(run_id, asset_id, change_type, asset_type, company_id, label, detail)
			SELECT $1, p.asset_id, 'gone', p.asset_type, p.company_id, p.label, '{}'::jsonb
			FROM monitor_snapshots p
			LEFT JOIN monitor_snapshots c ON c.run_id = $1 AND c.asset_id = p.asset_id
			WHERE p.run_id = $2 AND c.asset_id IS NULL`, runID, prevID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO monitor_changes
			(run_id, asset_id, change_type, asset_type, company_id, label, detail)
			SELECT $1, c.asset_id, 'changed', c.asset_type, c.company_id, c.label,
			       jsonb_build_object('before', p.attrs, 'after', c.attrs,
			                          'before_fingerprint', p.fingerprint, 'after_fingerprint', c.fingerprint)
			FROM monitor_snapshots c
			JOIN monitor_snapshots p ON p.run_id = $2 AND p.asset_id = c.asset_id
			WHERE c.run_id = $1 AND p.fingerprint IS DISTINCT FROM c.fingerprint`, runID, prevID); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE monitor_runs r SET
		finished_at = now(),
		duration_ms = GREATEST(0, (EXTRACT(EPOCH FROM (now() - r.started_at)) * 1000)::bigint),
		asset_total = (SELECT count(*) FROM monitor_snapshots WHERE run_id = r.id),
		new_count = (SELECT count(*) FROM monitor_changes WHERE run_id = r.id AND change_type = 'new'),
		gone_count = (SELECT count(*) FROM monitor_changes WHERE run_id = r.id AND change_type = 'gone'),
		changed_count = (SELECT count(*) FROM monitor_changes WHERE run_id = r.id AND change_type = 'changed')
		WHERE r.id = $1`, runID); err != nil {
		return nil, err
	}

	// Keep the monitoring horizon bounded; cascades clean snapshots/changes.
	if _, err := tx.ExecContext(ctx, `DELETE FROM monitor_runs WHERE started_at < now() - interval '90 days'`); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetRun(ctx, runID)
}

// DeleteRun removes a snapshot batch; monitor_snapshots / monitor_changes rows
// cascade with it. Deleting the latest successful run makes the next snapshot
// diff against the one before it (or become a fresh baseline if none is left).
func (s *MonitorStore) DeleteRun(ctx context.Context, id int64) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("monitor: database unavailable")
	}
	if id <= 0 {
		return 0, errors.New("monitor: invalid run id")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitor_runs WHERE id = $1`, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *MonitorStore) GetRun(ctx context.Context, id int64) (*MonitorRun, error) {
	var r MonitorRun
	var finished sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id, trigger_kind, source, baseline, started_at,
		finished_at, duration_ms, asset_total, new_count, gone_count, changed_count, error
		FROM monitor_runs WHERE id = $1`, id).Scan(&r.ID, &r.Trigger, &r.Source, &r.Baseline,
		&r.StartedAt, &finished, &r.DurationMS, &r.AssetTotal, &r.NewCount, &r.GoneCount,
		&r.ChangedCount, &r.Error)
	if err != nil {
		return nil, err
	}
	if finished.Valid {
		t := finished.Time
		r.FinishedAt = &t
	}
	return &r, nil
}

func (s *MonitorStore) LatestRun(ctx context.Context) (*MonitorRun, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM monitor_runs
		WHERE finished_at IS NOT NULL AND error = '' ORDER BY id DESC LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetRun(ctx, id)
}

func (s *MonitorStore) ListRuns(ctx context.Context, limit, offset int) ([]MonitorRun, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM monitor_runs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, trigger_kind, source, baseline, started_at,
		finished_at, duration_ms, asset_total, new_count, gone_count, changed_count, error
		FROM monitor_runs ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]MonitorRun, 0, limit)
	for rows.Next() {
		var r MonitorRun
		var finished sql.NullTime
		if err := rows.Scan(&r.ID, &r.Trigger, &r.Source, &r.Baseline, &r.StartedAt,
			&finished, &r.DurationMS, &r.AssetTotal, &r.NewCount, &r.GoneCount,
			&r.ChangedCount, &r.Error); err != nil {
			return nil, 0, err
		}
		if finished.Valid {
			t := finished.Time
			r.FinishedAt = &t
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *MonitorStore) ListChanges(ctx context.Context, f MonitorChangeFilter) ([]MonitorChange, int, error) {
	if f.RunID <= 0 {
		latest, err := s.LatestRun(ctx)
		if err != nil {
			return nil, 0, err
		}
		if latest == nil {
			return []MonitorChange{}, 0, nil
		}
		f.RunID = latest.ID
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	where := `WHERE c.run_id = $1`
	args := []any{f.RunID}
	add := func(clause string, v any) {
		args = append(args, v)
		where += " AND " + clause
	}
	if f.CompanyID > 0 {
		add("c.company_id = $"+strconv.Itoa(len(args)+1), f.CompanyID)
	}
	if f.ChangeType != "" {
		add("c.change_type = $"+strconv.Itoa(len(args)+1), f.ChangeType)
	}
	if f.AssetType != "" {
		add("c.asset_type = $"+strconv.Itoa(len(args)+1), f.AssetType)
	}
	if f.Query != "" {
		add("c.label ILIKE $"+strconv.Itoa(len(args)+1), "%"+f.Query+"%")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM monitor_changes c `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.run_id, c.asset_id, c.change_type,
		c.asset_type, c.company_id, coalesce(comp.name,''), c.label, c.detail, c.created_at
		FROM monitor_changes c
		LEFT JOIN companies comp ON comp.id = c.company_id
		`+where+` ORDER BY c.id DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]MonitorChange, 0, f.Limit)
	for rows.Next() {
		var c MonitorChange
		var detail []byte
		if err := rows.Scan(&c.ID, &c.RunID, &c.AssetID, &c.ChangeType, &c.AssetType,
			&c.CompanyID, &c.Company, &c.Label, &detail, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		if len(detail) > 0 {
			if err := json.Unmarshal(detail, &c.Detail); err != nil {
				return nil, 0, err
			}
			enrichMonitorDetail(c.Detail)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// enrichMonitorDetail turns the stored before/after attribute snapshots into a
// compact field-level diff so the UI can explain what actually changed.
func enrichMonitorDetail(detail map[string]any) {
	if detail == nil {
		return
	}
	before, ok1 := detail["before"].(map[string]any)
	after, ok2 := detail["after"].(map[string]any)
	if !ok1 || !ok2 {
		return
	}
	seen := make(map[string]struct{}, len(before)+len(after))
	for k := range before {
		seen[k] = struct{}{}
	}
	for k := range after {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	changes := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		if reflect.DeepEqual(before[k], after[k]) {
			continue
		}
		changes = append(changes, map[string]any{"field": k, "before": before[k], "after": after[k]})
	}
	detail["changes"] = changes
}

func (s *MonitorStore) Overview(ctx context.Context) (*MonitorOverview, error) {
	out := &MonitorOverview{AssetCounts: map[string]int{}}
	rows, err := s.db.QueryContext(ctx, `SELECT type, count(*) FROM assets GROUP BY type`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var typ string
		var n int
		if err := rows.Scan(&typ, &n); err != nil {
			rows.Close()
			return nil, err
		}
		out.AssetCounts[typ] = n
		out.AssetTotal += n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = s.db.QueryRowContext(ctx, `SELECT count(*) FROM companies`).Scan(&out.CompanyTotal)
	_ = s.db.QueryRowContext(ctx, `SELECT count(*) FROM monitor_runs`).Scan(&out.RunsTotal)
	if out.LatestRun, err = s.LatestRun(ctx); err != nil {
		return nil, err
	}
	countSince := func(days int) (MonitorChangeCounts, error) {
		var c MonitorChangeCounts
		err := s.db.QueryRowContext(ctx, `SELECT
			count(*) FILTER (WHERE change_type='new'),
			count(*) FILTER (WHERE change_type='gone'),
			count(*) FILTER (WHERE change_type='changed')
			FROM monitor_changes WHERE created_at >= now() - make_interval(days => $1)`, days).
			Scan(&c.New, &c.Gone, &c.Changed)
		return c, err
	}
	if out.Last24h, err = countSince(1); err != nil {
		return nil, err
	}
	if out.Last7d, err = countSince(7); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *MonitorStore) Trend(ctx context.Context, days int) ([]MonitorTrendPoint, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	rows, err := s.db.QueryContext(ctx, `SELECT to_char(d.day, 'YYYY-MM-DD'),
		coalesce(count(*) FILTER (WHERE c.change_type='new'), 0),
		coalesce(count(*) FILTER (WHERE c.change_type='gone'), 0),
		coalesce(count(*) FILTER (WHERE c.change_type='changed'), 0)
		FROM generate_series(current_date - make_interval(days => $1 - 1), current_date, interval '1 day') AS d(day)
		LEFT JOIN monitor_changes c ON c.created_at >= d.day AND c.created_at < d.day + interval '1 day'
		GROUP BY d.day ORDER BY d.day`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MonitorTrendPoint, 0, days)
	for rows.Next() {
		var p MonitorTrendPoint
		if err := rows.Scan(&p.Date, &p.New, &p.Gone, &p.Changed); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
