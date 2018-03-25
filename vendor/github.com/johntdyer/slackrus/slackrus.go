// Package slackrus provides a Hipchat hook for the logrus loggin package.
package slackrus

import (
	"fmt"

	"github.com/johntdyer/slack-go"
	"github.com/sirupsen/logrus"
)

// Project version
const (
	VERISON = "0.0.3"
)

// SlackrusHook is a logrus Hook for dispatching messages to the specified
// channel on Slack.
type SlackrusHook struct {
	// Messages with a log level not contained in this array
	// will not be dispatched. If nil, all messages will be dispatched.
	AcceptedLevels []logrus.Level
	HookURL        string
	IconURL        string
	Channel        string
	IconEmoji      string
	Username       string
	Asynchronous   bool
	Extra          map[string]interface{}
}

// Levels sets which levels to sent to slack
func (sh *SlackrusHook) Levels() []logrus.Level {
	if sh.AcceptedLevels == nil {
		return AllLevels
	}
	return sh.AcceptedLevels
}

// Fire -  Sent event to slack
func (sh *SlackrusHook) Fire(e *logrus.Entry) error {
	color := ""
	switch e.Level {
	case logrus.DebugLevel:
		color = "#9B30FF"
	case logrus.InfoLevel:
		color = "good"
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		color = "danger"
	default:
		color = "warning"
	}

	msg := &slack.Message{
		Username:  sh.Username,
		Channel:   sh.Channel,
		IconEmoji: sh.IconEmoji,
		IconUrl:   sh.IconURL,
	}

	attach := msg.NewAttachment()

	newEntry := sh.newEntry(e)
	// If there are fields we need to render them at attachments
	if len(newEntry.Data) > 0 {

		// Add a header above field data
		attach.Text = "Message fields"

		for k, v := range newEntry.Data {
			slackField := &slack.Field{}

			slackField.Title = k
			slackField.Value = fmt.Sprint(v)
			// If the field is <= 20 then we'll set it to short
			if len(slackField.Value) <= 20 {
				slackField.Short = true
			}

			attach.AddField(slackField)
		}
		attach.Pretext = newEntry.Message
	} else {
		attach.Text = newEntry.Message
	}
	attach.Fallback = newEntry.Message
	attach.Color = color

	c := slack.NewClient(sh.HookURL)

	if sh.Asynchronous {
		go c.SendMessage(msg)
		return nil
	}

	return c.SendMessage(msg)
}

func (sh *SlackrusHook) newEntry(entry *logrus.Entry) *logrus.Entry {
	data := map[string]interface{}{}

	for k, v := range sh.Extra {
		data[k] = v
	}
	for k, v := range entry.Data {
		data[k] = v
	}

	newEntry := &logrus.Entry{
		Logger:  entry.Logger,
		Data:    data,
		Time:    entry.Time,
		Level:   entry.Level,
		Message: entry.Message,
	}

	return newEntry
}
