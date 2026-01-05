package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestKeywordTemplate(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	template := &storage.StoredKeywordTemplate{
		Name: "TESTING",
		Body: "FIRST VALUE GOES HERE",
	}

	// First we should get a 404 on this template
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/keyword_templates/"+template.Name, nil)
	r.SetPathValue("name", template.Name)
	httpKeywordTemplates(api, w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
	test.AssertApiError(t, w, "M_NOT_FOUND", "Template not found")

	// Now we can populate it
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/v1/keyword_templates/"+template.Name, bytes.NewBufferString(template.Body))
	r.SetPathValue("name", template.Name)
	httpKeywordTemplates(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	test.AssertJsonBody(t, w, template)

	// Verify it was persisted
	val, err := api.storage.GetKeywordTemplate(context.Background(), template.Name)
	assert.NoError(t, err)
	assert.Equal(t, template, val)

	// Fetch it for good measure
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/v1/keyword_templates/"+template.Name, nil)
	r.SetPathValue("name", template.Name)
	httpKeywordTemplates(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	test.AssertJsonBody(t, w, template)

	// Set it to something different
	template.Body = "ALTERED"
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/v1/keyword_templates/"+template.Name, bytes.NewBufferString(template.Body))
	r.SetPathValue("name", template.Name)
	httpKeywordTemplates(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	test.AssertJsonBody(t, w, template)

	// Verify it was persisted
	val, err = api.storage.GetKeywordTemplate(context.Background(), template.Name)
	assert.NoError(t, err)
	assert.Equal(t, template, val)
}

func TestKeywordTemplateWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut /* should be GET|POST */, "/api/v1/keyword_templates/TESTING", bytes.NewBufferString("doesn't matter"))
	r.SetPathValue("name", "TESTING")
	httpKeywordTemplates(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	test.AssertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}
