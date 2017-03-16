package slack

import (
	"flag"
	"os/user"
	"testing"
)

var (
	client *Client
)

func init() {
	client = &Client{}
	flag.StringVar(&client.Url, "url", "", "webhook url")
	flag.Parse()
	if client.Url == "" {
		flag.PrintDefaults()
		panic("\n=================\nYou need to specify -url flag\n=================\n\n")
	}
}

func TestSendMessage(t *testing.T) {
	msg := &Message{}
	msg.Channel = "#slack-go-test"
	msg.Text = "Slack API Test from go"
	user, _ := user.Current()
	msg.Username = user.Username
	client.SendMessage(msg)
}

func TestSendMessageWithAttachement(t *testing.T) {
	msg := &Message{}
	msg.Channel = "#slack-go-test"
	msg.Text = "Slack API Test from go - with attachment"
	user, _ := user.Current()
	msg.Username = user.Username

	attach := msg.NewAttachment()
	attach.Text = "This is an attachment!"
	attach.Pretext = "This is the pretext of an attachment"
	attach.Color = "good"
	attach.Fallback = "That's the fallback field"

	field := attach.NewField()
	field.Title = "Field one"
	field.Value = "Field one value"

	client.SendMessage(msg)
}
