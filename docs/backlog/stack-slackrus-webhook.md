# Replace slackrus with generic --webhook

Tags: stack · Effort: S

## Why
`johntdyer/slackrus` is a 2023 pseudo-version, effectively unmaintained.

## Approach
Generic `--webhook URL` posting the JSON event from [agent-structured-output](agent-structured-output.md); Slack-compatible payload option. Keep `--slackhook` as deprecated alias for one release.

## Open questions
- Retry/timeout policy.
