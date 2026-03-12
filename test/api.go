package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func MakeJsonBody(t *testing.T, body any) io.Reader {
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.NotNil(t, b)
	return bytes.NewReader(b)
}

func AssertApiError(t *testing.T, w *httptest.ResponseRecorder, errcode string, error string) {
	jsonErr := make(map[string]any)
	err := json.Unmarshal(w.Body.Bytes(), &jsonErr)
	assert.NoError(t, err)
	assert.Equal(t, errcode, jsonErr["errcode"])
	assert.Equal(t, error, jsonErr["error"])
}

func AssertJsonBody(t *testing.T, w *httptest.ResponseRecorder, expected any) {
	expectedJson, err := json.Marshal(expected)
	assert.NoError(t, err)
	assert.JSONEq(t, string(expectedJson), w.Body.String())
}
