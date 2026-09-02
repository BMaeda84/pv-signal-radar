package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/BMaeda84/pv-signal-radar/internal/version"
)

func TestStaticUIExcludesRetiredAndMisleadingClaims(t *testing.T) {
	index := mustReadEmbeddedStatic(t, "static/index.html")
	app := mustReadEmbeddedStatic(t, "static/app.js")
	publicUI := strings.ToLower(index + "\n" + app)

	// Canvas-only charts and the former PII intake have no accessible or
	// governed fallback. Their reintroduction must therefore fail at review.
	for label, fragment := range map[string]string{
		"canvas element":        "<canvas",
		"canvas construction":   "createelement(\"canvas\")",
		"retired feedback API":  "/api/v1/feedback",
		"feedback form":         "id=\"feedback",
		"feedback e-mail field": "name=\"email\"",
	} {
		if strings.Contains(publicUI, fragment) {
			t.Errorf("public UI contains %s (%q)", label, fragment)
		}
	}

	// VigiMed is intentionally unavailable until an official, traceable
	// snapshot exists. EMA may be cited as a source, but these phrases would
	// incorrectly present the Evans teaching heuristic as an EMA criterion.
	for label, pattern := range map[string]string{
		"VigiMed surface":          `(?i)\bvigimed\b`,
		"ANVISA comparison":        `(?i)\banvisa\b`,
		"Portuguese EMA criterion": `(?i)crit[eé]rio\s+(?:da\s+)?ema`,
		"Spanish EMA criterion":    `(?i)criterio\s+(?:de\s+la\s+)?ema|umbral\s+ema`,
		"English EMA criterion":    `(?i)ema\s+criteri(?:on|a)`,
		"cross-source confirmation": `(?i)confirmad[oa]\s+(?:nas\s+duas|en\s+ambas)\s+bases|` +
			`confirmed\s+in\s+both\s+(?:data)?bases`,
		"registry presented as scientific validation": `(?i)use (?:um |un |a )?snapshot v2 validado|` +
			`use a validated v2 snapshot`,
	} {
		if regexp.MustCompile(pattern).MatchString(publicUI) {
			t.Errorf("public UI contains a prohibited %s claim", label)
		}
	}

	recorder := httptest.NewRecorder()
	newTestMux().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if csp := recorder.Header().Get("Content-Security-Policy"); csp == "" {
		t.Fatal("public UI response must declare a Content-Security-Policy")
	} else if strings.Contains(strings.ToLower(csp), "unsafe-inline") {
		t.Fatalf("public UI CSP must not allow unsafe-inline: %q", csp)
	} else if !strings.Contains(csp, "object-src 'none'") {
		t.Fatalf("public UI CSP must disable plugin/object content: %q", csp)
	}
}

func TestStaticUILocaleCatalogsCoverMarkupAndStayInParity(t *testing.T) {
	index := mustReadEmbeddedStatic(t, "static/index.html")
	app := mustReadEmbeddedStatic(t, "static/app.js")

	markupKeys := map[string]struct{}{}
	attributePattern := regexp.MustCompile(`data-i18n(?:-aria|-placeholder)?="([A-Za-z][A-Za-z0-9_]*)"`)
	for _, match := range attributePattern.FindAllStringSubmatch(index, -1) {
		markupKeys[match[1]] = struct{}{}
	}
	if len(markupKeys) == 0 {
		t.Fatal("index.html exposes no translatable data-i18n attributes")
	}

	catalogs := map[string]map[string]struct{}{}
	for _, locale := range []string{"pt-BR", "es", "en"} {
		catalogs[locale] = parseLocaleMessageKeys(t, app, locale)
		for key := range markupKeys {
			if _, exists := catalogs[locale][key]; !exists {
				t.Errorf("locale %s is missing markup key %q", locale, key)
			}
		}
	}

	// Runtime-only messages (loading, errors and chart summaries) also need
	// parity, otherwise changing locale after initial render creates mixed UI.
	for _, locale := range []string{"es", "en"} {
		missing, extra := keySetDifference(catalogs["pt-BR"], catalogs[locale])
		if len(missing) > 0 || len(extra) > 0 {
			t.Errorf("locale %s differs from pt-BR; missing=%v extra=%v", locale, missing, extra)
		}
	}
}

func TestStaticUIWiresVersionEndpointsAndResearchBoundary(t *testing.T) {
	index := mustReadEmbeddedStatic(t, "static/index.html")
	app := mustReadEmbeddedStatic(t, "static/app.js")

	for label, fragment := range map[string]string{
		"version badge":        `id="versionBadge"`,
		"health endpoint":      `fetch("/api/v1/health"`,
		"live endpoint":        `fetch("/api/v1/analyze?drug=`,
		"dataset endpoint":     `fetch("/api/v2/datasets"`,
		"research endpoint":    `fetch("/api/v2/analyses"`,
		"live boundary banner": `data-i18n="liveTitle"`,
	} {
		if !strings.Contains(index+"\n"+app, fragment) {
			t.Errorf("static UI does not expose %s (%q)", label, fragment)
		}
	}
	for locale, phrase := range map[string]string{
		"pt-BR": "não citável",
		"es":    "no citable",
		"en":    "not citable",
	} {
		body := extractLocaleObject(t, app, locale)
		if !strings.Contains(strings.ToLower(body), phrase) {
			t.Errorf("locale %s does not disclose that live results are not citable", locale)
		}
	}
	for locale, phrase := range map[string]string{
		"pt-BR": "não filtra o papel do medicamento",
		"es":    "no filtra el rol del medicamento",
		"en":    "drug role is not filtered",
	} {
		body := strings.ToLower(extractLocaleObject(t, app, locale))
		if !strings.Contains(body, phrase) || !strings.Contains(body, "drugcharacterization") {
			t.Errorf("locale %s does not disclose that live openFDA pools drug roles", locale)
		}
	}

	health := httptest.NewRecorder()
	newTestMux().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health endpoint returned %d", health.Code)
	}
	var healthPayload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(health.Body).Decode(&healthPayload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if healthPayload.Version != version.Current {
		t.Fatalf("health version %q differs from application version %q", healthPayload.Version, version.Current)
	}

	deprecated := httptest.NewRecorder()
	newTestMux().ServeHTTP(deprecated, httptest.NewRequest(http.MethodGet, "/api/v1/analyze", nil))
	if deprecated.Header().Get("Deprecation") != v1DeprecationHeader {
		t.Fatalf("v1 live endpoint must declare the RFC 9745 deprecation date %q", v1DeprecationHeader)
	}
	if deprecated.Header().Get("Sunset") != v1SunsetHeader {
		t.Fatalf("v1 live endpoint must declare the documented sunset %q", v1SunsetHeader)
	}
	if link := deprecated.Header().Get("Link"); !strings.Contains(link, "/api/v2/datasets") || !strings.Contains(link, `rel="successor-version"`) {
		t.Fatalf("v1 live endpoint does not identify its v2 successor: %q", link)
	}
	if warning := strings.ToLower(deprecated.Header().Get("Warning")); !strings.Contains(warning, "not citable") {
		t.Fatalf("v1 live endpoint warning does not disclose citation boundary: %q", warning)
	}
}

func TestStaticUIProvidesKeyboardChartAndEquivalentTables(t *testing.T) {
	index := mustReadEmbeddedStatic(t, "static/index.html")
	app := mustReadEmbeddedStatic(t, "static/app.js")
	normalized := strings.Join(strings.Fields(index), " ")

	contracts := map[string]string{
		"keyboard-focusable chart": `<div id="chart" class="chart" role="img" tabindex="0"`,
		"chart text equivalent":    `data-i18n="tableEquivalent"`,
		"results table":            `<table id="signalsTable">`,
		"research results table":   `<table id="researchResultsTable">`,
		"keyboard table region":    `class="table-scroll research-table-region" role="region" tabindex="0"`,
		"previous page control":    `<button id="researchPreviousPage" type="button" disabled`,
		"next page control":        `<button id="researchNextPage" type="button" disabled`,
		"2x2 table caption":        `<caption data-i18n="matrixCaption">`,
		"column header semantics":  `<th scope="col"`,
		"row header semantics":     `<th scope="row"`,
		"tab keyboard semantics":   `role="tablist"`,
		"chart button group":       `<div class="segmented" role="group" aria-label="Tipo de gráfico"`,
		"active chart state":       `id="chartMapTab" class="active" type="button" aria-pressed="true"`,
		"inactive chart state":     `id="chartForestTab" type="button" aria-pressed="false"`,
		"polite status updates":    `role="status" aria-live="polite"`,
	}
	for label, fragment := range contracts {
		if !strings.Contains(normalized, fragment) {
			t.Errorf("static UI is missing %s (%q)", label, fragment)
		}
	}

	chartStart := strings.Index(app, "function renderMap")
	chartEnd := strings.Index(app, "async function fetchDatasets")
	if chartStart < 0 || chartEnd <= chartStart {
		t.Fatal("cannot isolate live SVG renderers")
	}
	liveChartRenderers := app[chartStart:chartEnd]
	if strings.Contains(liveChartRenderers, `tabindex: "0"`) || strings.Contains(liveChartRenderers, `role: "img"`) {
		t.Fatal("aria-hidden SVG descendants must not remain focusable or expose conflicting roles; use the labelled container and table equivalent")
	}
	if !strings.Contains(app, `candidate.setAttribute("aria-pressed", String(active))`) {
		t.Fatal("chart button group does not keep aria-pressed synchronized with the visible chart")
	}
}

func TestStaticUIWrapsLongDatasetMetadataWithoutViewportOverflow(t *testing.T) {
	style := mustReadEmbeddedStatic(t, "static/style.css")
	for label, pattern := range map[string]string{
		"grid children may shrink":                 `(?s)\.research-workspace-grid > \*,.*?\{[^}]*min-width:\s*0;[^}]*max-width:\s*100%;`,
		"catalog item stays in column":             `(?s)\.research-catalog-panel \.dataset-item\s*\{[^}]*width:\s*100%;`,
		"metadata container may shrink":            `(?s)\.dataset-meta\s*\{[^}]*min-width:\s*0;[^}]*max-width:\s*100%;`,
		"long hash wraps":                          `(?s)\.dataset-meta \.pill\s*\{[^}]*max-width:\s*100%;[^}]*overflow-wrap:\s*anywhere;`,
		"result metadata auto-fits cards":          `(?s)\.research-result-meta\s*\{[^}]*grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(220px,\s*1fr\)\);`,
		"result identities wrap":                   `(?s)#researchAnalysisID, #researchResultDigest, #researchResultDataset\s*\{[^}]*max-width:\s*100%;[^}]*overflow-wrap:\s*anywhere;[^}]*word-break:\s*break-word;`,
		"result grid contains min-content":         `(?s)\.research-result\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\);[^}]*min-width:\s*0;[^}]*max-width:\s*100%;`,
		"result children may shrink":               `(?s)\.research-result > \*,.*?\{[^}]*min-width:\s*0;[^}]*max-width:\s*100%;`,
		"chart owns horizontal scrolling":          `(?s)\.chart\s*\{[^}]*min-width:\s*0;[^}]*max-width:\s*100%;[^}]*overflow-x:\s*auto;`,
		"table owns horizontal scrolling":          `(?s)\.table-scroll\s*\{[^}]*min-width:\s*0;[^}]*max-width:\s*100%;[^}]*overflow-x:\s*auto;`,
		"mobile result is one shrinkable column":   `(?s)@media \(max-width:\s*700px\).*?\.form-grid, \.protocol-constants, \.research-result-meta\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\);`,
		"mobile pagination has shrinkable columns": `(?s)@media \(max-width:\s*700px\).*?\.pagination-controls\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);[^}]*min-width:\s*0;[^}]*white-space:\s*normal;`,
		"pagination status gets its own row":       `(?s)\.pagination-controls #researchPageStatus\s*\{[^}]*grid-column:\s*1 / -1;[^}]*grid-row:\s*1;`,
		"pagination buttons may wrap":              `(?s)\.pagination-controls button\s*\{[^}]*min-width:\s*0;[^}]*width:\s*100%;[^}]*white-space:\s*normal;`,
	} {
		if !regexp.MustCompile(pattern).MatchString(style) {
			t.Errorf("research catalog CSS is missing %s", label)
		}
	}
}

func TestStaticUILiveStatusRetranslatesAndDoesNotAutoQuery(t *testing.T) {
	app := mustReadEmbeddedStatic(t, "static/app.js")

	readyStart := strings.Index(app, `document.addEventListener("DOMContentLoaded"`)
	readyEnd := strings.Index(app, "function normalizeStoredPreferences")
	if readyStart < 0 || readyEnd <= readyStart {
		t.Fatal("cannot isolate DOMContentLoaded initialization")
	}
	if strings.Contains(app[readyStart:readyEnd], "performAnalysis(") {
		t.Fatal("page initialization must remain idle; live openFDA is queried only after an explicit submit or preset action")
	}

	for label, fragment := range map[string]string{
		"stored translation state": `liveStatus: null`,
		"locale re-render":         "applyTranslations();\n    renderLiveStatus();",
		"loading state":            `setLiveStatus("loading", { drug: drug })`,
		"HTTP error state":         `setLiveStatus("error", { failure: normalizeLiveFailure(payload, response) })`,
		"network error state":      `code: "network_error"`,
		"translated status":        `status.textContent = t(state.liveStatus.key, params)`,
		"known-code mapping":       `analysis_unavailable: "liveErrorUnavailable"`,
		"sanitized fallback":       `sanitizeLiveErrorDetail(value.detail, 240)`,
		"retry guidance":           `t("liveErrorRetry", { seconds: Math.ceil(retryAfter) })`,
		"technical code retained":  `t("liveErrorCode", { code: code })`,
	} {
		if !strings.Contains(app, fragment) {
			t.Errorf("live status locale contract is missing %s (%q)", label, fragment)
		}
	}
	for code, key := range map[string]string{
		"drug_required":         "liveErrorDrugRequired",
		"invalid_drug":          "liveErrorInvalidDrug",
		"analysis_busy":         "liveErrorBusy",
		"analysis_rate_limited": "liveErrorRateLimited",
		"analysis_unavailable":  "liveErrorUnavailable",
		"method_not_allowed":    "liveErrorMethodNotAllowed",
	} {
		mapping := code + `: "` + key + `"`
		if !strings.Contains(app, mapping) {
			t.Errorf("live endpoint code %q is not mapped to localized key %q", code, key)
		}
	}
	if strings.Contains(app, `setLiveStatus("error", { message: error.message`) {
		t.Fatal("raw browser errors must be normalized and sanitized before entering the localized status state")
	}
}

func TestStaticResearchFormUsesOnlyImplementedV2Protocol(t *testing.T) {
	index := mustReadEmbeddedStatic(t, "static/index.html")
	app := mustReadEmbeddedStatic(t, "static/app.js")
	normalized := strings.Join(strings.Fields(index), " ")

	for label, fragment := range map[string]string{
		"research form":           `<form id="researchForm"`,
		"registered dataset":      `<select id="researchDataset" name="dataset_id" required disabled>`,
		"explicit drug concept":   `name="drug_concept_id"`,
		"canonical event scope":   `<code>all_recorded_source_pts</code>`,
		"canonical comparator":    `<code>all_other_eligible_reports</code>`,
		"none profile":            `<option value="none"`,
		"educational profile":     `<option value="evans-educational-v1"`,
		"advanced method control": `<fieldset class="advanced-only method-fieldset">`,
		"guided explanation":      `<div class="guided-only protocol-explanation"`,
		"batch-only disclosure":   `BCPNN/IC`,
		"GPS disclosure":          `GPS/EBGM`,
		"temporal disclosure":     `<span>Temporal</span>`,
		"strata disclosure":       `<span>Strata</span>`,
	} {
		if !strings.Contains(normalized, fragment) {
			t.Errorf("research UI is missing %s (%q)", label, fragment)
		}
	}

	rolePattern := regexp.MustCompile(`<option value="(primary_suspect|secondary_suspect|concomitant|interacting|suspect|all)"`)
	roles := map[string]struct{}{}
	for _, match := range rolePattern.FindAllStringSubmatch(index, -1) {
		roles[match[1]] = struct{}{}
	}
	for _, role := range []string{"primary_suspect", "secondary_suspect", "concomitant", "interacting", "suspect", "all"} {
		if _, exists := roles[role]; !exists {
			t.Errorf("research form does not expose canonical drug role %q", role)
		}
	}

	methodPattern := regexp.MustCompile(`name="research_method" value="([^"]+)"`)
	methods := map[string]struct{}{}
	for _, match := range methodPattern.FindAllStringSubmatch(index, -1) {
		methods[match[1]] = struct{}{}
	}
	expectedMethods := map[string]struct{}{"prr": {}, "ror": {}, "fisher_exact": {}}
	missing, extra := keySetDifference(expectedMethods, methods)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("interactive research methods must be exactly PRR/ROR/Fisher; missing=%v extra=%v", missing, extra)
	}

	for label, fragment := range map[string]string{
		"research schema":          `schema_version: "pv-signal-radar.research/v1"`,
		"event scope payload":      `event_scope: "all_recorded_source_pts"`,
		"comparator payload":       `comparator: "all_other_eligible_reports"`,
		"empty period":             `period: {}`,
		"fail-closed engine gate":  `payload.research_analysis_enabled === true`,
		"POST method":              `method: "POST"`,
		"JSON request":             `body: JSON.stringify(protocol)`,
		"result identity":          `payload.analysis_id`,
		"result row digest":        `payload.result_digest`,
		"result row count":         `payload.row_count`,
		"complete result manifest": `payload.dataset_manifest`,
		"result rows":              `payload.rows`,
		"result caveats":           `result.caveats`,
		"deterministic export":     `"/api/v2/analyses/" + encodeURIComponent(payload.analysis_id) + "/export"`,
	} {
		if !strings.Contains(app, fragment) {
			t.Errorf("research client is missing %s (%q)", label, fragment)
		}
	}

	// Batch-only methods may be explained, but must never be submitted by an
	// enabled form control in the SQLite-backed online workflow.
	for _, forbidden := range []string{`name="research_method" value="bcpnn_ic"`, `name="research_method" value="gps_ebgm"`} {
		if strings.Contains(index, forbidden) {
			t.Errorf("batch-only method is exposed as an online control: %q", forbidden)
		}
	}
	exportTag := regexp.MustCompile(`<a id="researchExport"[^>]*>`).FindString(index)
	if exportTag == "" || !strings.Contains(exportTag, " hidden") {
		t.Fatalf("export control must be an initially hidden inert anchor, got %q", exportTag)
	}
	if strings.Contains(exportTag, "href=") {
		t.Fatalf("export anchor must not receive a URL before a successful analysis, got %q", exportTag)
	}
}

func TestStaticResearchResultHasPaginatedAccessibleEquivalent(t *testing.T) {
	index := mustReadEmbeddedStatic(t, "static/index.html")
	app := mustReadEmbeddedStatic(t, "static/app.js")
	normalized := strings.Join(strings.Fields(index), " ")

	for label, fragment := range map[string]string{
		"initially hidden result":  `<section id="researchResult" class="research-result" aria-labelledby="researchResultTitle" hidden>`,
		"hidden export":            `<a id="researchExport" class="button-link" hidden`,
		"analysis identifier":      `id="researchAnalysisID"`,
		"result digest":            `id="researchResultDigest"`,
		"declared row count":       `id="researchResultRows"`,
		"dataset reference":        `id="researchResultDataset"`,
		"caveat list":              `id="researchCaveats"`,
		"focusable forest":         `id="researchForest" class="chart research-forest" role="img" tabindex="0"`,
		"forest description":       `aria-describedby="researchForestDescription researchForestSelection researchTableHelp"`,
		"visual selection summary": `id="researchForestSelection" class="visual-selection"`,
		"pagination summary":       `id="researchPaginationSummary" class="pagination-summary" role="status" aria-live="polite" tabindex="-1"`,
		"pagination group":         `class="pagination-controls" role="group" data-i18n-aria="researchPaginationLabel"`,
		"keyboard table region":    `role="region" tabindex="0" data-i18n-aria="researchTableRegion"`,
		"paginated table":          `<table id="researchResultsTable">`,
		"table caption":            `<caption class="sr-only" data-i18n="researchTableCaption">`,
		"table body":               `id="researchResultsBody"`,
		"review flags":             `data-i18n="researchFlags"`,
	} {
		if !strings.Contains(normalized, fragment) {
			t.Errorf("research result is missing %s (%q)", label, fragment)
		}
	}
	for _, heading := range []string{"<th scope=\"col\">a</th>", "<th scope=\"col\">b</th>", "<th scope=\"col\">c</th>", "<th scope=\"col\">d</th>", "<th scope=\"col\">N</th>", "PRR (IC 95%)", "ROR (IC 95%)", "Fisher p", "BH q"} {
		if !strings.Contains(normalized, heading) {
			t.Errorf("research result table lacks %q", heading)
		}
	}
	for _, field := range []string{"result.result_digest", "result.row_count", "contingency_table", "table.a", "table.b", "table.c", "table.d", "table.n", "metrics.prr", "metrics.ror", "metrics.fisher_exact.p_value", "metrics.fisher_exact.q_value", "review_flags"} {
		if !strings.Contains(app, field) {
			t.Errorf("research renderer does not consume %q", field)
		}
	}
	researchRenderer := app[strings.Index(app, "function renderResearchForest"):]
	if strings.Contains(researchRenderer, `.slice(`) {
		t.Fatal("research forest silently truncates event labels")
	}
}

func TestStaticResearchResultBoundsDOMWorkWithoutTruncatingExport(t *testing.T) {
	index := mustReadEmbeddedStatic(t, "static/index.html")
	app := mustReadEmbeddedStatic(t, "static/app.js")
	normalized := strings.Join(strings.Fields(index), " ")

	for label, fragment := range map[string]string{
		"fixed table page size":       `const RESEARCH_PAGE_SIZE = 50;`,
		"fixed forest visual limit":   `const RESEARCH_FOREST_LIMIT = 30;`,
		"bounded page selection":      `const pageRows = rows.filter(function (_, index) { return index >= startIndex && index < endIndex; });`,
		"table receives page only":    `renderResearchTable(pageRows);`,
		"forest receives page only":   `renderResearchForest(pageRows, {`,
		"forest limit guard":          `plottable.length >= RESEARCH_FOREST_LIMIT`,
		"complete export URL":         `"/api/v2/analyses/" + encodeURIComponent(payload.analysis_id) + "/export"`,
		"localized pagination markup": `data-i18n="researchPreviousPage"`,
		"localized region name":       `data-i18n-aria="researchTableRegion"`,
	} {
		if !strings.Contains(app+"\n"+normalized, fragment) {
			t.Errorf("large-result UI is missing %s (%q)", label, fragment)
		}
	}
	for _, forbidden := range []string{`renderResearchTable(rows);`, `renderResearchForest(rows);`} {
		if strings.Contains(app, forbidden) {
			t.Errorf("research renderer can create unbounded DOM nodes: %q", forbidden)
		}
	}
	for locale, phrase := range map[string]string{
		"pt-BR": "bundle exportado preserva todos os {total} pares",
		"es":    "bundle exportado preserva los {total} pares",
		"en":    "exported bundle preserves all {total} pairs",
	} {
		body := strings.ToLower(extractLocaleObject(t, app, locale))
		if !strings.Contains(body, phrase) {
			t.Errorf("locale %s does not distinguish visual pagination from complete export", locale)
		}
	}
}

func mustReadEmbeddedStatic(t *testing.T, name string) string {
	t.Helper()
	content, err := staticFS.ReadFile(name)
	if err != nil {
		t.Fatalf("read embedded %s: %v", name, err)
	}
	return string(content)
}

func parseLocaleMessageKeys(t *testing.T, app, locale string) map[string]struct{} {
	t.Helper()
	body := extractLocaleObject(t, app, locale)
	keys := map[string]struct{}{}

	for offset := 0; offset < len(body); {
		for offset < len(body) && (body[offset] == ' ' || body[offset] == '\t' || body[offset] == '\r' || body[offset] == '\n' || body[offset] == ',') {
			offset++
		}
		if offset == len(body) {
			break
		}
		start := offset
		for offset < len(body) && ((body[offset] >= 'a' && body[offset] <= 'z') || (body[offset] >= 'A' && body[offset] <= 'Z') || (body[offset] >= '0' && body[offset] <= '9') || body[offset] == '_') {
			offset++
		}
		if start == offset {
			t.Fatalf("locale %s has an unsupported message key near %q", locale, body[offset:min(offset+24, len(body))])
		}
		key := body[start:offset]
		for offset < len(body) && (body[offset] == ' ' || body[offset] == '\t') {
			offset++
		}
		if offset >= len(body) || body[offset] != ':' {
			t.Fatalf("locale %s key %q is not followed by a colon", locale, key)
		}
		offset++
		for offset < len(body) && (body[offset] == ' ' || body[offset] == '\t') {
			offset++
		}
		if offset >= len(body) || body[offset] != '"' {
			t.Fatalf("locale %s key %q must have a string value", locale, key)
		}
		offset++
		for escaped := false; offset < len(body); offset++ {
			if escaped {
				escaped = false
				continue
			}
			if body[offset] == '\\' {
				escaped = true
				continue
			}
			if body[offset] == '"' {
				offset++
				break
			}
		}
		if _, duplicate := keys[key]; duplicate {
			t.Fatalf("locale %s declares message key %q more than once", locale, key)
		}
		keys[key] = struct{}{}
	}
	return keys
}

func extractLocaleObject(t *testing.T, app, locale string) string {
	t.Helper()
	markers := map[string]string{
		"pt-BR": `"pt-BR": {`,
		"es":    `es: {`,
		"en":    `en: {`,
	}
	marker, supported := markers[locale]
	if !supported {
		t.Fatalf("unsupported test locale %q", locale)
	}
	markerOffset := strings.Index(app, marker)
	if markerOffset < 0 {
		t.Fatalf("message catalog for locale %s was not found", locale)
	}
	start := markerOffset + strings.Index(app[markerOffset:], "{") + 1
	depth := 1
	inString := false
	escaped := false
	for offset := start; offset < len(app); offset++ {
		character := app[offset]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
			} else if character == '"' {
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return app[start:offset]
			}
		}
	}
	t.Fatalf("message catalog for locale %s has no closing brace", locale)
	return ""
}

func keySetDifference(reference, candidate map[string]struct{}) (missing, extra []string) {
	for key := range reference {
		if _, exists := candidate[key]; !exists {
			missing = append(missing, key)
		}
	}
	for key := range candidate {
		if _, exists := reference[key]; !exists {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}
