package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/trust"
	"github.com/stretchr/testify/assert"
)

func TestSetMuninnHallSourceData(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	sampleEvent := `{
	  "content": {
		"com.muninn-hall.member_directory": {
		  "one.example.org": ["@user:example.org"],
		  "two.example.org": ["@user:example.org"],
		  "three.example.org": ["@user:example.org"]
		},
		"body": "Member Directory - plaintext body not available",
		"format": "org.matrix.custom.html",
		"formatted_body": "snipped for brevity",
		"m.mentions": {},
		"m.relates_to": {
		  "m.in_reply_to": {
			"event_id": "$example"
		  }
		},
		"msgtype": "m.notice"
	  },
	  "origin_server_ts": 1764558177788,
	  "sender": "@muninn:example.org",
	  "type": "m.room.message",
	  "unsigned": {
		"membership": "join"
	  },
	  "event_id": "$example2",
	  "room_id": "!muninn:example.org"
	}`

	eventServers := []string{"one.example.org", "two.example.org", "three.example.org"}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sources/muninn/set_member_directory_event", bytes.NewReader([]byte(sampleEvent)))
	httpSetMuninnSourceData(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	source, err := trust.NewMuninnHallSource(api.storage)
	assert.NoError(t, err)
	assert.NotNil(t, source)

	servers, err := source.GetServers(context.Background())
	assert.NoError(t, err)
	assert.ElementsMatch(t, eventServers, servers)
}

func TestSetMuninnHallSourceDataWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	sampleEvent := `{"doesn't'": "matter"}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut /*should be POST*/, "/api/v1/sources/muninn/set_member_directory_event", bytes.NewReader([]byte(sampleEvent)))
	httpSetMuninnSourceData(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}
