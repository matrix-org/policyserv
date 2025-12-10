package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/matrix-org/policyserv/metrics"
)

func parseJsonBody(val any, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &val)
	if err != nil {
		return err
	}
	return nil
}

func respondJson(action string, r *http.Request, w http.ResponseWriter, val any) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}

	defer metrics.RecordHttpResponse(r.Method, action, http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
	return nil
}
