package openfda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

func TestUnavailableFisherPIsOmittedInsteadOfSerializedAsSentinel(t *testing.T) {
	result := stats.ContingencyTable{A: 10, B: 90, C: 20, D: 880, N: 1000}.Calculate("drug", "event")
	summary := SignalSummary{
		FisherExactP:  availableFisherP(result),
		FisherExactOK: result.FisherExactOK,
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if strings.Contains(string(payload), "fisher_exact_two_sided_p") {
		t.Fatalf("unavailable Fisher p-value must be absent, got %s", payload)
	}
	if !strings.Contains(string(payload), `"fisher_exact_available":false`) {
		t.Fatalf("availability flag must remain explicit, got %s", payload)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testClient(server *httptest.Server) *Client {
	return &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
		apiKey:     "test-openfda-secret",
	}
}

func TestGetUniverseTotalQueriesEntireDataset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if got := query.Get("search"); got != "" {
			t.Fatalf("universe query must not exclude reports: search=%q", got)
		}
		if got := query.Get("limit"); got != "1" {
			t.Fatalf("expected limit=1, got %q", got)
		}
		if got := query.Get("api_key"); got != "test-openfda-secret" {
			t.Fatalf("expected API key to be encoded as a parameter, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"meta":{"results":{"total":12345}}}`)
	}))
	defer server.Close()

	total, err := testClient(server).GetUniverseTotal(context.Background())
	if err != nil {
		t.Fatalf("unexpected universe error: %v", err)
	}
	if total != 12345 {
		t.Fatalf("expected full-dataset total 12345, got %d", total)
	}
}

func TestGetUniverseTotalRejectsOversizedUpstreamResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"meta":{"results":{"total":1}},"padding":"%s"}`, strings.Repeat("x", maxOpenFDAResponseBytes))
	}))
	defer server.Close()

	if _, err := testClient(server).GetUniverseTotal(context.Background()); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected bounded upstream-response failure, got %v", err)
	}
}

func TestExactPhraseEscapesOpenFDAGrammar(t *testing.T) {
	input := `ACME" + patient.reaction.reactionmeddrapt:"HEADACHE`
	got := exactPhrase(input)
	want := `"ACME\" + patient.reaction.reactionmeddrapt:\"HEADACHE"`
	if got != want {
		t.Fatalf("unexpected escaped phrase:\nwant %q\n got %q", want, got)
	}
	if got := exactPhrase(`A\B`); got != `"A\\B"` {
		t.Fatalf("expected a literal backslash to be escaped, got %q", got)
	}

	search := drugSearchQuery(input)
	if strings.Contains(search, `ACME" + patient`) {
		t.Fatalf("search contains an unescaped quote that can alter grammar: %q", search)
	}
	if count := strings.Count(search, `patient.drug.`); count != 3 {
		t.Fatalf("expected three explicit drug fields, got %d in %q", count, search)
	}
}

func TestTransportFailureDoesNotExposeAPIKey(t *testing.T) {
	client := NewClient("test-openfda-secret")
	client.httpClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: errors.New("network unavailable")}
	})}

	_, err := client.GetUniverseTotal(context.Background())
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), "test-openfda-secret") {
		t.Fatalf("transport error leaked the API key: %q", err)
	}
}

func TestAnalyzeDrugRejectsMissingBackgroundInsteadOfFabricatingSignal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch {
		case query.Get("search") == "":
			_, _ = fmt.Fprint(w, `{"meta":{"results":{"total":1000000}}}`)
		case query.Get("count") != "":
			_, _ = fmt.Fprint(w, `{"meta":{"results":{"total":1}},"results":[{"term":"HEADACHE","count":10}]}`)
		case strings.Contains(query.Get("search"), "patient.reaction.reactionmeddrapt.exact"):
			http.Error(w, "background unavailable", http.StatusServiceUnavailable)
		default:
			_, _ = fmt.Fprint(w, `{"meta":{"results":{"total":1000}}}`)
		}
	}))
	defer server.Close()

	_, err := testClient(server).AnalyzeDrug(context.Background(), "ExampleDrug")
	if err == nil {
		t.Fatal("expected analysis to fail when a background marginal is unavailable")
	}
	if !strings.Contains(err.Error(), "background count") {
		t.Fatalf("expected a background-count error, got %q", err)
	}
}
