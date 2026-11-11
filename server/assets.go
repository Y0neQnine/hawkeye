package server

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
)

// companyScopeInputs accepts both the new [{kind,value}] contract and the
// historical ["example.com","203.0.113.10"] contract.
type companyScopeInputs []db.ScopeInput

const maxCompanyMutationBodyBytes = 2 << 20

func decodeCompanyMutationRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxCompanyMutationBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeErr(w, http.StatusRequestEntityTooLarge, "请求正文过大")
		} else {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		}
		return false
	}
	return true
}

func (items *companyScopeInputs) UnmarshalJSON(data []byte) error {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return err
	}
	out := make([]db.ScopeInput, 0, len(rawItems))
	for i, raw := range rawItems {
		var legacy string
		if err := json.Unmarshal(raw, &legacy); err == nil {
			out = append(out, db.ScopeInput{Value: legacy})
			continue
		}
		var structured db.ScopeInput
		if err := json.Unmarshal(raw, &structured); err != nil {
			return fmt.Errorf("scope[%d] must be a string or {kind,value}", i)
		}
		out = append(out, structured)
	}
	*items = out
	return nil
}

// assetStore returns the asset store.
func (s *Server) assetStore() *db.AssetStore {
	if s.m.pg == nil {
		return nil
	}
	return s.m.pg.Assets()
}

// companyStore returns the company store.
func (s *Server) companyStore() *db.CompanyStore {
	if s.m.pg == nil {
		return nil
	}
	return s.m.pg.Companies()
}

// =====================================================================
// GET /api/companies
// =====================================================================

func (s *Server) listCompanies(w http.ResponseWriter, r *http.Request) {
	cs := s.companyStore()
	if cs == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	companies, err := cs.ListCompanies()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, companies)
}

// =====================================================================
// POST /api/companies
// =====================================================================

func (s *Server) createCompany(w http.ResponseWriter, r *http.Request) {
	cs := s.companyStore()
	if cs == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	var req struct {
		Name  string             `json:"name"`
		Logo  string             `json:"logo"`
		Scope companyScopeInputs `json:"scope"`
	}
	if !decodeCompanyMutationRequest(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, 400, "name required")
		return
	}
	if err := db.ValidateCompanyScopeInputBounds(req.Scope); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	id, added, skipped, invalid, scopeErrs, err := cs.CreateCompanyWithScope(req.Name, req.Logo, req.Scope, "api")
	if err != nil {
		if errors.Is(err, db.ErrCompanyNameConflict) {
			writeErr(w, http.StatusConflict, "企业名称已存在")
			return
		}
		var validationErr *db.CompanyScopeValidationError
		if errors.As(err, &validationErr) {
			writeErr(w, http.StatusBadRequest, validationErr.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{
		"id":            id,
		"created":       true,
		"scope_added":   added,
		"scope_skipped": skipped,
		"scope_invalid": invalid,
	}
	if len(scopeErrs) > 0 {
		out["scope_errors"] = scopeErrs
	}
	writeJSON(w, 201, out)
}

// =====================================================================
// GET /api/companies/{id}
// =====================================================================

func (s *Server) getCompany(w http.ResponseWriter, r *http.Request) {
	cs := s.companyStore()
	if cs == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	c, err := cs.GetCompany(id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if c == nil {
		writeErr(w, 404, "company not found")
		return
	}
	scope, err := cs.GetScope(id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"company": c, "scope": scope})
}

// =====================================================================
// POST /api/companies/{id}/scope
// =====================================================================

func (s *Server) addCompanyScope(w http.ResponseWriter, r *http.Request) {
	cs := s.companyStore()
	if cs == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid company id")
		return
	}
	var req struct {
		Scope  companyScopeInputs `json:"scope"`
		Reason string             `json:"reason"`
		Reset  bool               `json:"reset"` // if true, replace existing scope
	}
	if !decodeCompanyMutationRequest(w, r, &req) {
		return
	}
	if err := db.ValidateCompanyScopeInputBounds(req.Scope); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var added, skipped, invalid int
	var errs []string
	var mutationErr error
	if req.Reset {
		added, invalid, errs, mutationErr = cs.UpdateScopeInputsChecked(id, req.Scope, req.Reason)
	} else {
		added, skipped, invalid, errs, mutationErr = cs.AddScopeInputsChecked(id, req.Scope, req.Reason)
	}
	if mutationErr != nil {
		if errors.Is(mutationErr, db.ErrCompanyNotFound) {
			writeErr(w, http.StatusNotFound, "company not found")
			return
		}
		var validationErr *db.CompanyScopeValidationError
		if errors.As(mutationErr, &validationErr) {
			writeErr(w, http.StatusBadRequest, validationErr.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, mutationErr.Error())
		return
	}
	out := map[string]any{
		"added":   added,
		"skipped": skipped,
		"invalid": invalid,
	}
	if len(errs) > 0 {
		out["errors"] = errs
	}
	// Reported separately from errors: this is pre-existing bad data, not a fault
	// in the submitted rules, but the operator still has to see it or those assets
	// look like the IP/CIDR rules simply never match. A failure to build the
	// warning must not fail the scope write that already committed.
	if warning, err := cs.MalformedIPAssetWarning(); err == nil && warning != "" {
		out["warnings"] = []string{warning}
	}
	writeJSON(w, 200, out)
}

// =====================================================================
// DELETE /api/companies/{id}
// =====================================================================

func (s *Server) deleteCompany(w http.ResponseWriter, r *http.Request) {
	cs := s.companyStore()
	if cs == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	var req struct {
		DeleteAssets bool `json:"delete_assets"`
	}
	// The body is optional. Only an actual empty body is ignored; malformed or
	// trailing JSON is a client error.
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	} else {
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			if err == nil {
				writeErr(w, http.StatusBadRequest, "invalid JSON: multiple values")
			} else {
				writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			}
			return
		}
	}

	assetsDeleted, err := s.m.DeleteCompanyWithAssets(id, req.DeleteAssets)
	if err != nil {
		if errors.Is(err, db.ErrCompanyNotFound) {
			writeErr(w, http.StatusNotFound, "company not found")
			return
		}
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": 1, "assets_deleted": assetsDeleted})
}

// =====================================================================
// POST /api/companies/reattribute
// =====================================================================

func (s *Server) reattribute(w http.ResponseWriter, r *http.Request) {
	cs := s.companyStore()
	if cs == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	if err := cs.RecomputeAttribution(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// =====================================================================
// GET /api/assets
// =====================================================================

const (
	defaultAssetPageSize = 50
	maxAssetPageSize     = 200
)

func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	as := s.assetStore()
	if as == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	q := r.URL.Query()
	typ := q.Get("type")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = defaultAssetPageSize
	} else if limit > maxAssetPageSize {
		limit = maxAssetPageSize
	}
	if offset < 0 {
		offset = 0
	}

	assets := []*db.Asset{}
	var err error
	// total is the full match count ignoring limit/offset. Count first so an offset
	// beyond the last row can return an empty page without an expensive scan.
	total := 0

	if dsl := q.Get("dsl"); dsl != "" {
		if err := db.ValidateDSL(dsl); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		// task_id scopes the DSL search to a task's assets (the task detail
		// "测试资产" search); company_id scopes it to one company; 0 means the
		// global asset view.
		taskID, _ := strconv.ParseInt(q.Get("task_id"), 10, 64)
		companyID, _ := strconv.ParseInt(q.Get("company_id"), 10, 64)
		total, err = as.CountDSL(dsl, typ, taskID, companyID)
		if err == nil && offset < total {
			assets, err = as.QueryDSL(dsl, typ, taskID, companyID, limit, offset)
		}
	} else {
		companyID, _ := strconv.ParseInt(q.Get("company_id"), 10, 64)
		taskID, _ := strconv.ParseInt(q.Get("task_id"), 10, 64)
		switch {
		case companyID > 0:
			total, err = as.CountByCompany(companyID, typ)
			if err == nil && offset < total {
				assets, err = as.QueryByCompany(companyID, typ, limit, offset)
			}
		case taskID > 0:
			total, err = as.CountByTask(taskID, typ)
			if err == nil && offset < total {
				assets, err = as.QueryByTask(taskID, typ, limit, offset)
			}
		default:
			if typ == "" {
				typ = "root_domain"
			}
			total, err = as.CountByType(typ)
			if err == nil && offset < total {
				assets, err = as.QueryByType(typ, limit, offset)
			}
		}
	}

	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"count":  len(assets),
		"total":  total,
		"assets": assets,
	})
}

// =====================================================================
// GET /api/assets/counts
// =====================================================================

func (s *Server) assetCounts(w http.ResponseWriter, r *http.Request) {
	as := s.assetStore()
	if as == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	var counts map[string]int
	var err error
	if companyID, _ := strconv.ParseInt(r.URL.Query().Get("company_id"), 10, 64); companyID > 0 {
		counts, err = as.CountsByTypeForCompany(companyID)
	} else if taskID, _ := strconv.ParseInt(r.URL.Query().Get("task_id"), 10, 64); taskID > 0 {
		counts, err = as.CountsByTypeForTask(taskID)
	} else {
		counts, err = as.CountsByType()
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, counts)
}

// =====================================================================
// DELETE /api/assets
// =====================================================================

func (s *Server) deleteAssets(w http.ResponseWriter, r *http.Request) {
	as := s.assetStore()
	if as == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if len(req.IDs) == 0 {
		writeErr(w, 400, "ids required")
		return
	}
	deleted, err := as.DeleteByIDs(req.IDs)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": deleted})
}

// =====================================================================
// POST /api/assets
// =====================================================================

// assetInput is one asset item accepted by POST /api/assets and by the CSV
// importer (POST /api/assets/import).
type assetInput struct {
	Type string `json:"type"`
	// root_domain / subdomain
	Domain      string   `json:"domain"`
	ICP         string   `json:"icp"`
	RecordType  string   `json:"record_type"`
	RecordValue []string `json:"record_value"`
	// ip
	IP           string           `json:"ip"`
	BoundDomains []string         `json:"bound_domains"`
	OpenPorts    []db.PortService `json:"open_ports"`
	// app
	AppName     string `json:"app_name"`
	BundleID    string `json:"bundle_id"`
	Category    string `json:"category"`
	Description string `json:"description"`
	AppICP      string `json:"app_icp"`
	// service http
	URL           string           `json:"url"`
	Technologies  []string         `json:"technologies"`
	StatusCode    *int             `json:"status_code"`
	ContentLength *int64           `json:"content_length"`
	PageTitle     string           `json:"page_title"`
	FaviconMMH3   string           `json:"favicon_mmh3"`
	Auth          []map[string]any `json:"auth"`
	ServiceIP     string           `json:"service_ip"`
	// service other
	Port        int    `json:"port"`
	ServiceName string `json:"service_name"`
	// endpoint
	Method string           `json:"method"`
	Params []map[string]any `json:"params"`
}

type assetInsertResult struct {
	Index int    `json:"index"`
	ID    int64  `json:"id"`
	Type  string `json:"type"`
}

type assetInsertError struct {
	Index int    `json:"index"`
	Line  int    `json:"line,omitempty"`
	Error string `json:"error"`
}

func insertAssetInputs(as *db.AssetStore, taskID int64, items []assetInput) ([]assetInsertResult, []assetInsertError) {
	var results []assetInsertResult
	var errs []assetInsertError
	for i, a := range items {
		var id int64
		var err error
		switch a.Type {
		case "root_domain":
			id, err = as.UpsertRootDomain(db.UpsertRootDomainReq{Domain: a.Domain, ICP: a.ICP, TaskID: taskID})
		case "ip":
			id, err = as.UpsertIP(db.UpsertIPReq{IP: a.IP, BoundDomains: a.BoundDomains, OpenPorts: a.OpenPorts, TaskID: taskID})
		case "subdomain":
			id, err = as.UpsertSubdomain(db.UpsertSubdomainReq{Domain: a.Domain, RecordType: a.RecordType, RecordValue: a.RecordValue, ICP: a.ICP, TaskID: taskID})
		case "app":
			id, err = as.UpsertApp(db.UpsertAppReq{Name: a.AppName, BundleID: a.BundleID, Category: a.Category, Description: a.Description, ICP: a.AppICP, TaskID: taskID})
		case "service":
			if a.URL != "" {
				svcIP := a.ServiceIP
				if svcIP == "" {
					svcIP = a.IP
				}
				id, err = as.UpsertHTTPService(db.UpsertHTTPServiceReq{URL: a.URL, Technologies: a.Technologies, StatusCode: a.StatusCode, ContentLength: a.ContentLength, PageTitle: a.PageTitle, FaviconMMH3: a.FaviconMMH3, Auth: a.Auth, IP: svcIP, TaskID: taskID})
			} else {
				id, err = as.UpsertOtherService(db.UpsertOtherServiceReq{Domain: a.Domain, IP: a.IP, Port: a.Port, ServiceName: a.ServiceName, Auth: a.Auth, TaskID: taskID})
			}
		case "endpoint":
			svcIP := a.ServiceIP
			if svcIP == "" {
				svcIP = a.IP
			}
			id, err = as.UpsertEndpoint(db.UpsertEndpointReq{URL: a.URL, Method: a.Method, Params: a.Params, IP: svcIP, TaskID: taskID})
		default:
			errs = append(errs, assetInsertError{Index: i, Error: "unknown type: " + a.Type})
			continue
		}
		if err != nil {
			errs = append(errs, assetInsertError{Index: i, Error: err.Error()})
			continue
		}
		if taskID > 0 {
			_ = as.SetTaskAssetSource(taskID, id, "api", "通过资产 API 登记", nil)
		}
		results = append(results, assetInsertResult{Index: i, ID: id, Type: a.Type})
	}
	return results, errs
}

func (s *Server) insertAssets(w http.ResponseWriter, r *http.Request) {
	as := s.assetStore()
	if as == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	var req struct {
		TaskID int64        `json:"task_id"`
		Assets []assetInput `json:"assets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid JSON: "+err.Error())
		return
	}
	results, errs := insertAssetInputs(as, req.TaskID, req.Assets)
	writeJSON(w, 200, map[string]any{"results": results, "errors": errs})
}

// =====================================================================
// GET /api/assets/export
// =====================================================================

var assetExportHeader = []string{
	"type", "domain", "root_domain", "ip", "port", "service_name", "url", "method",
	"record_type", "record_value", "icp", "app_name", "bundle_id", "category", "app_icp",
	"bound_domains", "open_ports", "status_code", "content_length", "page_title",
	"technologies", "favicon_mmh3", "params", "auth", "last_seen",
}

func exportCellString(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func exportAssetRow(a *db.Asset) []string {
	port := ""
	if a.Port != nil {
		port = strconv.Itoa(*a.Port)
	}
	statusCode := ""
	if a.StatusCode != nil {
		statusCode = strconv.Itoa(*a.StatusCode)
	}
	contentLength := ""
	if a.ContentLength != nil {
		contentLength = strconv.FormatInt(*a.ContentLength, 10)
	}
	openPorts := make([]string, 0, len(a.OpenPorts))
	for _, p := range a.OpenPorts {
		portVal := ""
		if v, ok := p["port"]; ok {
			portVal = fmt.Sprint(v)
		}
		svc, _ := p["service"].(string)
		if svc != "" {
			openPorts = append(openPorts, portVal+"/"+svc)
		} else if portVal != "" {
			openPorts = append(openPorts, portVal)
		}
	}
	return []string{
		a.Type, a.Domain, a.RootDomain, a.IP, port, a.ServiceName, a.URL, a.Method,
		a.RecordType, strings.Join(a.RecordValue, "|"), a.ICP, a.AppName, a.BundleID,
		a.Category, a.AppICP,
		strings.Join(a.BoundDomains, "|"), strings.Join(openPorts, "|"),
		statusCode, contentLength, a.PageTitle,
		strings.Join(a.Technologies, "|"), a.FaviconMMH3,
		exportCellString(a.Params), exportCellString(a.Auth), a.LastSeen,
	}
}

var exportableAssetTypes = []string{"root_domain", "subdomain", "ip", "service", "app", "endpoint"}

// exportAssetLimit caps a DSL-filtered export; unfiltered company/task exports
// use the stores' unlimited queries instead.
const exportAssetLimit = 100000

func isExportableAssetType(typ string) bool {
	for _, t := range exportableAssetTypes {
		if t == typ {
			return true
		}
	}
	return false
}

func (s *Server) exportAssets(w http.ResponseWriter, r *http.Request) {
	as := s.assetStore()
	if as == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	q := r.URL.Query()
	companyID, _ := strconv.ParseInt(q.Get("company_id"), 10, 64)
	taskID, _ := strconv.ParseInt(q.Get("task_id"), 10, 64)
	// type keeps the export aligned with the active asset tab: exporting from
	// 「接口」 writes endpoints only, not the whole library.
	typ := strings.TrimSpace(q.Get("type"))
	dsl := strings.TrimSpace(q.Get("dsl"))
	if typ != "" && !isExportableAssetType(typ) {
		writeErr(w, http.StatusBadRequest, "未知资产类型: "+typ)
		return
	}
	if dsl != "" {
		if err := db.ValidateDSL(dsl); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	var (
		assets []*db.Asset
		err    error
		scope  = "all"
	)
	switch {
	case companyID > 0:
		scope = fmt.Sprintf("company-%d", companyID)
	case taskID > 0:
		scope = fmt.Sprintf("task-%d", taskID)
	}
	if typ != "" {
		scope += "-" + typ
	}
	switch {
	case dsl != "":
		assets, err = as.QueryDSL(dsl, typ, taskID, companyID, exportAssetLimit, 0)
	case companyID > 0:
		assets, err = as.QueryByCompany(companyID, typ, 0, 0)
	case taskID > 0:
		assets, err = as.QueryByTask(taskID, typ, 0, 0)
	case typ != "":
		assets, err = as.QueryByType(typ, 0, 0)
	default:
		for _, t := range exportableAssetTypes {
			rows, listErr := as.QueryByType(t, 0, 0)
			if listErr != nil {
				err = listErr
				break
			}
			assets = append(assets, rows...)
		}
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	filename := fmt.Sprintf("assets-%s-%s.csv", scope, time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM keeps Excel on UTF-8
	cw := csv.NewWriter(w)
	_ = cw.Write(assetExportHeader)
	for _, a := range assets {
		_ = cw.Write(exportAssetRow(a))
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Printf("[assets] export write failed: %v", err)
	}
}

// =====================================================================
// POST /api/assets/import
// =====================================================================

func splitImportCell(cell string) []string {
	parts := strings.FieldsFunc(cell, func(r rune) bool { return r == '|' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parseImportPorts(cell string) []db.PortService {
	var out []db.PortService
	for _, entry := range splitImportCell(cell) {
		portText, service := entry, ""
		if i := strings.Index(entry, "/"); i > 0 {
			portText, service = entry[:i], strings.TrimSpace(entry[i+1:])
		}
		port, err := strconv.Atoi(strings.TrimSpace(portText))
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		out = append(out, db.PortService{Port: port, Service: service})
	}
	return out
}

func inferAssetInputFromValue(value string) (assetInput, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return assetInput{}, fmt.Errorf("空值")
	}
	if net.ParseIP(value) != nil {
		return assetInput{Type: "ip", IP: value}, nil
	}
	if strings.Contains(value, "://") {
		return assetInput{Type: "service", URL: value}, nil
	}
	host := strings.TrimSuffix(strings.ToLower(value), ".")
	if host == "" || strings.ContainsAny(host, " /\\") {
		return assetInput{}, fmt.Errorf("无法识别的资产值: %s", value)
	}
	labels := strings.Split(host, ".")
	if len(labels) == 2 {
		return assetInput{Type: "root_domain", Domain: host}, nil
	}
	if len(labels) > 2 {
		return assetInput{Type: "subdomain", Domain: host}, nil
	}
	return assetInput{}, fmt.Errorf("无法识别的资产值: %s", value)
}

func parseAssetImportCSV(records [][]string, headerIdx int) ([]assetInput, []assetInsertError) {
	idx := map[string]int{}
	for i, h := range records[headerIdx] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	get := func(rec []string, key string) string {
		i, ok := idx[key]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	var items []assetInput
	var errs []assetInsertError
	for n, rec := range records[headerIdx+1:] {
		line := headerIdx + n + 2
		joined := strings.TrimSpace(strings.Join(rec, ""))
		if joined == "" {
			continue
		}
		in := assetInput{
			Type:         strings.ToLower(get(rec, "type")),
			Domain:       get(rec, "domain"),
			ICP:          get(rec, "icp"),
			RecordType:  get(rec, "record_type"),
			RecordValue: splitImportCell(get(rec, "record_value")),
			IP:          get(rec, "ip"),
			BoundDomains: splitImportCell(get(rec, "bound_domains")),
			OpenPorts:   parseImportPorts(get(rec, "open_ports")),
			AppName:     get(rec, "app_name"),
			BundleID:    get(rec, "bundle_id"),
			Category:    get(rec, "category"),
			AppICP:      get(rec, "app_icp"),
			URL:         get(rec, "url"),
			ServiceIP:   get(rec, "service_ip"),
			PageTitle:   get(rec, "page_title"),
			FaviconMMH3: get(rec, "favicon_mmh3"),
			Technologies: splitImportCell(get(rec, "technologies")),
			ServiceName:  get(rec, "service_name"),
			Method:       strings.ToUpper(get(rec, "method")),
			Description:  get(rec, "description"),
		}
		if in.Type == "" {
			switch {
			case in.IP != "":
				in.Type = "ip"
			case in.URL != "":
				in.Type = "service"
			case in.AppName != "":
				in.Type = "app"
			case in.Domain != "":
				if labels := strings.Split(strings.TrimSuffix(in.Domain, "."), "."); len(labels) > 2 {
					in.Type = "subdomain"
				} else {
					in.Type = "root_domain"
				}
			}
		}
		if portText := get(rec, "port"); portText != "" {
			if port, err := strconv.Atoi(portText); err == nil && port > 0 && port <= 65535 {
				in.Port = port
			}
		}
		if sc := get(rec, "status_code"); sc != "" {
			if v, err := strconv.Atoi(sc); err == nil {
				in.StatusCode = &v
			}
		}
		if cl := get(rec, "content_length"); cl != "" {
			if v, err := strconv.ParseInt(cl, 10, 64); err == nil {
				in.ContentLength = &v
			}
		}
		if err := validateImportedAsset(in); err != nil {
			errs = append(errs, assetInsertError{Index: len(items), Line: line, Error: err.Error()})
			continue
		}
		items = append(items, in)
	}
	return items, errs
}

func validateImportedAsset(in assetInput) error {
	switch in.Type {
	case "root_domain":
		if in.Domain == "" {
			return fmt.Errorf("缺少 domain")
		}
	case "subdomain":
		if in.Domain == "" {
			return fmt.Errorf("缺少 domain")
		}
	case "ip":
		if in.IP == "" {
			return fmt.Errorf("缺少 ip")
		}
	case "app":
		if in.AppName == "" {
			return fmt.Errorf("缺少 app_name")
		}
	case "service":
		if in.URL == "" && (in.IP == "" || in.Port == 0) {
			return fmt.Errorf("service 需要 url，或 ip+port")
		}
	case "endpoint":
		if in.URL == "" {
			return fmt.Errorf("缺少 url")
		}
	default:
		return fmt.Errorf("未知类型: %q", in.Type)
	}
	return nil
}

// parseAssetImport accepts either the CSV produced by /api/assets/export (the
// first row containing a "type" column is the header) or a plain host/IP list
// (one domain or IP per line).
func parseAssetImport(data []byte) ([]assetInput, []assetInsertError) {
	text := strings.TrimPrefix(string(data), "\ufeff")
	cr := csv.NewReader(strings.NewReader(text))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, []assetInsertError{{Error: "CSV 解析失败: " + err.Error()}}
	}
	headerIdx := -1
	for i, rec := range records {
		for _, cell := range rec {
			if strings.EqualFold(strings.TrimSpace(cell), "type") {
				headerIdx = i
				break
			}
		}
		if headerIdx >= 0 {
			break
		}
	}
	if headerIdx >= 0 {
		return parseAssetImportCSV(records, headerIdx)
	}
	var items []assetInput
	var errs []assetInsertError
	for i, rec := range records {
		if len(rec) == 0 {
			continue
		}
		value := strings.TrimSpace(rec[0])
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		in, inferErr := inferAssetInputFromValue(value)
		if inferErr != nil {
			errs = append(errs, assetInsertError{Index: len(items), Line: i + 1, Error: inferErr.Error()})
			continue
		}
		items = append(items, in)
	}
	return items, errs
}

func (s *Server) importAssets(w http.ResponseWriter, r *http.Request) {
	as := s.assetStore()
	if as == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	companyID, _ := strconv.ParseInt(r.URL.Query().Get("company_id"), 10, 64)
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeErr(w, http.StatusRequestEntityTooLarge, "文件过大（上限 32MB）")
			return
		}
		writeErr(w, 400, "读取失败: "+err.Error())
		return
	}
	items, errs := parseAssetImport(data)
	if len(items) == 0 && len(errs) == 0 {
		writeErr(w, 400, "没有可导入的资产")
		return
	}
	results, insertErrs := insertAssetInputs(as, 0, items)
	errs = append(errs, insertErrs...)
	attached := int64(0)
	if companyID > 0 && len(results) > 0 {
		ids := make([]int64, 0, len(results))
		for _, res := range results {
			ids = append(ids, res.ID)
		}
		n, attachErr := as.SetCompanyExplicit(companyID, ids)
		if attachErr != nil {
			errs = append(errs, assetInsertError{Error: "归属企业失败: " + attachErr.Error()})
		} else {
			attached = n
		}
	}
	writeJSON(w, 200, map[string]any{
		"imported":   len(results),
		"failed":     len(errs),
		"company_id": companyID,
		"attached":   attached,
		"errors":     errs,
	})
}
