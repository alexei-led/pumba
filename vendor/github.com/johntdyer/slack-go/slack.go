package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client struct {
	Url string
}

type Message struct {
	Text        string        `json:"text"`
	Username    string        `json:"username"`
	IconUrl     string        `json:"icon_url"`
	IconEmoji   string        `json:"icon_emoji"`
	Channel     string        `json:"channel"`
	UnfurlLinks bool          `json:"unfurl_links"`
	Attachments []*Attachment `json:"attachments"`
}

type Attachment struct {
	Title    string   `json:"title"`
	Fallback string   `json:"fallback"`
	Text     string   `json:"text"`
	Pretext  string   `json:"pretext"`
	Color    string   `json:"color"`
	Fields   []*Field `json:"fields"`
}

type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type SlackError struct {
	Code int
	Body string
}

func (e *SlackError) Error() string {
	return fmt.Sprintf("SlackError: %d %s", e.Code, e.Body)
}

func NewClient(url string) *Client {
	return &Client{url}
}

func (c *Client) SendMessage(msg *Message) error {

	body, _ := json.Marshal(msg)
	buf := bytes.NewReader(body)

	http.NewRequest("POST", c.Url, buf)
	resp, err := http.Post(c.Url, "application/json", buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t, _ := ioutil.ReadAll(resp.Body)
		return &SlackError{resp.StatusCode, string(t)}
	}

	return nil
}

func (m *Message) NewAttachment() *Attachment {
	a := &Attachment{}
	m.AddAttachment(a)
	return a
}

func (m *Message) AddAttachment(a *Attachment) {
	m.Attachments = append(m.Attachments, a)
}

func (a *Attachment) NewField() *Field {
	f := &Field{}
	a.AddField(f)
	return f
}

func (a *Attachment) AddField(f *Field) {
	a.Fields = append(a.Fields, f)
}
