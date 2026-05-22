package scan

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSignerMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`# HELP nomic_signer_errors Number of errors encountered
# TYPE nomic_signer_errors counter
nomic_signer_errors 7
# HELP nomic_signer_checkpoint_index Current checkpoint index
# TYPE nomic_signer_checkpoint_index gauge
nomic_signer_checkpoint_index 42
# HELP nomic_signer_checkpoint_timestamp The creation time of the newest checkpoint
# TYPE nomic_signer_checkpoint_timestamp gauge
nomic_signer_checkpoint_timestamp 1716400000
`))
	}))
	defer srv.Close()

	metrics, err := getSignerMetrics(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("getSignerMetrics returned error: %v", err)
	}
	if metrics.errorsCounter != 7 {
		t.Fatalf("expected errorsCounter 7, got %d", metrics.errorsCounter)
	}
	if metrics.checkpointIndex != 42 {
		t.Fatalf("expected checkpointIndex 42, got %d", metrics.checkpointIndex)
	}
	if metrics.checkpointUnixTime != 1716400000 {
		t.Fatalf("expected checkpointUnixTime 1716400000, got %d", metrics.checkpointUnixTime)
	}
}
