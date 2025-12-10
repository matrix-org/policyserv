# API

Policyserv has a rudimentary API to do some common tasks. To enable the API, set `PS_API_KEY` to a secret value.

## Authentication

Supply the `Authorization` header with a `Bearer` token matching `PS_API_KEY`. For example, if you have `PS_API_KEY=changeme` then your header would be `Authorization: Bearer changeme`.

## Join Rooms API

**Deprecated: Replaced by `POST /api/v1/rooms/{roomId}/join`** - do not use deprecated endpoints.

When policyserv is joined to a room, it considers that room protected. Do not ask policyserv to join rooms you don't want to protect.

Note: you can also use `PS_JOIN_ROOM_IDS` to join rooms. The API method just avoids a restart.

Example:
```bash
# Set APIKEY to your PS_API_KEY value
APIKEY=changeme
curl -s -X POST -H "Authorization: Bearer ${APIKEY}" --data-binary '{"via":"example.org","room_ids": ["!room:example.org"]}' https://example.org/api/v1/join_rooms
```

Request method: `POST`
Request body:
```json
{
  "via": "example.org",
  "room_ids": [
    "!room:example.org"
  ]
}
```

If there's an error, a standard Matrix error will be returned. If successful, expect `{"joined_all": true}` and 200 OK.

The logs can be monitored to ensure the room IDs were correctly picked up.

Room joins may be retried internally, blocking the request until they complete.

## Set room moderator API

**Deprecated: Not intended to be used long-term.**

If policyserv has an access token for the room's moderator, it will attempt to redact any events it deems not allowed.

Each room can have its moderator set by the following endpoint.

Example:
```bash
APIKEY=changeme
curl -s -X POST -H "Authorization: Bearer ${APIKEY}" --data-binary '{"room_id":"!room:example.org","moderator_user_id":"@mod:example.org"}' https://example.org/api/v1/set_room_moderator
```

Request method: `POST`
Request body:
```json
{
  "room_id": "!room:example.org",
  "moderator_user_id": "@mod:example.org"
}
```

To unset the moderator, use an empty string for `moderator_user_id`.

If there's an error, a standard Matrix error will be returned. If successful, expect your request body to be returned to you and 200 OK.

## Get rooms API

Retrieves the rooms policyserv is protecting, and some details about them.

Example:
```bash
APIKEY=changeme
curl -s -X GET -H "Authorization: Bearer ${APIKEY}" https://example.org/api/v1/rooms
```

Request method: `GET`
Request body: None

Returns a standard error response upon error, or the following with 200 OK on success:

```json
[
  {
    "room_id": "!room:example.org",
    "room_version": "10",
    "moderator_user_id": "@mod:example.org",
    "last_cached_state_timestamp": 1759773439484,
    "community_id": "33DDrMuWa8IxiRupoG6fTLbEoBP"
  }
]
```

`moderator_user_id` will be empty if not set for the room.

To retrieve a single room's details, use `GET /api/v1/rooms/{roomId}` instead. It returns `404 M_NOT_FOUND` if the room does not exist on the server.

## Join Room API

Joins a room and assigns it to a community.

Example:
```bash
APIKEY=changeme
curl -s -X POST -H "Authorization: Bearer ${APIKEY}" --data-binary '{"community_id": "33DDrMuWa8IxiRupoG6fTLbEoBP"}' https://example.org/api/v1/rooms/!ROOMID/join
```

Request method: `POST`
Request body:

```json
{
  "community_id": "33DDrMuWa8IxiRupoG6fTLbEoBP"
}
```

Returns a standard error response upon error (`M_BAD_STATE` for rooms which already exist, etc), or the following with 200 OK on success:

```json
{
  "room_id": "!room:example.org",
  "room_version": "10",
  "last_cached_state_timestamp": 1759773439484,
  "community_id": "33DDrMuWa8IxiRupoG6fTLbEoBP"
}
```

## Communities API

Can be used to create/get/update community details.

Example:
```bash
APIKEY=changeme
curl -s -X POST -H "Authorization: Bearer ${APIKEY}" --data-binary '{"name":"new community"}' https://example.org/api/v1/communities/new
curl -s -X GET -H "Authorization: Bearer ${APIKEY}" https://example.org/api/v1/communities/33DDrMuWa8IxiRupoG6fTLbEoBP
curl -s -X POST -H "Authorization: Bearer ${APIKEY}" --data-binary '{"keyword_filter_keywords":["spammy spam", "spam"]}' https://example.org/api/v1/communities/33DDrMuWa8IxiRupoG6fTLbEoBP/config
```

See above for implied request methods and bodies. Note: it is not currently possible to change the community name via the API.

All endpoints return standard error responses upon error, or the following with 200 OK on success:

```json
{
  "community_id": "33DDrMuWa8IxiRupoG6fTLbEoBP",
  "name": "test community",
  "config": {
    "keyword_filter_keywords": ["spammy spam", "spam"]
  }
}
```

**Note**: The instance's config can be retrieved via `GET /api/v1/instance/community_config`.

### Set Muninn Hall Source Data (Member Directory Event)

Use this endpoint to set the latest member directory event from [Muninn Hall](https://muninn-hall.com/). To get this event, say `!member-directory` in the Muninn Hall room, then View Source on the reply. That event JSON is what should be supplied here.

Example:
```bash
APIKEY=changeme
curl -s -X POST -H "Authorization: Bearer ${APIKEY}" --data-binary '{ ... event ... }' https://example.org/api/v1/sources/muninn/set_member_directory_event
```

The endpoint returns 200 OK on success, or a standard error response upon error.
