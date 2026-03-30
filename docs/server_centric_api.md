# Server-centric Communities API

Homeservers or other tooling with support for the server-centric communities API can ask policyserv to check content without it needing to be in a room.

The API is rudimentary and expected to gain more capabilities over time. 

## Authentication

To use the API, callers must get a community access token from their policyserv operator. See the [policyserv API](./api.md) for more details.

The access token is provided in the `Authorization` header as `Bearer <token>`.

## Check API

Different kinds of content can be checked using the endpoints below.

### Text content

Endpoint: `POST /_policyserv/v1/check/text`
Request body: the text to check.

If the text is allowed by the community's filters, a 200 response is returned. The response body may be empty.

If the text is not allowed, a 400 [`M_SAFETY`](https://github.com/matrix-org/matrix-spec-proposals/pull/4387) standard Matrix error is returned.

**Note**: Because [MSC4387's `M_SAFETY` error code](https://github.com/matrix-org/matrix-spec-proposals/pull/4387) is unstable, this API might return unstable identifiers.

### Event IDs

Endpoint: `POST /_policyserv/v1/check/event_id`
Request body: `{"event_id": "<your event ID>"}`

If the event is considered spammy, a 400 `M_FORBIDDEN` error is returned. Otherwise, a 200 response with an ignorable body is returned.

This will fetch events over federation if necessary.

**Note**: Because policyserv doesn't (currently) store information about which communities an event was sent in, this endpoint can be used to check events from other communities. This behaviour *should not* be relied upon as it may be removed in a future version without notice.

## Joining rooms

If a community is set up with `can_self_join_rooms`, the following endpoint can be used to join and associate a room with that community.

If the room is already associated with a community or if the room is already known, an error is returned.

Endpoint: `POST /_policyserv/v1/join/{roomId}`
Request body: empty

If the room is successfully joined, a 200 response is returned.

`M_FORBIDDEN` is returned if the community cannot join rooms. `M_BAD_STATE` is returned if the room is already known or already associated with a community.
