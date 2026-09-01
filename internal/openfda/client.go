package openfda

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

const (
	DefaultBaseURL     = "https://api.fda.gov/drug/event.json"
	DefaultTimeout     = 10 * time.Second
	MaxReactionWorkers = 5
	// FallbackUniverseN provides an approximate baseline if global total query fails (~26 million FAERS reports)
	FallbackUniverseN int64 = 26000000
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

// GetUniverseTotal fetches the total number of adverse event records currently in the FAERS database.
func (c *Client) GetUniverseTotal(ctx context.Context) (int64, error) {
	reqURL := fmt.Sprintf("%s?search=_exists_:patient.reaction.reactionmeddrapt&limit=1", c.baseURL)
	if c.apiKey != "" {
		reqURL += "&api_key=" + url.QueryEscape(c.apiKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return FallbackUniverseN, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FallbackUniverseN, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FallbackUniverseN, fmt.Errorf("openfda returned status %d", resp.StatusCode)
	}

	var data TotalResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return FallbackUniverseN, err
	}

	if data.Meta.Results.Total > 0 {
		return data.Meta.Results.Total, nil
	}

	return FallbackUniverseN, nil
}

// GetDrugTotalReports fetches total adverse event reports where the requested drug is mentioned.
func (c *Client) GetDrugTotalReports(ctx context.Context, drugName string) (int64, error) {
	cleanDrug := strings.TrimSpace(drugName)
	escapedDrug := url.QueryEscape(fmt.Sprintf(`"%s"`, cleanDrug))
	searchQuery := fmt.Sprintf(`patient.drug.openfda.generic_name:%s+patient.drug.openfda.brand_name:%s+patient.drug.medicinalproduct:%s`, escapedDrug, escapedDrug, escapedDrug)

	reqURL := fmt.Sprintf("%s?search=%s&limit=1", c.baseURL, searchQuery)
	if c.apiKey != "" {
		reqURL += "&api_key=" + url.QueryEscape(c.apiKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
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
		return 0, err
	}

	return data.Meta.Results.Total, nil
}

// GetTopReactionsForDrug retrieves the top reported MedDRA Preferred Terms (PT) for the drug.
func (c *Client) GetTopReactionsForDrug(ctx context.Context, drugName string, limit int) ([]CountResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	cleanDrug := strings.TrimSpace(drugName)
	escapedDrug := url.QueryEscape(fmt.Sprintf(`"%s"`, cleanDrug))
	searchQuery := fmt.Sprintf(`patient.drug.openfda.generic_name:%s+patient.drug.openfda.brand_name:%s+patient.drug.medicinalproduct:%s`, escapedDrug, escapedDrug, escapedDrug)

	reqURL := fmt.Sprintf("%s?search=%s&count=patient.reaction.reactionmeddrapt.exact&limit=%d", c.baseURL, searchQuery, limit)
	if c.apiKey != "" {
		reqURL += "&api_key=" + url.QueryEscape(c.apiKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return data.Results, nil
}

// GetReactionBackgroundTotal fetches the total number of times a given MedDRA reaction appears across all FAERS reports.
func (c *Client) GetReactionBackgroundTotal(ctx context.Context, reactionPT string) (int64, error) {
	escapedReaction := url.QueryEscape(fmt.Sprintf(`"%s"`, reactionPT))
	reqURL := fmt.Sprintf("%s?search=patient.reaction.reactionmeddrapt.exact:%s&limit=1", c.baseURL, escapedReaction)
	if c.apiKey != "" {
		reqURL += "&api_key=" + url.QueryEscape(c.apiKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
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
		return 0, err
	}

	return data.Meta.Results.Total, nil
}

// AnalyzeDrug conducts a complete disproportionality scan for the target drug.
func (c *Client) AnalyzeDrug(ctx context.Context, drugName string) (*DrugEventAnalysis, error) {
	cleanDrug := strings.TrimSpace(drugName)
	if cleanDrug == "" {
		return nil, fmt.Errorf("drug name cannot be empty")
	}

	// 1. Fetch universe N and Drug Total
	var universeN int64
	var drugTotal int64
	var topReactions []CountResult

	var wg sync.WaitGroup
	var errUniverse, errDrug, errReactions error

	wg.Add(3)
	go func() {
		defer wg.Done()
		universeN, errUniverse = c.GetUniverseTotal(ctx)
	}()
	go func() {
		defer wg.Done()
		drugTotal, errDrug = c.GetDrugTotalReports(ctx, cleanDrug)
	}()
	go func() {
		defer wg.Done()
		topReactions, errReactions = c.GetTopReactionsForDrug(ctx, cleanDrug, 25)
	}()
	wg.Wait()

	if errDrug != nil {
		return nil, fmt.Errorf("failed to query drug totals: %w", errDrug)
	}
	if errReactions != nil {
		return nil, fmt.Errorf("failed to query reactions: %w", errReactions)
	}
	if errUniverse != nil || universeN == 0 {
		universeN = FallbackUniverseN
	}

	if drugTotal == 0 || len(topReactions) == 0 {
		return &DrugEventAnalysis{
			QueryDrug:         drugName,
			NormalizedDrug:    cleanDrug,
			DrugTotalReports:  0,
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
			sem <- struct{}{}
			defer func() { <-sem }()

			bgTotal, err := c.GetReactionBackgroundTotal(ctx, term)
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
		if res.err != nil || res.total == 0 {
			res.total = res.count // Fallback
		}

		table := stats.NewContingencyTable(res.count, drugTotal, res.total, universeN)
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

	analysis := &DrugEventAnalysis{
		QueryDrug:          drugName,
		NormalizedDrug:     cleanDrug,
		DrugTotalReports:   drugTotal,
		DatabaseUniverseN:  universeN,
		ActiveSignalsCount: activeCount,
		TotalReactions:     len(signalSummaries),
		Signals:            signalSummaries,
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
		Disclaimer:         "FAERS data reflects spontaneous reports where causality is unconfirmed. Disproportionality metrics (PRR/ROR) indicate statistical reporting associations, not incidence rates.",
	}

	return analysis, nil
}
