# policyserv

A community-centric [MSC4284 Policy Server](https://github.com/matrix-org/matrix-spec-proposals/pull/4284) implementation 
for Matrix. Learn more about policy servers from [the matrix.org blog](https://matrix.org/blog/2025/12/policyserv/).

✨ Looking to use policyserv in your own Matrix community? [Try the Foundation's instance](https://github.com/matrix-org/policyserv-setup-bot?tab=readme-ov-file#usage)
or [deploy your own](#deploying)!

Policyserv offers communities the ability to apply filters and moderation policies to content before it reaches other
users or the room itself. This is currently limited to communities made up of rooms on Matrix (possibly via a space),
but will include the homeserver itself as a large dedicated community in the future. Server support is currently in the
"research" phase and may change significantly before the first recommended release.

Policyserv can (and probably should) be used alongside other safety tooling, such as [Draupnir](https://github.com/the-draupnir-project/Draupnir),
[Meowlnir](https://github.com/maunium/meowlnir), and [Mjolnir](https://github.com/matrix-org/mjolnir). These additional
layers ensure maximum protection for your rooms, especially if content makes it through the policy server.

For help and support with policyserv, or getting set up on the Foundation's instance, visit [#policyserv:matrix.org](https://matrix.to/#/#policyserv:matrix.org)
on Matrix.

## Deploying

Some experience with deploying homeservers is assumed. We recommend first deploying [Synapse](https://github.com/element-hq/synapse)
to gain experience with the overall process, though Synapse is not required to run or use policyserv.

Before deploying policyserv, you will need to generate two signing keys: one for events, and one for the homeserver.
Both keys can be generated using the `gen_signing_keys` binary included in the policyserv Docker image, like so:

```bash
docker run --rm -it -v /path/to/data:/data ghcr.io/matrix-org/policyserv:main gen_signing_keys
```

Once you have your signing keys, you'll need to prepare your configuration. All configuration options are provided as
environment variables as shown below. Note that some environment variables will define the "instance config" - this is
the default configuration applied to all communities using the instance and can be overridden through the API.

The required configuration options are:

* `PS_DATABASE` (default `postgres://policyserv:devonly@localhost/policyserv?sslmode=disable`) - The URI for your PostgreSQL
  database. We recommend using a dedicated database for policyserv, and using a supported PostgreSQL version. We have
  tested policyserv with PostgreSQL 16.
* `PS_HOMESERVER_NAME` (default `localhost`) - The hostname of the policy server itself. This is the domain name used to
  set up federation. Port 443 (HTTPS) *must* be open on this domain, *must* terminate SSL, and *must* point to the bind
  address for policyserv. It's strongly recommended to forward all traffic to policyserv through a reverse proxy.
* `PS_HTTP_BIND` (default `0.0.0.0:8080`) - The address to bind the HTTP server to. This should not need changing in 
  Docker, though the specific port mapping may be different.

Everything else is optional, though may be useful for some deployments:

* `PS_DATABASE_MAX_OPEN_CONNS` (default `10`) - The maximum number of connections to open to the database.
* `PS_DATABASE_MAX_IDLE_CONNS` (default `5`) - The maximum number of idle connections to open to the database.
* `PS_DATABASE_READ` (default empty value) - The readonly URI for the policyserv database. If empty, the normal database will be used as a read source.
* `PS_DATABASE_READ_MAX_OPEN_CONNS` (default `10`) - The maximum number of readonly connections to open to the database.
* `PS_DATABASE_READ_MAX_IDLE_CONNS` (default `5`) - The maximum number of idle connections to open to the database.
* `PS_KEY_QUERY_SERVER` (default `matrix.org,ed25519:a_RXGa,l8Hft5qXKn1vfHrg3p4+W8gELQVo8N13JkluMfmn2sQ`) - The **trusted** server to query keys from and its key information in CSV format (`name,keyId,keyBase64`).
* `PS_TRUSTED_ORIGINS` (default `matrix.org,element.io`) - The hostnames in CSV format which are trusted to provide information like room state to policyserv. It's best to list at least 1 server here. Do not list servers which might deliberately or accidentally return confusing/inaccurate state for rooms.
* `PS_STATE_CACHE_MINUTES` (default `5`) - The minimum number of minutes to keep room state caches fresh.
* `PS_JOIN_SERVER` (default `matrix.org`) - The server to send the join event through.
* `PS_JOIN_ROOM_IDS` (default empty value) - The room IDs to join to receive events in, and therefore protect. Removing a room from this list does *not* unprotect it. Rooms will become part of the `default` community.
* `PS_JOIN_LOCALPART` (default `policyserv`) - The localpart for the user ID which joins the rooms.
* `PS_API_KEY` (default empty value) - The API key which enables use of the policyserv API. If set, this should be a random value and considered a password. If unset or empty, the API will not be enabled.
* `PS_MODERATOR_ACCESS_TOKENS` (default empty value) - The access tokens and Client-Server API URL domains for those tokens used for moderation (redaction). Example: `matrix-client.matrix.org:syt_example,gnome.ems.host:syt_example2`
* `PS_HTTP_PPROF_BIND` (default `0.0.0.0:8082` in Docker, empty value otherwise) - The address to bind the [pprof](https://pkg.go.dev/net/http/pprof) endpoints to. Not bound if an empty value. Recommended to be a local address (or not exposed by the container).
* `PS_HTTP_METRICS_BIND` (default `0.0.0.0:8081`) - The address to bind the Prometheus metrics endpoint to. Recommended to be a local address (or not exposed by the container). Cannot be disabled.
* `PS_ALLOWED_WEBHOOK_DOMAINS` (default `element.ems.host`) - CSV list of the hostnames/domains policyserv is allowed to send webhooks to.
* `PS_HOMESERVER_MEDIA_CLIENT_URL` (default `https://matrix-client.matrix.org`) - The client-server API URL to use for fetching media.
* `PS_HOMESERVER_MEDIA_CLIENT_ACCESS_TOKEN` (default empty value) - The access token to use for fetching media on the above client-server API URL.

Support information can be supplied using the following environment variables. These are used to populate the [`/.well-known/matrix/support`](https://spec.matrix.org/v1.17/client-server-api/#getwell-knownmatrixsupport)
endpoint, and may be used by clients to help communities get set up using your policyserv instance.

* `PS_SUPPORT_ADMIN_CONTACTS` (default empty value) - CSV list of email addresses or Matrix user IDs which can be contacted for general support.
* `PS_SUPPORT_SECURITY_CONTACTS` (default empty value) - CSV list of email addresses or Matrix user IDs which can be contacted for security issues.
* `PS_SUPPORT_URL` (default empty value) - The URL where users can find more information or support for the policyserv instance. Typically, this is an instance-specific user guide.

Some environment variables that can be set explicitly but shouldn't in most cases are:

* `PS_DATABASE_MIGRATIONS_DIR` (default `/opt/migrations` in Docker, `./migrations` otherwise) - The directory where database migrations reside. Should not need changing in Docker.
* `PS_PROCESSING_POOL_SIZE` (default `100`) - How many concurrent events to process, roughly speaking.
* `PS_MODERATION_POOL_SIZE` (default `25`) - How many concurrent moderation actions (redactions) to process, roughly speaking.
* `PS_WEBHOOK_POOL_SIZE` (default `5`) - How many concurrent webhook notifications to process, roughly speaking.
* `PS_HOMESERVER_SIGNING_KEY_PATH` (default `/data/signing.key` in Docker, `./signing.key` otherwise) - The path to the signing key generated above. Should not need changing in Docker.
* `PS_HOMESERVER_EVENT_SIGNING_KEY_PATH` (default `/data/event_signing.key` in Docker, `./event_signing.key` otherwise) - The path to the signing key used to sign events, generated above. Should not need changing in Docker. Note: The Key Version (ID) of this key is not used.

Once you have your signing keys and an idea for your config, you can deploy policyserv using the Docker image mentioned 
below. If you prefer to compile policyserv yourself, run `go build -o bin/policyserv ./cmd/app/...` and then run the 
resulting binary.

```bash
docker run -d -p 127.0.0.1:8080:8080 -v /path/to/data:/data \
  -e PS_HOMESERVER_NAME=beta2.matrix.org \  # and etc
  ghcr.io/matrix-org/policyserv:main  # you should use a tagged release rather than `main`.
```

## Filter configuration

The instance defaults (used by all communities by default) are configured with environment variables. Individual communities
can have these overridden or changed. Typically, communities are expected to manage their policyserv usage through a dedicated
[policyserv-setup-bot](https://github.com/matrix-org/policyserv-setup-bot) instance attached to your policyserv instance.
It is not recommended to allow communities to use the policyserv API directly - that is for you and your setup bot to use.

In future it will be possible for communities to specify the order/sequencing of filters. For now, the order is determined
within policyserv itself.

### Filter considerations

Policyserv is best used in public or near-public communities. For rooms, this typically means having `public` join rules,
or a way for the policyserv user to join the room without an invite (policyserv does not support receiving invites). Rooms 
being protected by policyserv should *not* be encrypted. Policyserv will not scan encrypted messages properly, which might 
lead to spam making it through to users.

For servers, public or near-public primarily means that a relatively untrusted user can gain an account on the server,
even after email verification or other requirements have been met. A server is considered private if *all* users on the 
server are trusted by the server administrator(s).

### General

* `PS_SPAM_THRESHOLD` (default `0.8`) - A value between 0 and 1 denoting how much "confidence" is needed before an event 
  is considered spammy. Note that policyserv can currently only generate scores of 0, 0.5, and 1 - a future version will 
  allow for more precise scores, so it's recommended to keep this value as a decimal.

### Allowed senders prefilter

This "prefilter" is applied before other filters, allowing certain user IDs to bypass the remaining filters.

* `PS_SENDER_PREFILTER_ALLOWED_SENDERS` (default empty value) - The CSV-formatted user IDs which should never be filtered. 
  Set to an empty value to disable the filter.

### Allowed event types prefilter

This "prefilter" is applied before other filters, allowing certain event types to bypass the remaining filters regardless
of sender.


* `PS_EVENT_TYPE_PREFILTER_ALLOWED_EVENT_TYPES` (default `m.room.redaction`) - The CSV-formatted event types which should 
  never be filtered. This set only applies when there is a null `state_key` on the event - see the state event types set 
  for details on state events. Set to an empty value alongside the state event types list below to disable this filter.
* `PS_EVENT_TYPE_PREFILTER_ALLOWED_STATE_EVENT_TYPES` (default `m.room.power_levels,m.room.avatar,m.room.name,m.room.topic,m.room.join_rules,m.room.history_visibility,m.room.create,m.room.server_acl,m.room.tombstone,m.room.encryption,m.room.canonical_alias`) - 
  The same as the event type filter above, but only used when an event has a non-null `state_key`. Set to an empty value 
  alongside the event types list above to disable this filter.

### Hellban (timeout) filter

A "hellban" is a temporary ban which short-circuits remaining filters for a period of time after a spammy event is received.
This can be used to place users in a temporary timeout, preventing possible additional spam. The timeout applies across
an entire community.

It's called a "hellban" because it can effectively hide the cause of a filter activation from users attempting to find
the limits/edges of the overall filter configuration. It can also cause high levels of frustration in those users as they
attempt to find the timeout duration.

**Note**: we've found this filter to be a bit overbearing when combined with other filters (especially the media filter)
due to small mistakes causing disproportionately large effects to users. It's recommended to measure the effects of this
filter and balance the timeout duration against the likelihood of persistent spam. Note that policyserv does not issue 
bans for spammy events, so it's possible a spammer could outlast the timeout and evade future filters, causing disruption.

* `PS_HELLBAN_POSTFILTER_MINUTES` (default `60`) - The number of minutes to consider a sender's events as banned for, 
  regardless of other results. Set to zero or negative to disable this filter. Repeated events will not refresh the cache.
  Note that the default is 1 hour, which can be seriously disruptive to users. It's strongly recommended to set this to a
  significantly lower value when not dealing with active, persistent, spam.

**Note**: the hellban filter doesn't apply to events allowed under the allowed senders prefilter or allowed event types 
prefilter above. It also doesn't extend infinitely: the first instance of a spammy event will cause the timeout to start,
and the timeout will end after the prescribed number of minutes. This is to prevent a user's timeout becoming permanent 
if their client tries to automatically retry sending spammy events (or the user waits 4 minutes instead of 5 before
sending another event). 

### Keyword filter

The keyword filter is the most basic of the filters. If a user sends an event containing any of the listed keywords, that
event will be marked as spam.

* `PS_KEYWORD_FILTER_KEYWORDS` (default `spammy spam`) - Keywords in CSV format. Events with any one of these keywords 
  will be marked as spam. Set to an empty value to disable the filter.

### Keyword template filter

Some keyword matching sources cannot be made open source, but would ideally still be available for use within policyserv.
This filter uses Go's [`text/template`](https://pkg.go.dev/text/template) package to allow for some amount of custom 
scripting within the keyword template. If the template's goal is a simple "contains" check, the regular keyword filter 
should be used instead. This filter is intended for more complex logic being applied to keywords.

Templates are provided two variables: `BodyRaw` (`string`) and `BodyWords` (`[]string`). The template must then evaluate
to a whitespace-separated list of [MSC4387](https://github.com/matrix-org/matrix-spec-proposals/pull/4387) harms the body
(raw or words) matches. If there are no matches, the template should evaluate to an empty string.

Templates additionally have the following functions available to them:

* `StrSlice` - Creates a string slice. Usage: `{{ $slice := StrSlice "one" "two" "three" }}`
* `ToLower` - Converts a string to lowercase. Usage: `{{ .BodyRaw | ToLower }}`.
* `ToUpper` - Converts a string to uppercase. Usage: `{{ .BodyRaw | ToUpper }}`.
* `RemovePunctuation` - Removes punctuation from a string. Usage: `{{ .BodyRaw | TrimPunctuation }}`.
* `StrSliceContains` - Checks if a slice of strings contains a given value. Usage: `{{ if StrSliceContains .BodyWords "badword" }}...{{ end }}`
* `StringContains` - Checks if a string contains a given substring. Usage: `{{ if StringContains .BodyRaw "badword" }}...{{ end }}`

**Note**: this filter appends a message's `formatted_body` to its `body` to reduce the number of template executions.
This also means that the `BodyWords` will contain broken formatting after splitting the combined body and formatted body.

An example template might be:

```template
{{ badWords := StrSlice "one" "two" "three" }}
{{ range $word := .BodyWords }}
  {{ if StrSliceContains $badWords $word }}
    org.matrix.msc4387.spam
  {{ end }}
{{ end }}
```

Communities cannot upload new or custom templates with this to minimize abuse. Templates are uploaded to policyserv using
the [API](./docs/api.md).

* `PS_KEYWORD_TEMPLATE_FILTER_TEMPLATE_NAMES` (default empty value) - The CSV-formatted names of keyword templates to 
  use. If a listed filter is not found, it is skipped. Set to an empty value to disable the filter. Template names are
  set when uploading them via the policyserv API.

### Mention filter

The mentions filter looks for both plaintext and `m.mentions`-style mentions in events. If a user sends an event containing 
more than the configured number of mentions, that event will be marked as spam.

* `PS_MENTION_FILTER_MAX_MENTIONS` (default `20`) - The number of mentions to see in a single event before deciding that 
  event is spam. Set negative to disable the filter.
* `PS_MENTION_FILTER_MIN_PLAINTEXT_LENGTH` (default `5`) - The minimum length a displayname must be to be considered a 
  mention in a message.

### Many "ats" filter

Simply the number of `@` symbols allowed in an event. This is useful as a rudimentary backup to the mentions filter.

* `PS_MANY_ATS_FILTER_MAX_ATS` (default `20`) - The number of `@` symbols to see in a single event before deciding that 
  event is spam. Set negative to disable.

### Media filter

The media filter is a blunt filter to deny media (files) in rooms. 

* `PS_MEDIA_FILTER_MEDIA_TYPES` (default `m.sticker,m.image,m.video,m.file,m.audio`) - The CSV formatted event types and 
  `msgtype`s to consider spam. Set to an empty value to disable the filter.

### Untrusted media filter

This is a variant of the media filter which considers a user's relative trust before deciding whether to allow media from
them. Trust is expressed as 3 possible levels: trusted, default, and untrusted. If any configured trust source considers
the user untrusted, the user is untrusted regardless of what the other trust sources say. If no source considers the user
trusted, the user is assumed to be untrusted.

Untrusted users cannot post media to rooms.

* `PS_UNTRUSTED_MEDIA_FILTER_MEDIA_TYPES` (default `m.sticker,m.image,m.video,m.file,m.audio`) - The CSV formatted event 
  types and `msgtype`s to consider spam even if the sender is not trusted. Set to an empty value to disable. Not configuring 
  a source of trust (the `PS_UNTRUSTED_MEDIA_FILTER_USE_*` options below) will cause this filter to behave the same as 
  the media filter.
* `PS_UNTRUSTED_MEDIA_FILTER_USE_MUNINN` (default `true`) - When true, the member directory from [Muninn Hall](https://muninn-hall.com/) 
  will be trusted to send media.
* `PS_UNTRUSTED_MEDIA_FILTER_USE_POWER_LEVELS` (default `true`) - When true, users with above-default power levels in the 
  room will be trusted to send media. This includes the room creator in v12+ rooms.
* `PS_UNTRUSTED_MEDIA_FILTER_ALLOWED_USER_GLOBS` (default empty value) - The CSV-formatted globs to match against user 
  IDs of trusted users. Overridden by the deny list below. This is in addition to other trust sources.
* `PS_UNTRUSTED_MEDIA_FILTER_DENIED_USER_GLOBS` (default empty value) - The CSV-formatted globs to match against user 
  IDs of untrusted users. This is in addition to other trust sources.

Note that to use the Muninn Hall trust source, you will need to either set the API details below or use the policyserv
API to supply a member directory event manually. The following configuration variables cannot be modified or accessed
by communities:

* `PS_MUNINN_HALL_SOURCE_API_URL` (default `https://mau.bot/_matrix/maubot/plugin/muninnbot/member_directory`) - The API URL for the Muninn Hall member directory. Set to an empty value to disable automatic polling (you'll have to use the API endpoint described below instead).
* `PS_MUNINN_HALL_SOURCE_API_KEY` (default empty value) - The API key to use for the Muninn Hall member directory API. Set to an empty value to disable automatic polling.

### Policy list filter

This filter interprets a room containing [moderation policy list rules](https://spec.matrix.org/v1.16/client-server-api/#moderation-policy-lists)
to deny users and servers from sending messages in the community. Rules against rooms are ignored.

* `PS_MJOLNIR_FILTER_ENABLED` (default `true`) - When true, the policy list room ID below will be used. When false, the 
  filter is disabled.

The room ID cannot be configured by communities:

* `PS_MJOLNIR_FILTER_ROOM_ID` (default empty value) - The policy room ID to check against bot-issued bans. Must be a 
  joined room for the policy server. Set to an empty value to disable the filter.

**Note**: In a very old version of policyserv, this filter relied on Mjolnir-specific behaviour and was not compatible
with other moderation bots. This changed, but the filter configuration was not renamed. It's expected to be fixed when
different policy list room IDs can be supplied by communities.

### Density filter

Messages with a high proportion of non-whitespace characters are considered "too dense" and will be marked as spammy. For
example, `helloworld` has a density of 1.0 while `hello world` is just slightly below 1.0. 

**Note**: we've found that some permalinks to messages can activate this filter unexpectedly, leading to false positives.
If you decide to enable this filter, the density threshold may need to be adjusted relatively often.

* `PS_DENSITY_FILTER_MAX_DENSITY` (default `0.95`) - The maximum "density" a `body` may have. Calculated as 
  `length_with_whitespace_removed / original_length`. Set to negative or zero to disable.
* `PS_DENSITY_FILTER_MIN_TRIGGER_LENGTH` (default `150`) - The minimum length of the `body` before this filter checks the 
  density. This value is not used to enable/disable the filter.

### Length filters

There are two kinds of length filters: the first is a simple length check of the JSON representation of the event while
the other removes whitespace from the `body` field before imposing a maximum length. They can be configured independently.

* `PS_TRIM_FILTER_MAX_DIFFERENCE` (default `25`) - The maximum length difference between an untrimmed `body` and space-trimmed 
  `body`. Set to negative or zero to disable.
* `PS_LENGTH_FILTER_MAX_LENGTH` (default `10000`) - The maximum length an `m.room.message` event can have when represented 
  as a JSON string. Set to negative or zero to disable.

### Sticky events filter

Sticky events are events which are typically more prominent than normal messages in some clients. They are often used in
newer VoIP architectures to communicate who is in the call. Because there are limited other use cases, it is recommended
to disable sticky events in rooms which do not require VoIP functions. Note that this has no effect on pinned events - they
use a different mechanism.

* `PS_STICKY_EVENTS_FILTER_ALLOW_STICKY_EVENTS` (default `true`) - Whether to allow [MSC4354-style Sticky Events](https://github.com/matrix-org/matrix-spec-proposals/pull/4354) in rooms.

### OpenAI filter

**Note**: this filter is currently experimental and may change in future versions.

The OpenAI filter requires an [OpenAI Platform](https://platform.openai.com/) account. The underlying model is OpenAI's
[omni content moderation model](https://platform.openai.com/docs/guides/moderation), which is free to use, though we've
found that the account needs to be funded with about $10 USD first *before* the API key is created, otherwise it'll return
401/429 errors.

Current model usage is limited to text messages only. Media is not scanned by this filter. 

Future experimentation is expected to include [gpt-oss-safeguard](https://openai.com/index/introducing-gpt-oss-safeguard/)
for locally-hosted text scanning (gpt-oss-safeguard can't currently handle media).

* `PS_OPENAI_FILTER_FAIL_SECURE` (default `true`) - When `true`, the OpenAI filter will return a spam response when it 
  encounters an error from OpenAI (rate limits, etc). When `false`, the filter logs the error and returns a neutral 
  response.

Setting up the filter requires server configuration. Communities cannot change these settings:

* `PS_OPENAI_FILTER_API_KEY` (default empty value) - The API key to use for calls to OpenAI's Omni moderation model. Set 
  to an empty value to disable the filter.
* `PS_OPENAI_FILTER_ALLOWED_ROOM_IDS` (default empty value) - The CSV-formatted room IDs which are allowed to use the 
  OpenAI filter, and will be forced to use it.


### Hasher-Matcher-Actioner (HMA) filter

HMA is a tool for detecting similar content to what has already been identified. Exchanges can be used to pull hashes
from external sources, such as [NCMEC](https://www.missingkids.org/home). For more details or to set up the required
HMA instance, please see our [HMA documentation](https://github.com/matrix-org/hma-matrix).

* `PS_HMA_FILTER_ENABLED_BANKS` (default empty value) - The CSV-formatted HMA bank names to run media through. Set to an 
  empty value to disable the HMA filter.

Server-side configuration cannot be changed by communities:

* `PS_HMA_API_URL` (default empty value) - The base API URL for your [HMA](https://github.com/matrix-org/hma-matrix) instance. Set to an empty value to disable 
  the HMA filter.
* `PS_HMA_API_KEY` (default empty value) - The API key for your HMA instance.

## Contributing

We're always happy to accept new features and bug fixes! Please see our [contributing guide](./CONTRIBUTING.md) for more 
details.

## API

See the [API documentation](./docs/api.md) for more details.

## Versioning / release schedule

We aim to release new versions 1-2 weeks after commit activity. The [releases](https://github.com/matrix-org/policyserv/releases)
page shows changes between versions. It's recommended to update promptly following a release.

We use [Pride Versioning v0.3.0](https://pridever.org/) to release policyserv. We extend this to include `-rc.1` tags to 
the end to indicate prereleases. Prereleases are not intended for production use, but do help us identify bugs before they 
become shame version bumps. We aim to minimize shame version bumps.
