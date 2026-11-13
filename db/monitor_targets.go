package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// 自动监控目标：企业 + 根域名 + 周期。调度器到期后发起一轮增量采集任务，
// 任务结束后自动快照并 diff。全局开关见 settings.monitor.auto_enabled。

var (
	ErrMonitorTargetInvalid         = errors.New("监控目标参数无效")
	ErrMonitorTargetDomainRequired  = errors.New("请填写要监控的根域名")
	ErrMonitorTargetCompanyNotFound = errors.New("关联企业不存在")
	ErrMonitorTargetNotFound        = errors.New("监控目标不存在")
	ErrMonitorTargetInterval        = errors.New("监控周期需在 15 分钟到 30 天之间")
)

const (
	MonitorAutoSettingKey = "monitor.auto_enabled"
	monitorMinInterval    = 15
	monitorMaxInterval    = 30 * 24 * 60
	monitorDefaultMinutes = 24 * 60
)

// MonitorTarget is one company/root-domain pair watched on a schedule.
type MonitorTarget struct {
	ID              int64      `json:"id"`
	CompanyID       int64      `json:"company_id"`
	CompanyName     string     `json:"company_name,omitempty"`
	RootDomain      string     `json:"root_domain"`
	IntervalMinutes int        `json:"interval_minutes"`
	AssetsOnly      bool       `json:"assets_only"`
	Enabled         bool       `json:"enabled"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	LastTaskID      *int64     `json:"last_task_id,omitempty"`
	LastSnapshotAt  *time.Time `json:"last_snapshot_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// MonitorTargetInput creates or updates a target (unique by company + root domain).
type MonitorTargetInput struct {
	CompanyID       int64  `json:"company_id"`
	RootDomain      string `json:"root_domain"`
	IntervalMinutes int    `json:"interval_minutes"`
	AssetsOnly      bool   `json:"assets_only"`
	Enabled         bool   `json:"enabled"`
}

// MonitorTargetPatch edits a target; nil fields stay unchanged.
type MonitorTargetPatch struct {
	IntervalMinutes *int  `json:"interval_minutes"`
	AssetsOnly      *bool `json:"assets_only"`
	Enabled         *bool `json:"enabled"`
}

func normalizeMonitorDomain(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrMonitorTargetDomainRequired
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil || u.Hostname() == "" {
			return "", ErrMonitorTargetDomainRequired
		}
		s = u.Hostname()
	} else {
		if i := strings.IndexAny(s, "/?#"); i >= 0 {
			s = s[:i]
		}
		if host, _, err := net.SplitHostPort(s); err == nil {
			s = host
		}
	}
	s = strings.TrimSuffix(strings.ToLower(s), ".")
	if s == "" || net.ParseIP(s) != nil {
		return "", ErrMonitorTargetDomainRequired
	}
	root, _ := RootDomain(s)
	if root == "" {
		return "", ErrMonitorTargetDomainRequired
	}
	return root, nil
}

func validateMonitorInterval(minutes int) (int, error) {
	if minutes == 0 {
		return monitorDefaultMinutes, nil
	}
	if minutes < monitorMinInterval || minutes > monitorMaxInterval {
		return 0, ErrMonitorTargetInterval
	}
	return minutes, nil
}

const monitorTargetCols = `t.id, t.company_id, coalesce(c.name,''), t.root_domain, t.interval_minutes,
	t.assets_only, t.enabled, t.last_run_at, t.last_task_id, t.last_snapshot_at, t.created_at`

type monitorTargetScanner interface {
	Scan(...any) error
}

func scanMonitorTarget(sc monitorTargetScanner) (*MonitorTarget, error) {
	var t MonitorTarget
	var lastRun, lastSnap sql.NullTime
	var lastTask sql.NullInt64
	if err := sc.Scan(&t.ID, &t.CompanyID, &t.CompanyName, &t.RootDomain, &t.IntervalMinutes,
		&t.AssetsOnly, &t.Enabled, &lastRun, &lastTask, &lastSnap, &t.CreatedAt); err != nil {
		return nil, err
	}
	if lastRun.Valid {
		v := lastRun.Time
		t.LastRunAt = &v
	}
	if lastTask.Valid {
		v := lastTask.Int64
		t.LastTaskID = &v
	}
	if lastSnap.Valid {
		v := lastSnap.Time
		t.LastSnapshotAt = &v
	}
	return &t, nil
}

// ListTargets returns every configured target (enabled or not).
func (s *MonitorStore) ListTargets(ctx context.Context) ([]MonitorTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+monitorTargetCols+`
		FROM monitor_targets t LEFT JOIN companies c ON c.id = t.company_id
		ORDER BY t.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MonitorTarget, 0, 8)
	for rows.Next() {
		t, err := scanMonitorTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// GetTarget returns one target or ErrMonitorTargetNotFound.
func (s *MonitorStore) GetTarget(ctx context.Context, id int64) (*MonitorTarget, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+monitorTargetCols+`
		FROM monitor_targets t LEFT JOIN companies c ON c.id = t.company_id
		WHERE t.id = $1`, id)
	t, err := scanMonitorTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMonitorTargetNotFound
	}
	return t, err
}

// UpsertTarget creates a target or updates the existing company+domain pair.
func (s *MonitorStore) UpsertTarget(ctx context.Context, in MonitorTargetInput) (*MonitorTarget, error) {
	if in.CompanyID <= 0 {
		return nil, fmt.Errorf("%w: 请选择企业", ErrMonitorTargetInvalid)
	}
	domain, err := normalizeMonitorDomain(in.RootDomain)
	if err != nil {
		return nil, err
	}
	interval, err := validateMonitorInterval(in.IntervalMinutes)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1)`, in.CompanyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrMonitorTargetCompanyNotFound
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `INSERT INTO monitor_targets
		(company_id, root_domain, interval_minutes, assets_only, enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (company_id, root_domain) DO UPDATE SET
			interval_minutes = EXCLUDED.interval_minutes,
			assets_only = EXCLUDED.assets_only,
			enabled = EXCLUDED.enabled,
			updated_at = now()
		RETURNING id`, in.CompanyID, domain, interval, in.AssetsOnly, in.Enabled).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetTarget(ctx, id)
}

// UpdateTarget edits interval/assets_only/enabled.
func (s *MonitorStore) UpdateTarget(ctx context.Context, id int64, patch MonitorTargetPatch) (*MonitorTarget, error) {
	if id <= 0 {
		return nil, ErrMonitorTargetNotFound
	}
	if patch.IntervalMinutes != nil {
		interval, err := validateMonitorInterval(*patch.IntervalMinutes)
		if err != nil {
			return nil, err
		}
		patch.IntervalMinutes = &interval
	}
	res, err := s.db.ExecContext(ctx, `UPDATE monitor_targets SET
		interval_minutes = COALESCE($2, interval_minutes),
		assets_only = COALESCE($3, assets_only),
		enabled = COALESCE($4, enabled),
		updated_at = now()
		WHERE id = $1`, id, patch.IntervalMinutes, patch.AssetsOnly, patch.Enabled)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, ErrMonitorTargetNotFound
	}
	return s.GetTarget(ctx, id)
}

// DeleteTarget removes one target.
func (s *MonitorStore) DeleteTarget(ctx context.Context, id int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitor_targets WHERE id = $1`, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DueTargets lists enabled targets whose interval elapsed and whose previous run
// has already been snapshotted (so runs never overlap).
func (s *MonitorStore) DueTargets(ctx context.Context, limit int) ([]MonitorTarget, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+monitorTargetCols+`
		FROM monitor_targets t LEFT JOIN companies c ON c.id = t.company_id
		WHERE t.enabled
		  AND (t.last_task_id IS NULL OR t.last_snapshot_at IS NOT NULL)
		  AND (t.last_run_at IS NULL
		       OR t.last_run_at <= now() - make_interval(mins => t.interval_minutes))
		ORDER BY t.last_run_at NULLS FIRST, t.id
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MonitorTarget, 0, limit)
	for rows.Next() {
		t, err := scanMonitorTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// TargetsAwaitingSnapshot lists launched targets whose run has not been
// snapshotted yet; the scheduler checks whether the task is terminal first.
func (s *MonitorStore) TargetsAwaitingSnapshot(ctx context.Context) ([]MonitorTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+monitorTargetCols+`
		FROM monitor_targets t LEFT JOIN companies c ON c.id = t.company_id
		WHERE t.last_task_id IS NOT NULL AND t.last_snapshot_at IS NULL
		ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MonitorTarget, 0, 8)
	for rows.Next() {
		t, err := scanMonitorTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// MarkTargetLaunched records the task that will collect this target.
func (s *MonitorStore) MarkTargetLaunched(ctx context.Context, id, taskID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE monitor_targets SET
		last_run_at = now(), last_task_id = $2, last_snapshot_at = NULL, updated_at = now()
		WHERE id = $1`, id, taskID)
	return err
}

// MarkTargetSnapshotted records that the post-run snapshot was taken.
func (s *MonitorStore) MarkTargetSnapshotted(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE monitor_targets SET
		last_snapshot_at = now(), updated_at = now()
		WHERE id = $1`, id)
	return err
}
