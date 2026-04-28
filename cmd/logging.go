package main

import (
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
)

// setupLogging configures the global logrus logger from --log-level, --json,
// --slackhook, and --slackchannel global flags. Called once from before().
func setupLogging(f cliflags.Flags) {
	switch level := f.String("log-level"); level {
	case "debug", "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "info", "INFO":
		log.SetLevel(log.InfoLevel)
	case "warning", "WARNING":
		log.SetLevel(log.WarnLevel)
	case "error", "ERROR":
		log.SetLevel(log.ErrorLevel)
	case "fatal", "FATAL":
		log.SetLevel(log.FatalLevel)
	case "panic", "PANIC":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.WarnLevel)
	}
	if f.Bool("json") {
		log.SetFormatter(&log.JSONFormatter{})
	}
	if f.String("slackhook") != "" {
		log.AddHook(&slackrus.SlackrusHook{
			HookURL:        f.String("slackhook"),
			AcceptedLevels: slackrus.LevelThreshold(log.GetLevel()),
			Channel:        f.String("slackchannel"),
			IconEmoji:      ":boar:",
			Username:       "pumba_bot",
		})
	}
}
