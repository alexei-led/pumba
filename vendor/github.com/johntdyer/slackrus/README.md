slackrus
========

Slack hook for [Logrus](https://github.com/sirupsen/logrus). 
>>>>>>> Fix import path

## Use

```go
package main

import (
	logrus "github.com/sirupsen/logrus"
	"github.com/johntdyer/slackrus"
	"os"
)

func main() {

	logrus.SetFormatter(&logrus.JSONFormatter{})

	logrus.SetOutput(os.Stderr)

	logrus.SetLevel(logrus.DebugLevel)
	
	logrus.AddHook(&slackrus.SlackrusHook{
		HookURL:        "https://hooks.slack.com/services/abc123/defghijklmnopqrstuvwxyz",
		AcceptedLevels: slackrus.LevelThreshold(logrus.DebugLevel),
		Channel:        "#slack-testing",
		IconEmoji:      ":ghost:",
		Username:       "foobot",
	})

	logrus.Warn("warn")
	logrus.Info("info")
	logrus.Debug("debug")
}

```

### Extra fields
You can also add some extra fields to be sent with every slack message
```go
extra := map[string]interface{}{
			"hostname": "nyc-server-1",
			"tag": "some-tag",
		}
	
logrus.AddHook(&slackrus.SlackrusHook{
		//HookURL:        "https://hooks.slack.com/services/abc123/defghijklmnopqrstuvwxyz",
		Extra: 			extra,
})
```

## Parameters

#### Required
  * HookURL

#### Optional
  * IconEmoji
  * IconURL
  * Username
  * Channel
  * Asynchronous
  * Extra
## Installation

    go get github.com/johntdyer/slackrus

## Credits

Based on hipchat handler by [nuboLAB](https://github.com/nubo/hiprus)
