# Generic --webhook events

Tags: stack · Effort: S

## Status

The slackrus dependency is gone. An internal Slack-compatible hook preserves the
existing `--slackhook` and `--slackchannel` flags. Generic event delivery remains
unimplemented.

## Approach

Generic `--webhook URL` posting the `pumba.event/v1` result events
([docs/agents.md](../agents.md)), with a Slack-compatible
payload option. Keep `--slackhook` as a deprecated alias for one release.

## Open questions

- Retry/timeout policy for generic webhook delivery.
