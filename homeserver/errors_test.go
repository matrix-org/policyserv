package homeserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatrixHttpError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	MatrixHttpError(w, http.StatusBadRequest, "M_FORBIDDEN", "Example error message")
	assert.Equal(t, w.Code, http.StatusBadRequest)
	assert.Equal(t, w.Body.String(), `{"errcode":"M_FORBIDDEN","error":"Example error message"}`)
}

func TestMustServeError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	MustServeError(w, &ClientError{
		HttpCode: http.StatusBadRequest,
		Errcode:  "M_FORBIDDEN",
		Message:  "Example error message",
		AdditionalFields: map[string]any{
			"example_field": "example_value",
		},
	})
	assert.Equal(t, w.Code, http.StatusBadRequest)
	assert.Equal(t, w.Body.String(), `{"errcode":"M_FORBIDDEN","error":"Example error message","example_field":"example_value"}`)

	// AdditionalFields should also be optional
	w = httptest.NewRecorder()
	MustServeError(w, &ClientError{
		HttpCode: http.StatusBadRequest,
		Errcode:  "M_FORBIDDEN",
		Message:  "Example error message",
	})
	assert.Equal(t, w.Code, http.StatusBadRequest)
	assert.Equal(t, w.Body.String(), `{"errcode":"M_FORBIDDEN","error":"Example error message"}`)
}
