# To-device Messaging Protocol

When a policyserv community sets their `moderation_bot_user_id` configuration option, policyserv will send to-device messages
to that user ID for some actions. The user doesn't *need* to be a moderation bot, but it typically is.

All to-device messages sent by policyserv are "fire and forget", meaning policyserv does not wait for a success or error
response. It is the responsibility of the receiving user to handle retries (if they want to) or to ignore messages that
were sent a while ago (also if they want to).

## Message format

The general format for a to-device message sent by policyserv is:

```json5
{
  "type": "org.matrix.policyserv.command",
  "content": {
    "command": "<whatever>",
    "room_id": "<room ID>", // the room where this command applies
    // ... other fields as required ...
    "signatures": {
      "policy.example.org": {
        "ed25519:policy_server": "<unpadded base64 signature>"
      }
    }
  }
}
```

The signature covers the entire `content` field, using normal [Matrix signing](https://spec.matrix.org/v1.18/appendices/#signing-json).
The public key used to sign the message will be the one stored in the [`m.room.policy`](https://spec.matrix.org/v1.18/client-server-api/#mroompolicy)
state event in the protected room.

Receivers MUST verify the signature before processing the command. If *any* of the signatures are invalid, or if the room's
policy server has *not* signed the message, the message MUST be ignored (dropped).

## Commands

Currently, policyserv only supports the `redact` command.

### Redaction

Policyserv sends this command when it believes a spammy event has leaked into a room and should be redacted as a result.

Example content:

```json5
{
  "command": "redact",
  "room_id": "!room:example.org", // the room where the event resides
  "event_id": "$event", // the event to redact in the room
  "signatures": { /* ... */ }
}
```
