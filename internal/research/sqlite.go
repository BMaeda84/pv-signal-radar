package research

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/BMaeda84/pv-signal-radar/internal/stats"
	_ "modernc.org/sqlite"
)

var (
	ErrNoMatchingRows          = errors.New("no aggregate rows match the analysis protocol")
	ErrBatchMethodRequired     = errors.New("requested method requires a separately validated R batch result")
	ErrOnlineAnalysisTooLarge  = errors.New("analysis result exceeds the online execution bound")
	ErrUnknownThresholdProfile = errors.New("threshold profile is not registered")
)

const maxOnlineAnalysisRows = 50_000

var aggregateColumns = []string{
	"dataset_id", "drug_text", "drug_text_source", "drug_role", "event_pt",
	"a", "b", "c", "d", "drug_reports", "event_reports", "universe_reports",
	"comparator", "event_scope", "deduplication_policy",
}

const aggregateSchemaSQL = "PRAGMA foreign_keys = ON;" +
	"CREATE TABLE disproportionality_cells (" +
	"dataset_id TEXT NOT NULL," +
	"drug_text TEXT NOT NULL," +
	"drug_text_source TEXT NOT NULL CHECK (drug_text_source IN ('PROD_AI','DRUGNAME'))," +
	"drug_role TEXT NOT NULL,event_pt TEXT NOT NULL," +
	"a INTEGER NOT NULL CHECK (a >= 0),b INTEGER NOT NULL CHECK (b >= 0)," +
	"c INTEGER NOT NULL CHECK (c >= 0),d INTEGER NOT NULL CHECK (d >= 0)," +
	"drug_reports INTEGER NOT NULL CHECK (drug_reports = a + b)," +
	"event_reports INTEGER NOT NULL CHECK (event_reports = a + c)," +
	"universe_reports INTEGER NOT NULL CHECK (universe_reports = a + b + c + d)," +
	"comparator TEXT NOT NULL,event_scope TEXT NOT NULL,deduplication_policy TEXT NOT NULL," +
	"PRIMARY KEY (dataset_id,drug_text,drug_text_source,drug_role,event_pt)" +
	") STRICT, WITHOUT ROWID;" +
	"CREATE INDEX disproportionality_cells_event_idx ON disproportionality_cells(dataset_id,event_pt);" +
	"PRAGMA user_version = 1;"

// MaterializeSQLite imports the checked TSV interchange into a fresh database.
// It streams rows, validates every marginal before insertion, and removes a
// partial database on failure so a caller cannot register incomplete output.
func MaterializeSQLite(ctx context.Context, tsvPath, outputPath string, manifest DatasetManifest) (rowCount int64, err error) {
	if err := ValidateDatasetManifest(manifest); err != nil {
		return 0, err
	}
	inputInfo, err := os.Lstat(tsvPath)
	if err != nil {
		return 0, fmt.Errorf("inspect aggregate TSV: %w", err)
	}
	if !inputInfo.Mode().IsRegular() || inputInfo.Mode()&os.ModeSymlink != 0 {
		return 0, errors.New("aggregate TSV must be a regular non-symlink file")
	}
	if _, err := os.Lstat(outputPath); err == nil {
		return 0, errors.New("refusing to overwrite an existing SQLite artifact")
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	// #nosec G304 -- tsvPath is an explicit local materializer input whose Lstat above requires a regular non-symlink file.
	input, err := os.Open(tsvPath)
	if err != nil {
		return 0, err
	}
	defer input.Close()
	db, err := sql.Open("sqlite", outputPath)
	if err != nil {
		return 0, err
	}
	succeeded := false
	defer func() {
		closeErr := db.Close()
		if err == nil && closeErr != nil {
			err = closeErr
			succeeded = false
		}
		if !succeeded {
			_ = os.Remove(outputPath)
		}
	}()
	if _, err = db.ExecContext(ctx, aggregateSchemaSQL); err != nil {
		return 0, fmt.Errorf("create aggregate schema: %w", err)
	}
	transaction, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer transaction.Rollback()
	statement, err := transaction.PrepareContext(ctx,
		"INSERT INTO disproportionality_cells VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
	)
	if err != nil {
		return 0, err
	}
	defer statement.Close()
	reader := csv.NewReader(input)
	reader.Comma = '\t'
	reader.FieldsPerRecord = len(aggregateColumns)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read aggregate header: %w", err)
	}
	if strings.Join(header, "\x00") != strings.Join(aggregateColumns, "\x00") {
		return 0, fmt.Errorf("unexpected aggregate TSV header: %v", header)
	}
	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return 0, fmt.Errorf("read aggregate row %d: %w", rowCount+2, readErr)
		}
		if record[0] != manifest.DatasetID {
			return 0, fmt.Errorf("row %d dataset_id %q does not match manifest", rowCount+2, record[0])
		}
		if record[1] == "" || (record[2] != "PROD_AI" && record[2] != "DRUGNAME") || !allowedDrugRoles[record[3]] || record[4] == "" {
			return 0, fmt.Errorf("row %d has an invalid drug/event identity", rowCount+2)
		}
		counts := make([]int64, 7)
		for index := range counts {
			value, parseErr := strconv.ParseInt(record[5+index], 10, 64)
			if parseErr != nil || value < 0 {
				return 0, fmt.Errorf("row %d column %s is not a non-negative int64", rowCount+2, aggregateColumns[5+index])
			}
			counts[index] = value
		}
		a, b, c, d := counts[0], counts[1], counts[2], counts[3]
		if err := reconcileAggregateCounts(a, b, c, d, counts[4], counts[5], counts[6]); err != nil {
			return 0, fmt.Errorf("row %d fails marginal reconciliation: %w", rowCount+2, err)
		}
		// Every 2x2 table uses the same report-level eligible population declared
		// by the manifest. Internal marginal agreement alone is insufficient: a
		// producer could otherwise publish coherent cells for the wrong universe.
		if counts[6] != manifest.Completeness.EligibleReports {
			return 0, fmt.Errorf(
				"row %d universe_reports %d does not match manifest eligible_reports %d",
				rowCount+2,
				counts[6],
				manifest.Completeness.EligibleReports,
			)
		}
		// The serving engine currently exposes counts through the float64 v1
		// statistics contract. Reject an aggregate that cannot be represented
		// exactly instead of materializing data that every analysis must refuse.
		if _, err := stats.NewContingencyTable(a, counts[4], counts[5], counts[6]); err != nil {
			return 0, fmt.Errorf("row %d is not statistically representable: %w", rowCount+2, err)
		}
		if record[12] != ComparatorAllOtherEligibleReports || record[13] != EventScopeAllRecordedSourcePTs || record[14] != manifest.Processing.DeduplicationPolicy {
			return 0, fmt.Errorf("row %d has incompatible comparator, scope, or deduplication policy", rowCount+2)
		}
		values := make([]any, len(record))
		for index, value := range record {
			values[index] = value
		}
		for index, value := range counts {
			values[5+index] = value
		}
		if _, err := statement.ExecContext(ctx, values...); err != nil {
			return 0, fmt.Errorf("insert aggregate row %d: %w", rowCount+2, err)
		}
		rowCount++
	}
	if rowCount == 0 {
		return 0, errors.New("aggregate TSV contains no data rows")
	}
	if err := statement.Close(); err != nil {
		return 0, err
	}
	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("commit aggregate import: %w", err)
	}
	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		return 0, fmt.Errorf("finalize aggregate database: %w", err)
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return 0, fmt.Errorf("materialized SQLite integrity check failed: %s", integrity)
	}
	succeeded = true
	return rowCount, nil
}

// SQLiteEngine computes deterministic PRR/ROR/Fisher analyses over the
// contract- and integrity-checked aggregate produced by the R pipeline. These
// checks are not scientific validation. SQLite is opened in immutable read-only
// mode; analysis outputs are written only to the separate FileStore.
type SQLiteEngine struct {
	db       *sql.DB
	manifest DatasetManifest
	software SoftwareReference
}

func OpenSQLiteEngine(ctx context.Context, databasePath string, manifest DatasetManifest, optionalSoftware ...SoftwareReference) (*SQLiteEngine, error) {
	if err := ValidateDatasetManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid dataset manifest: %w", err)
	}
	software := DevelopmentSoftwareReference()
	if len(optionalSoftware) > 0 {
		software = optionalSoftware[0]
	}
	if err := ValidateSoftwareReference(software); err != nil {
		return nil, fmt.Errorf("invalid analysis software reference: %w", err)
	}
	absolute, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite path: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, fmt.Errorf("inspect SQLite artifact: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("SQLite artifact must be a regular non-symlink file")
	}
	expectedHash, err := sqliteArtifactHash(manifest)
	if err != nil {
		return nil, err
	}
	actualHash, err := fileSHA256(absolute)
	if err != nil {
		return nil, fmt.Errorf("hash SQLite artifact: %w", err)
	}
	if actualHash != expectedHash {
		return nil, fmt.Errorf("SQLite artifact checksum mismatch: manifest=%s actual=%s", expectedHash, actualHash)
	}

	uriPath := filepath.ToSlash(absolute)
	if runtime.GOOS == "windows" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := &url.URL{Scheme: "file", Path: uriPath}
	query := dsn.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	query.Add("_pragma", "query_only(1)")
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open SQLite artifact: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	engine := &SQLiteEngine{db: db, manifest: manifest, software: software}
	if err := engine.validate(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close invalid SQLite artifact: %w", closeErr))
		}
		return nil, err
	}
	return engine, nil
}

func (e *SQLiteEngine) Software() SoftwareReference {
	if e == nil {
		return SoftwareReference{}
	}
	return e.software
}

func (e *SQLiteEngine) Close() error {
	if e == nil || e.db == nil {
		return nil
	}
	return e.db.Close()
}

func (e *SQLiteEngine) DatasetID() string {
	if e == nil {
		return ""
	}
	return e.manifest.DatasetID
}

func (e *SQLiteEngine) validate(ctx context.Context) error {
	if err := e.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping read-only SQLite artifact: %w", err)
	}
	var queryOnly int
	if err := e.db.QueryRowContext(ctx, "PRAGMA query_only").Scan(&queryOnly); err != nil || queryOnly != 1 {
		return errors.New("SQLite connection did not enter query-only mode")
	}
	var integrity string
	if err := e.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return fmt.Errorf("SQLite integrity check failed: %s", integrity)
	}
	rows, err := e.db.QueryContext(ctx, "PRAGMA table_info(disproportionality_cells)")
	if err != nil {
		return fmt.Errorf("inspect aggregate schema: %w", err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var sequence, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&sequence, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("read aggregate schema: %w", err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if strings.Join(columns, "\x00") != strings.Join(aggregateColumns, "\x00") {
		return fmt.Errorf("unexpected disproportionality_cells schema: %v", columns)
	}
	var foreignDatasetCount int
	if err := e.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT dataset_id) FROM disproportionality_cells WHERE dataset_id <> ?",
		e.manifest.DatasetID,
	).Scan(&foreignDatasetCount); err != nil {
		return fmt.Errorf("validate aggregate dataset_id: %w", err)
	}
	if foreignDatasetCount != 0 {
		return errors.New("aggregate contains rows for a dataset_id other than its manifest")
	}
	var datasetRows int64
	if err := e.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM disproportionality_cells WHERE dataset_id = ?",
		e.manifest.DatasetID,
	).Scan(&datasetRows); err != nil {
		return fmt.Errorf("count aggregate dataset rows: %w", err)
	}
	if datasetRows == 0 {
		return errors.New("aggregate contains no rows for its manifest dataset_id")
	}
	var distinctUniverses int64
	var minimumUniverse, maximumUniverse sql.NullInt64
	if err := e.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT universe_reports), MIN(universe_reports), MAX(universe_reports) "+
			"FROM disproportionality_cells WHERE dataset_id = ?",
		e.manifest.DatasetID,
	).Scan(&distinctUniverses, &minimumUniverse, &maximumUniverse); err != nil {
		return fmt.Errorf("validate aggregate eligible universe: %w", err)
	}
	// Opening an externally produced SQLite artifact is a separate trust
	// boundary from TSV materialization. Recheck that every stored table uses
	// exactly the manifest denominator before allowing any analysis or export.
	if distinctUniverses != 1 || !minimumUniverse.Valid || !maximumUniverse.Valid ||
		minimumUniverse.Int64 != e.manifest.Completeness.EligibleReports ||
		maximumUniverse.Int64 != e.manifest.Completeness.EligibleReports {
		return fmt.Errorf(
			"aggregate universe_reports range [%d,%d] across %d distinct value(s) does not match manifest eligible_reports %d",
			minimumUniverse.Int64,
			maximumUniverse.Int64,
			distinctUniverses,
			e.manifest.Completeness.EligibleReports,
		)
	}
	return nil
}

// Analyze executes only methods implemented and cross-testable in Go. BCPNN and
// GPS remain batch-only because silently substituting a simplified estimator
// would make results incomparable to the declared protocol.
func (e *SQLiteEngine) Analyze(ctx context.Context, request AnalysisRequest) (AnalysisRecord, error) {
	normalized, err := NormalizeAnalysisRequest(request)
	if err != nil {
		return AnalysisRecord{}, err
	}
	if normalized.DatasetID != e.manifest.DatasetID {
		return AnalysisRecord{}, fmt.Errorf("%w: %s", ErrDatasetNotFound, normalized.DatasetID)
	}
	for _, method := range normalized.Methods {
		if method == "bcpnn_ic" || method == "gps_ebgm" {
			return AnalysisRecord{}, fmt.Errorf("%w: %s", ErrBatchMethodRequired, method)
		}
	}
	if normalized.Period.StartDate != "" || normalized.Period.EndDate != "" || len(normalized.Subgroups) > 0 {
		return AnalysisRecord{}, errors.New("this aggregate does not contain temporal or subgroup strata; use a compatible batch artifact")
	}
	source, drugText, err := parseFAERSDrugConcept(normalized.DrugConceptID)
	if err != nil {
		return AnalysisRecord{}, err
	}
	if normalized.ThresholdProfileID != ThresholdProfileNone && normalized.ThresholdProfileID != ThresholdProfileEvansEducational {
		return AnalysisRecord{}, fmt.Errorf("%w: %s", ErrUnknownThresholdProfile, normalized.ThresholdProfileID)
	}

	queryText := "SELECT event_pt, a, b, c, d, drug_reports, event_reports, universe_reports, deduplication_policy " +
		"FROM disproportionality_cells " +
		"WHERE dataset_id = ? AND drug_text = ? AND drug_text_source = ? " +
		"AND drug_role = ? AND comparator = ? AND event_scope = ? ORDER BY event_pt"
	queryArguments := []any{
		normalized.DatasetID, drugText, source, normalized.DrugRole,
		normalized.Comparator, normalized.EventScope,
	}
	countText := "SELECT COUNT(*) FROM disproportionality_cells " +
		"WHERE dataset_id = ? AND drug_text = ? AND drug_text_source = ? " +
		"AND drug_role = ? AND comparator = ? AND event_scope = ?"
	var matchingRows int64
	if err := e.db.QueryRowContext(ctx, countText, queryArguments...).Scan(&matchingRows); err != nil {
		return AnalysisRecord{}, fmt.Errorf("count matching aggregate rows: %w", err)
	}
	if matchingRows > maxOnlineAnalysisRows {
		return AnalysisRecord{}, fmt.Errorf("%w: %d rows exceeds %d", ErrOnlineAnalysisTooLarge, matchingRows, maxOnlineAnalysisRows)
	}
	rows, err := e.db.QueryContext(ctx, queryText, queryArguments...)
	if err != nil {
		return AnalysisRecord{}, fmt.Errorf("query aggregate: %w", err)
	}
	defer rows.Close()

	var output []DrugEventResult
	for rows.Next() {
		var eventPT, deduplication string
		var a, b, c, d, drugReports, eventReports, universeReports int64
		if err := rows.Scan(&eventPT, &a, &b, &c, &d, &drugReports, &eventReports, &universeReports, &deduplication); err != nil {
			return AnalysisRecord{}, fmt.Errorf("scan aggregate row: %w", err)
		}
		if deduplication != e.manifest.Processing.DeduplicationPolicy {
			return AnalysisRecord{}, fmt.Errorf("aggregate deduplication policy mismatch for event %q", eventPT)
		}
		if err := reconcileAggregateCounts(a, b, c, d, drugReports, eventReports, universeReports); err != nil {
			return AnalysisRecord{}, fmt.Errorf("aggregate marginal reconciliation failed for event %q: %w", eventPT, err)
		}
		if universeReports != e.manifest.Completeness.EligibleReports {
			return AnalysisRecord{}, fmt.Errorf(
				"aggregate universe_reports %d for event %q does not match manifest eligible_reports %d",
				universeReports,
				eventPT,
				e.manifest.Completeness.EligibleReports,
			)
		}
		table, err := stats.NewContingencyTable(a, drugReports, eventReports, universeReports)
		if err != nil {
			return AnalysisRecord{}, fmt.Errorf("invalid contingency table for event %q: %w", eventPT, err)
		}
		includeFisher := containsMethod(normalized.Methods, "fisher_exact")
		calculated := table.Calculate(normalized.DrugConceptID, eventPT)
		if includeFisher {
			calculated = table.CalculateWithFisher(normalized.DrugConceptID, eventPT)
			if !calculated.FisherExactOK {
				return AnalysisRecord{}, fmt.Errorf("%w: fisher_exact exceeds the bounded online calculation", ErrBatchMethodRequired)
			}
		}
		metrics := requestedMetrics(normalized.Methods, calculated)
		flags := requestedReviewFlags(normalized.ThresholdProfileID, calculated)
		// The first FAERS contract preserves source PT text without pretending to
		// classify clinical reactions, errors, quality issues, or ineffectiveness.
		// A later licensed/versioned classifier can introduce narrower scopes.
		category := "unclassified_source_pt"
		output = append(output, DrugEventResult{
			DrugConceptID:  normalized.DrugConceptID,
			EventConceptID: "meddra-pt-text:" + eventPT,
			EventTerm:      eventPT,
			EventCategory:  category,
			Table:          IntegerTable{A: a, B: b, C: c, D: d, N: universeReports},
			Metrics:        metrics,
			ReviewFlags:    flags,
		})
	}
	if err := rows.Err(); err != nil {
		return AnalysisRecord{}, err
	}
	// The count and result query use the same immutable SQLite snapshot. An
	// unexpected difference is therefore a failed family construction, never a
	// reason to publish a silently shortened emitted family.
	if int64(len(output)) != matchingRows {
		return AnalysisRecord{}, fmt.Errorf("result family row count mismatch: counted %d, emitted %d", matchingRows, len(output))
	}
	if len(output) == 0 {
		return AnalysisRecord{}, ErrNoMatchingRows
	}
	applyBenjaminiHochberg(output)
	caveats := append([]string{}, e.manifest.Limitations...)
	caveats = append(caveats,
		"Reporting disproportionality does not establish causality, incidence, prevalence, or clinical risk.",
		"Event concept IDs preserve source MedDRA PT text because no licensed numeric concept mapping was asserted.",
		"The all_other_eligible_reports comparator is the report-level complement of the selected drug concept and role inside this frozen eligible universe; it is not an unexposed-patient cohort.",
		"Fisher q-values, when present, use Benjamini-Hochberg correction across all returned events in this protocol.",
	)
	result, err := NewAnalysisResultForSoftware(e.manifest, normalized, e.software, output, caveats)
	if err != nil {
		return AnalysisRecord{}, err
	}
	csvBytes, err := resultCSV(result)
	if err != nil {
		return AnalysisRecord{}, err
	}
	return AnalysisRecord{
		Dataset: e.manifest,
		Result:  result,
		Files: []ExportFile{
			{Name: "results.csv", MediaType: "text/csv; charset=utf-8", Data: csvBytes},
			{Name: "METHODS.txt", MediaType: "text/plain; charset=utf-8", Data: []byte(methodsText(result))},
		},
	}, nil
}

func containsMethod(methods []string, wanted string) bool {
	for _, method := range methods {
		if method == wanted {
			return true
		}
	}
	return false
}

func requestedMetrics(methods []string, result stats.DisproportionalityResult) []MetricEstimate {
	var metrics []MetricEstimate
	// Generation and validation share these typed constructors so a future
	// correction-policy change cannot silently make newly written records fail
	// their own trust-boundary validation (or, worse, describe different cells).
	effectCalculation := expectedEffectCalculation(result)
	observedCalculation := observedMetricCalculation()
	for _, method := range methods {
		switch method {
		case "prr":
			lower, upper := result.PRRLower95, result.PRRUpper95
			metrics = append(metrics, MetricEstimate{Method: "prr", Measure: "reporting_ratio", Estimate: result.PRR, Lower95: &lower, Upper95: &upper, Calculation: effectCalculation})
		case "ror":
			lower, upper := result.RORLower95, result.RORUpper95
			metrics = append(metrics, MetricEstimate{Method: "ror", Measure: "reporting_odds_ratio", Estimate: result.ROR, Lower95: &lower, Upper95: &upper, Calculation: effectCalculation})
		case "fisher_exact":
			if result.FisherExactOK {
				p := result.FisherExactP
				metrics = append(metrics, MetricEstimate{Method: "fisher_exact", Measure: "two_sided_probability_ordering", Estimate: p, PValue: &p, Calculation: observedCalculation})
			}
		}
	}
	return metrics
}

func requestedReviewFlags(profileID string, result stats.DisproportionalityResult) []ReviewFlag {
	if profileID == ThresholdProfileNone {
		return nil
	}
	outcome := "below_profile"
	if result.ScreeningOutcome == stats.ScreeningMeetsProfile {
		outcome = "meets_profile"
	} else if result.ScreeningOutcome == stats.ScreeningIntermediateReview {
		outcome = "intermediate_review"
	}
	return []ReviewFlag{{
		ProfileID: profileID,
		Outcome:   outcome,
		Reason:    fmt.Sprintf("a=%.0f; PRR=%.6g; Yates chi-square=%.6g", result.Table.A, result.PRR, result.ChiSquare),
	}}
}

func applyBenjaminiHochberg(rows []DrugEventResult) {
	type reference struct {
		p      float64
		metric *MetricEstimate
	}
	var values []reference
	for rowIndex := range rows {
		for metricIndex := range rows[rowIndex].Metrics {
			metric := &rows[rowIndex].Metrics[metricIndex]
			if metric.Method == "fisher_exact" && metric.PValue != nil {
				values = append(values, reference{p: *metric.PValue, metric: metric})
			}
		}
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].p < values[j].p })
	adjusted := 1.0
	for index := len(values) - 1; index >= 0; index-- {
		rank := float64(index + 1)
		candidate := values[index].p * float64(len(values)) / rank
		adjusted = math.Min(adjusted, math.Min(1, candidate))
		q := adjusted
		values[index].metric.QValue = &q
	}
}

// reconcileAggregateCounts validates report-level margins without evaluating
// unchecked int64 expressions. A wrapped a+b+c+d can otherwise equal a small,
// apparently valid universe and conceal a corrupt or hostile aggregate row.
func reconcileAggregateCounts(a, b, c, d, drugReports, eventReports, universeReports int64) error {
	add := func(left, right int64) (int64, error) {
		if left < 0 || right < 0 {
			return 0, errors.New("counts cannot be negative")
		}
		if left > math.MaxInt64-right {
			return 0, errors.New("count total overflows int64")
		}
		return left + right, nil
	}

	drugTotal, err := add(a, b)
	if err != nil {
		return err
	}
	eventTotal, err := add(a, c)
	if err != nil {
		return err
	}
	abc, err := add(drugTotal, c)
	if err != nil {
		return err
	}
	total, err := add(abc, d)
	if err != nil {
		return err
	}
	if drugTotal != drugReports || eventTotal != eventReports || total != universeReports {
		return errors.New("cells do not match the declared drug, event, and universe margins")
	}
	return nil
}

func parseFAERSDrugConcept(identifier string) (source, drugText string, err error) {
	parts := strings.SplitN(identifier, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", "", errors.New("drug_concept_id must use faers-prod_ai:<text> or faers-drugname:<text>")
	}
	switch strings.ToLower(parts[0]) {
	case "faers-prod_ai":
		return "PROD_AI", parts[1], nil
	case "faers-drugname":
		return "DRUGNAME", parts[1], nil
	default:
		return "", "", errors.New("drug_concept_id source must be faers-prod_ai or faers-drugname")
	}
}

func sqliteArtifactHash(manifest DatasetManifest) (string, error) {
	var hashes []string
	for _, artifact := range manifest.Artifacts {
		if artifact.MediaType == "application/vnd.sqlite3" || strings.EqualFold(filepath.Ext(artifact.Path), ".sqlite") {
			hashes = append(hashes, artifact.SHA256)
		}
	}
	if len(hashes) != 1 {
		return "", fmt.Errorf("manifest must declare exactly one SQLite artifact, found %d", len(hashes))
	}
	return hashes[0], nil
}

func fileSHA256(path string) (string, error) {
	// #nosec G304 -- callers Lstat a regular non-symlink artifact or pass a fixed filename inside a private staging directory.
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FileSHA256 returns the lowercase digest used by dataset manifests.
func FileSHA256(path string) (string, error) {
	return fileSHA256(path)
}

func resultCSV(result AnalysisResult) ([]byte, error) {
	var output strings.Builder
	writer := csv.NewWriter(&output)
	if err := writer.Write([]string{"analysis_id", "result_digest", "result_family_id", "result_row_count", "result_row_number", "drug_concept_id", "event_concept_id", "event_term", "event_category", "a", "b", "c", "d", "n", "method", "measure", "estimate", "lower_95", "upper_95", "p_value", "q_value", "input_cells", "zero_cell_correction_applied", "zero_cell_correction_method", "added_to_each_cell"}); err != nil {
		return nil, err
	}
	for rowIndex, row := range result.Rows {
		for _, metric := range row.Metrics {
			record := []string{
				spreadsheetSafeText(result.AnalysisID), spreadsheetSafeText(result.ResultDigest),
				spreadsheetSafeText(result.ResultFamily.FamilyID), strconv.FormatInt(result.RowCount, 10), strconv.Itoa(rowIndex + 1),
				spreadsheetSafeText(row.DrugConceptID),
				spreadsheetSafeText(row.EventConceptID), spreadsheetSafeText(row.EventTerm), spreadsheetSafeText(row.EventCategory),
				strconv.FormatInt(row.Table.A, 10), strconv.FormatInt(row.Table.B, 10),
				strconv.FormatInt(row.Table.C, 10), strconv.FormatInt(row.Table.D, 10), strconv.FormatInt(row.Table.N, 10),
				spreadsheetSafeText(metric.Method), spreadsheetSafeText(metric.Measure), formatFloat(metric.Estimate),
				formatOptionalFloat(metric.Lower95), formatOptionalFloat(metric.Upper95),
				formatOptionalFloat(metric.PValue), formatOptionalFloat(metric.QValue),
				spreadsheetSafeText(metric.Calculation.InputCells),
				strconv.FormatBool(metric.Calculation.ZeroCellCorrection.Applied),
				spreadsheetSafeText(metric.Calculation.ZeroCellCorrection.Method),
				formatFloat(metric.Calculation.ZeroCellCorrection.AddedToEachCell),
			}
			if err := writer.Write(record); err != nil {
				return nil, err
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return []byte(output.String()), nil
}

func methodsText(result AnalysisResult) string {
	return fmt.Sprintf(
		"PV Signal Radar reproducible analysis\nAnalysis ID: %s\nResult digest: %s\nResult family: %s\nEmitted rows: %d\nCanonical row order: %s\nDataset ID: %s\nSoftware: %s %s (%s)\nMethods: %s\nThreshold profile: %s\nComparator: %s\n\nThe result digest is SHA-256 over canonical JSON containing the result-family definition, declared row count, and exact ordered rows emitted in analysis.json. It detects changes to that emitted family; it does not establish upstream dataset completeness, expected scientific-family completeness, or scientific validity.\n\nEach a cell counts unique eligible reports containing the selected drug-event pair. The b/c/d cells and margins count unique eligible reports under the declared comparator; they are not patient exposure denominators. PRR/ROR use log-Wald 95%% intervals. When an observed cell is zero, their per-metric metadata records the Haldane-Anscombe correction; Fisher exact always uses observed integer cells. Fisher is two-sided by probability ordering and q-values use Benjamini-Hochberg across all returned events.\n\nCSV safety: textual fields whose first non-whitespace character is =, +, -, or @ are prefixed with an apostrophe to prevent spreadsheet formula execution. analysis.json remains the lossless canonical result. CSV repeats the digest, family ID, row count, and one-based canonical row number so detached rows retain their emitted-family context.\n\nThese statistics prioritize reports for review; they do not establish causality or incidence.\n",
		result.AnalysisID, result.ResultDigest, result.ResultFamily.FamilyID, result.RowCount, result.ResultFamily.RowOrder,
		result.Dataset.DatasetID, result.Software.Name, result.Software.Version, result.Software.Commit,
		strings.Join(result.Request.Methods, ", "), result.Request.ThresholdProfileID, result.Request.Comparator,
	)
}

func spreadsheetSafeText(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return value
	}
	if strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return formatFloat(*value)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}
