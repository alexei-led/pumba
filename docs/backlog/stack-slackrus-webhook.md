# Slack-format result notifications (deferred)

Tags: stack · Effort: S

Generic `--webhook` delivery of `pumba.event/v1` events has shipped; see the
[webhook contract](../agents.md#webhook). The existing `--slackhook` and
`--slackchannel` log hooks remain supported and are not deprecated.

Deferred idea: a Slack-compatible payload option for result notifications.
No Slack formatting is implemented for `--webhook`, and no follow-up release is committed.
