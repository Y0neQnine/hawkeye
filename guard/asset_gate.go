package guard

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/Autumn-27/norma/hook"
)

// assetGateToolFields maps tools whose arguments carry request targets to the
// input field holding the command/URL text. These calls are checked against the
// asset blocklist before any request leaves the host.
var assetGateToolFields = map[string]string{
	"Bash":              "command",
	"shell_open":        "command",
	"shell_send":        "text",
	"WebFetch":          "url",
	"probe_directories": "url",
}

var (
	urlTokenRe  = regexp.MustCompile(`(?i)https?://[^\s'"<>\\)]+`)
	ipTokenRe   = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	hostTokenRe = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,24}\b`)
)

// assetCandidates extracts the request targets a tool call touches. URL hosts and
// standalone host tokens are collected; tokens glued to '=' '/' '@' are query
// values, paths or emails rather than targets of their own, so they are skipped.
func assetCandidates(toolName string, input []byte) (domains, ips, urls []string) {
	field, ok := assetGateToolFields[toolName]
	if !ok {
		return nil, nil, nil
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(input, &raw) != nil {
		return nil, nil, nil
	}
	var text string
	if v, ok := raw[field]; ok {
		_ = json.Unmarshal(v, &text)
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil, nil
	}

	seenDomain := map[string]bool{}
	seenIP := map[string]bool{}
	seenURL := map[string]bool{}
	addDomain := func(s string) {
		s = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "."))
		if s != "" && !seenDomain[s] {
			seenDomain[s] = true
			domains = append(domains, s)
		}
	}
	addIP := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seenIP[s] {
			seenIP[s] = true
			ips = append(ips, s)
		}
	}
	addURL := func(s string) {
		if s != "" && !seenURL[s] {
			seenURL[s] = true
			urls = append(urls, s)
		}
	}

	for _, m := range urlTokenRe.FindAllStringIndex(text, -1) {
		rawURL := strings.TrimRight(text[m[0]:m[1]], ".,;:!?)]")
		addURL(rawURL)
		if u, err := url.Parse(rawURL); err == nil {
			if h := u.Hostname(); h != "" {
				if net.ParseIP(h) != nil {
					addIP(h)
				} else {
					addDomain(h)
				}
			}
		}
	}
	for _, m := range ipTokenRe.FindAllStringIndex(text, -1) {
		if tok := text[m[0]:m[1]]; net.ParseIP(tok) != nil {
			addIP(tok)
		}
	}
	for _, m := range hostTokenRe.FindAllStringIndex(text, -1) {
		if m[0] > 0 {
			switch text[m[0]-1] {
			case '=', '/', '@':
				continue
			}
		}
		addDomain(text[m[0]:m[1]])
	}
	return domains, ips, urls
}

// applyAssetGate blocks a tool call whose targets hit the asset blocklist. The
// task allow-list can override global rules (db.EvaluateAssetGate), so authorized
// tasks keep working when the global blocklist covers their target.
func (g *Guard) applyAssetGate(ctx context.Context, ev hook.Event) hook.Result {
	if g.interceptor == nil {
		return hook.Result{}
	}
	domains, ips, urls := assetCandidates(ev.ToolName, ev.Input)
	if len(domains)+len(ips)+len(urls) == 0 {
		return hook.Result{}
	}
	target, reason, blocked := g.interceptor.AssetGate(ctx, domains, ips, urls)
	if !blocked {
		return hook.Result{}
	}
	msg := reason
	if target != "" {
		msg = "目标 " + target + "：" + reason
	}
	return g.block(ev.ToolName, systemBlockMessage(msg), "")
}
