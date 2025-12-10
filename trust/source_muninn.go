package trust

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

type MuninnHallMemberDirectory map[string][]string

type MuninnMemberDirectoryEvent struct {
	/*
		Example event:
		{
		  "content": {
		    "com.muninn-hall.member_directory": {    // <--- this is what we extract (and specifically the keys)
		      "matrix.org": [
		        "@mackesque:matrix.org",
		        "@travis:t2l.io"
		      ]
		    },
			"msgtype": "m.notice"
			"body": "Member Directory - plaintext body not available",
		    "format": "org.matrix.custom.html",
		    "formatted_body": "[snip]",
		    "m.mentions": {},
		    "m.relates_to": {
		      "m.in_reply_to": {
		        "event_id": "$5WawMwdZ5Fg09EHvwO0x91UnZxb3bTNYK3eh2pz-IkA"
		      }
		    }
		  },
		  "origin_server_ts": 1764558177788,
		  "sender": "@muninn:maunium.net",
		  "type": "m.room.message",
		  "unsigned": {
		    "membership": "join"
		  },
		  "event_id": "$383r7SDb2jQ5u17Rlg-1_fb7tdXsKpp4rlIP8eg5WJc",
		  "room_id": "!EHp86jT1xltaYs4-1rbRJiu1zo9hDEgpYvbsPsrJDOY"
		}
	*/

	Content struct {
		MemberDirectory MuninnHallMemberDirectory `json:"com.muninn-hall.member_directory"`
	} `json:"content"`
}

// MuninnHallSource - uses the Muninn Hall member directory to determine which servers have higher trust levels in
// communities.
type MuninnHallSource struct {
	db storage.PersistentStorage
}

func NewMuninnHallSource(db storage.PersistentStorage) (*MuninnHallSource, error) {
	return &MuninnHallSource{
		db: db,
	}, nil
}

func (s *MuninnHallSource) HasCapability(ctx context.Context, userId string, roomId string, capability Capability) (Tristate, error) {
	parsedId, err := spec.NewUserID(userId, true)
	if err != nil {
		return TristateDefault, err
	}

	serverNames, err := s.GetServers(ctx)
	if err != nil {
		return TristateDefault, err
	}

	// Currently, all Muninn Hall members have all capabilities.
	if slices.Contains(serverNames, string(parsedId.Domain())) {
		return TristateTrue, nil
	}

	return TristateDefault, nil
}

// Dev note: below here we hide the persistence details from the rest of the code for maintenance purposes. Please keep
// this stuff together for visibility/ease of maintenance.

type muninnHallServerData struct {
	ServerNames []string `json:"server_names"`
}

const muninnHallSourceName = "muninn_hall"
const muninnHallSourceKey = "" // we don't scope to anything, so don't key by anything

func (s *MuninnHallSource) GetServers(ctx context.Context) ([]string, error) {
	val := &muninnHallServerData{}
	err := s.db.GetTrustData(ctx, muninnHallSourceName, muninnHallSourceKey, &val)
	if errors.Is(err, sql.ErrNoRows) {
		return make([]string, 0), nil
	}
	return val.ServerNames, err
}

func (s *MuninnHallSource) ImportData(ctx context.Context, directory MuninnHallMemberDirectory) error {
	val := &muninnHallServerData{
		ServerNames: make([]string, 0),
	}
	for k, _ := range directory {
		val.ServerNames = append(val.ServerNames, k)
	}
	return s.db.SetTrustData(ctx, muninnHallSourceName, muninnHallSourceKey, val)
}
