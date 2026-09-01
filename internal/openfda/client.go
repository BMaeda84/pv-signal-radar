package openfda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

const (
	DefaultBaseURL     = "https://api.fda.gov/drug/event.json"
	DefaultTimeout     = 10 * time.Second
	AnalysisTimeout    = 25 * time.Second
	MaxReactionWorkers = 5
	// MaxReactionsPerAnalysis bounds both the visible result set and the number
	// of reaction-background requests made by one cache-miss scan.
	MaxReactionsPerAnalysis = 25
)

// Client interacts with the openFDA Drug Adverse Event API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient initializes an openFDA API client.
func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		baseURL: DefaultBaseURL,
		apiKey:  apiKey,
	}
}

// buildURL encodes query parameters once at the HTTP boundary. It prevents a
// user-supplied term from changing parameter structure while keeping the API key
// out of manually assembled URLs and error messages.
func (c *Client) buildURL(search, count string, limit int) string {
	params := url.Values{}
	if search != "" {
		params.Set("search", search)
	}
	if count != "" {
		params.Set("count", count)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if c.apiKey != "" {
		params.Set("api_key", c.apiKey)
	}

	return c.baseURL + "?" + params.Encode()
}

// exactPhrase escapes the query grammar before wrapping a drug or MedDRA term
// in quotes. URL encoding alone protects transport syntax, not the openFDA
// search language that is parsed after the URL is decoded.
func exactPhrase(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func drugSearchQuery(drugName string) string {
	phrase := exactPhrase(drugName)
	// openFDA treats whitespace-separated clauses as OR. Searching all three
	// fields expands coverage without asserting that every harmonized field is
	// populated for the same report.
	return strings.Join([]string{
		"patient.drug.openfda.generic_name:" + phrase,
		"patient.drug.openfda.brand_name:" + phrase,
		"patient.drug.medicinalproduct:" + phrase,
	}, " ")
}

// GetUniverseTotal fetches the total number of adverse event records currently in the FAERS database.
func (c *Client) GetUniverseTotal(ctx context.Context) (int64, error) {
	reqURL := c.buildURL("", "", 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, errors.New("could not construct openFDA universe request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, errors.New("openFDA universe request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("openFDA universe query returned status %d", resp.StatusCode)
	}

	var data TotalResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("could not decode openFDA universe response: %w", err)
	}

	if data.Meta.Results.Total > 0 {
		return data.Meta.Results.Total, nil
	}

	return 0, errors.New("openFDA universe response contained no records")
}

// GetDrugTotalReports fetches total adverse event reports where the requested drug is mentioned.
func (c *Client) GetDrugTotalReports(ctx context.Context, drugName string) (int64, error) {
	cleanDrug := strings.TrimSpace(drugName)
	reqURL := c.buildURL(drugSearchQuery(cleanDrug), "", 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, errors.New("could not construct openFDA drug-total request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, errors.New("openFDA drug-total request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, nil
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("openfda query returned status %d", resp.StatusCode)
	}

	var data TotalResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("could not decode openFDA drug-total response: %w", err)
	}

	return data.Meta.Results.Total, nil
}

// GetTopReactionsForDrug retrieves the top reported MedDRA Preferred Terms (PT) for the drug.
func (c *Client) GetTopReactionsForDrug(ctx context.Context, drugName string, limit int) ([]CountResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	cleanDrug := strings.TrimSpace(drugName)
	reqURL := c.buildURL(drugSearchQuery(cleanDrug), "patient.reaction.reactionmeddrapt.exact", limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, errors.New("could not construct openFDA reaction-count request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("openFDA reaction-count request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openfda count query returned status %d", resp.StatusCode)
	}

	var data CountResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("could not decode openFDA reaction-count response: %w", err)
	}

	return data.Results, nil
}

// GetReactionBackgroundTotal fetches the total number of times a given MedDRA reaction appears across all FAERS reports.
func (c *Client) GetReactionBackgroundTotal(ctx context.Context, reactionPT string) (int64, error) {
	searchQuery := "patient.reaction.reactionmeddrapt.exact:" + exactPhrase(reactionPT)
	reqURL := c.buildURL(searchQuery, "", 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, errors.New("could not construct openFDA background request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, errors.New("openFDA background request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, nil
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("openfda reaction total query status %d", resp.StatusCode)
	}

	var data TotalResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("could not decode openFDA background response: %w", err)
	}

	return data.Meta.Results.Total, nil
}

// AnalyzeDrug conducts a complete disproportionality scan for the target drug.
func (c *Client) AnalyzeDrug(ctx context.Context, drugName string) (*DrugEventAnalysis, error) {
	cleanDrug := strings.TrimSpace(drugName)
	if cleanDrug == "" {
		return nil, fmt.Errorf("drug name cannot be empty")
	}

	// One scan fans out into multiple upstream requests. Bounding the full scan,
	// rather than only each HTTP request, prevents a slow batch from outliving the
	// public handler and presenting stale or partial statistics as fresh results.
	analysisCtx, cancel := context.WithTimeout(ctx, AnalysisTimeout)
	defer cancel()

	// 1. Fetch universe N and Drug Total
	var universeN int64
	var drugTotal int64
	var topReactions []CountResult

	var wg sync.WaitGroup
	var errUniverse, errDrug, errReactions error

	wg.Add(3)
	go func() {
		defer wg.Done()
		universeN, errUniverse = c.GetUniverseTotal(analysisCtx)
	}()
	go func() {
		defer wg.Done()
		drugTotal, errDrug = c.GetDrugTotalReports(analysisCtx, cleanDrug)
	}()
	go func() {
		defer wg.Done()
		topReactions, errReactions = c.GetTopReactionsForDrug(analysisCtx, cleanDrug, MaxReactionsPerAnalysis)
	}()
	wg.Wait()

	if errDrug != nil {
		return nil, fmt.Errorf("failed to query drug totals: %w", errDrug)
	}
	if errReactions != nil {
		return nil, fmt.Errorf("failed to query reactions: %w", errReactions)
	}
	if errUniverse != nil || universeN == 0 {
		return nil, errors.New("failed to establish the current FAERS universe")
	}

	if drugTotal == 0 || len(topReactions) == 0 {
		return &DrugEventAnalysis{
			QueryDrug:         drugName,
			NormalizedDrug:    cleanDrug,
			DrugTotalReports:  drugTotal,
			DatabaseUniverseN: universeN,
			Signals:           []SignalSummary{},
			Timestamp:         time.Now().UTC().Format(time.RFC3339),
			Disclaimer:        "Spontaneous adverse event reports do not prove causal association. Notoriety bias and lack of prescription denominators must be taken into account.",
		}, nil
	}

	// 2. Concurrently fetch background counts for top reactions
	type reactionBgResult struct {
		term  string
		count int64
		total int64
		err   error
	}

	resultsChan := make(chan reactionBgResult, len(topReactions))
	sem := make(chan struct{}, MaxReactionWorkers)

	var bgWg sync.WaitGroup
	for _, rx := range topReactions {
		bgWg.Add(1)
		go func(term string, count int64) {
			defer bgWg.Done()
			select {
			case sem <- struct{}{}:
			case <-analysisCtx.Done():
				resultsChan <- reactionBgResult{term: term, count: count, err: analysisCtx.Err()}
				return
			}
			defer func() { <-sem }()

			bgTotal, err := c.GetReactionBackgroundTotal(analysisCtx, term)
			resultsChan <- reactionBgResult{
				term:  term,
				count: count,
				total: bgTotal,
				err:   err,
			}
		}(rx.Term, rx.Count)
	}

	bgWg.Wait()
	close(resultsChan)

	// 3. Assemble contingency tables and compute statistical metrics
	var signalSummaries []SignalSummary
	activeCount := 0

	for res := range resultsChan {
		// The background count is a marginal total in the 2x2 table. Replacing a
		// failed lookup with a creates c=0 and can inflate PRR/ROR into a false
		// active signal, so an incomplete scan is rejected as a whole.
		if res.err != nil || res.total == 0 {
			return nil, fmt.Errorf("failed to establish background count for reaction %q", res.term)
		}

		table, err := stats.NewContingencyTable(res.count, drugTotal, res.total, universeN)
		if err != nil {
			return nil, fmt.Errorf("invalid contingency table for reaction %q: %w", res.term, err)
		}
		statRes := table.Calculate(cleanDrug, res.term)

		if statRes.Signal == stats.SignalActive {
			activeCount++
		}

		summary := SignalSummary{
			Reaction:       res.term,
			CountA:         int64(table.A),
			DrugTotal:      int64(table.A + table.B),
			ReactionTotal:  int64(table.A + table.C),
			PRR:            statRes.PRR,
			PRRLower95:     statRes.PRRLower95,
			PRRUpper95:     statRes.PRRUpper95,
			ROR:            statRes.ROR,
			RORLower95:     statRes.RORLower95,
			RORUpper95:     statRes.RORUpper95,
			ChiSquare:      statRes.ChiSquare,
			PValueApprox:   statRes.PValueApprox,
			SignalLevel:    string(statRes.Signal),
			SignalScore:    statRes.SignalScore,
			Interpretation: statRes.Recommendation,
		}

		signalSummaries = append(signalSummaries, summary)
	}

	// Concurrent background lookups complete in non-deterministic order. Sorting
	// the API output gives clients and audit captures a stable representation of
	// one calculation over the same source response.
	sort.Slice(signalSummaries, func(i, j int) bool {
		return signalSummaries[i].Reaction < signalSummaries[j].Reaction
	})

	analysis := &DrugEventAnalysis{
		QueryDrug:          drugName,
		NormalizedDrug:     cleanDrug,
		DrugTotalReports:   drugTotal,
		DatabaseUniverseN:  universeN,
		ActiveSignalsCount: activeCount,
		TotalReactions:     len(signalSummaries),
		Signals:            signalSummaries,
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
		Disclaimer:         "Exploratory FAERS screening only: report-level co-occurrence does not establish a drug-event causal link, clinical incidence, or a validated safety conclusion.",
	}

	return analysis, nil
}
