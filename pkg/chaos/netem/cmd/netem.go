// Package cmd wires netem subcommands to the generic NewAction[P] CLI builder.
// Per-action parsers delegate netem-level base-flag parsing to
// netem.ParseRequestBase, leaving each parser responsible only for its own
// action-specific flags.
package cmd
