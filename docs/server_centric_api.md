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
