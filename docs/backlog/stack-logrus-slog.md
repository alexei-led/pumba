# logrus to log/slog

Tags: stack · Effort: M (mechanical, ~40 files)

## Approach
One PR; map `--log-level` and `--json` onto slog handlers. Do after the webhook change removes the slackrus hook.

## Open questions
- Keep logrus-style field names for existing log consumers.
