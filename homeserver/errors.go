package homeserver

import (
	"encoding/json"
	"log"
	"net/http"
)

type ClientError struct {
	HttpCode         int
	Errcode          string
	Message          string
	AdditionalFields map[string]any
}

func MatrixHttpError(w http.ResponseWriter, code int, errcode string, msg string) {
	MustServeError(w, &ClientError{
		HttpCode: code,
		Errcode:  errcode,
		Message:  msg,
	})
}

func MustServeError(w http.ResponseWriter, clientErr *ClientError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(clientErr.HttpCode)

	// To get `AdditionalFields` to show up at the top level of the JSON object, we populate the error
	// details into it then marshal the result.
	if clientErr.AdditionalFields == nil {
		clientErr.AdditionalFields = make(map[string]any)
	}
	clientErr.AdditionalFields["errcode"] = clientErr.Errcode
	clientErr.AdditionalFields["error"] = clientErr.Message
	b, err := json.Marshal(clientErr.AdditionalFields)
	if err != nil {
		log.Fatal(err)
		return
	}
	_, _ = w.Write(b)
}
