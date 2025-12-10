package homeserver

import (
	"fmt"
	"net/http"
)

func MatrixHttpError(w http.ResponseWriter, code int, errcode string, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"errcode": "%s", "error": "%s"}`, errcode, msg)))
}
