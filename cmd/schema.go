package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/urfave/cli"
)

const (
	// schemaCommandName names the command before() lets run without a runtime.
	schemaCommandName = "schema"
	// cliSchema versions the `pumba schema` document. Within v1, fields may
	// be added; renaming, removing or re-meaning a field bumps the version.
	cliSchema = "pumba.cli/v1"
)

// cliSchemaDoc is the pumba.cli/v1 document: the CLI surface, walked from the
// live app, plus the result event vocabulary agents branch on.
type cliSchemaDoc struct {
	Schema         string                 `json:"schema"`
	PumbaVersion   string                 `json:"pumba_version"`
	EventSchema    string                 `json:"event_schema"`
	ErrorCodes     []chaos.ErrorCode      `json:"error_codes"`
	RunStatuses    []chaos.RunStatus      `json:"run_statuses"`
	TargetStatuses []chaos.TargetStatus   `json:"target_statuses"`
	Usage          string                 `json:"usage"`
	ArgsUsage      string                 `json:"args_usage"`
	GlobalFlags    []cliflags.FlagInfo    `json:"global_flags"`
	Commands       []cliflags.CommandInfo `json:"commands"`
}

// newSchemaCommand returns the `schema` command. It writes to stdout, not
// app.Writer, so the document lands on stdout in every --output mode.
func newSchemaCommand(stdout io.Writer) cli.Command {
	return cli.Command{
		Name:        schemaCommandName,
		Usage:       "print the CLI and result event schema as JSON (" + cliSchema + ")",
		Description: "describe global flags, the command tree with every flag's type, default and env vars, and the " + chaos.EventSchema + " error codes and statuses; needs no container runtime",
		Action: func(c *cli.Context) error {
			return writeSchema(stdout, c.App)
		},
	}
}

func describeApp(app *cli.App) (*cliSchemaDoc, error) {
	globals, err := cliflags.DescribeFlagsV1(app.Flags)
	if err != nil {
		return nil, fmt.Errorf("global flags: %w", err)
	}
	cmds, err := cliflags.DescribeCommandsV1(app.Commands, "")
	if err != nil {
		return nil, err
	}
	return &cliSchemaDoc{
		Schema:         cliSchema,
		PumbaVersion:   version,
		EventSchema:    chaos.EventSchema,
		ErrorCodes:     chaos.ErrorCodes(),
		RunStatuses:    chaos.RunStatuses(),
		TargetStatuses: chaos.TargetStatuses(),
		Usage:          app.Usage,
		ArgsUsage:      app.ArgsUsage,
		GlobalFlags:    globals,
		Commands:       cmds,
	}, nil
}

func writeSchema(w io.Writer, app *cli.App) error {
	doc, err := describeApp(app)
	if err != nil {
		return fmt.Errorf("describe CLI: %w", err)
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("write schema: %w", err)
	}
	return nil
}
