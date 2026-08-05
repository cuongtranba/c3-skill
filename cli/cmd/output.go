package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/toon"
)

// OutputFormat determines the serialization format.
type OutputFormat int

const (
	FormatJSON  OutputFormat = iota // --json explicit outside agent mode
	FormatTOON                      // default structured machine output
	FormatHuman                     // command-specific human prose paths
	FormatText                      // --format text: TOON shape, multi-line values as block scalars
)

// outputFormatFlag holds the process-wide --format selector, published by
// ParseArgs. Like C3X_MODE it is one choice per invocation rather than per
// command, so ResolveFormat reads it here instead of threading a field through
// every command's own Options struct.
var outputFormatFlag string

func setOutputFormatFlag(v string) { outputFormatFlag = v }

// formatFlagValues are the accepted --format values. "mermaid" is graph-only
// (cmd/graph.go owns that render path); ResolveFormat deliberately ignores it so
// every other command keeps its normal default.
var formatFlagValues = []string{"text", "toon", "json", "mermaid"}

// ValidateFormatFlag rejects an unknown --format value up front, so a typo fails
// loudly instead of silently falling back to the default format.
func ValidateFormatFlag(v string) error {
	if v == "" {
		return nil
	}
	for _, valid := range formatFlagValues {
		if v == valid {
			return nil
		}
	}
	return fmt.Errorf("error: unknown --format %q\nhint: valid values are %s (mermaid is graph-only)",
		v, strings.Join(formatFlagValues, ", "))
}

// ResolveFormat determines output format from options and environment.
// Precedence: explicit --format > --json > C3X_MODE=agent default.
func ResolveFormat(jsonExplicit bool, agent bool) OutputFormat {
	// An explicit --format is a deliberate render choice and outranks both --json
	// and the agent default — the same rule graph already applies to --format mermaid.
	switch outputFormatFlag {
	case "text":
		return FormatText
	case "toon":
		return FormatTOON
	case "json":
		return FormatJSON
	}
	if agent {
		return FormatTOON
	}
	if jsonExplicit {
		return FormatJSON
	}
	return FormatTOON
}

// HelpHint is a single next-step suggestion.
type HelpHint struct {
	Command     string `json:"command"`     // e.g. "c3x read <id>"
	Description string `json:"description"` // e.g. "read entity content"
}

func (h HelpHint) String() string {
	return h.Command
}

// WriteTableOutput writes a tabular dataset with optional help hints.
// JSON is explicit compatibility, --format text keeps real newlines; everything
// else uses TOON.
func WriteTableOutput(w io.Writer, label string, data any, fields []string, format OutputFormat, hints []HelpHint) error {
	switch format {
	case FormatJSON:
		if err := writeJSON(w, data); err != nil {
			return err
		}
		writeHints(w, hints)
		return nil
	case FormatText:
		out, err := toon.MarshalTableText(label, data, fields)
		if err != nil {
			return err
		}
		fmt.Fprint(w, out)
		writeHints(w, hints)
		return nil
	default:
		out, err := toon.MarshalTable(label, data, fields)
		if err != nil {
			return err
		}
		fmt.Fprint(w, out)
		writeHints(w, hints)
		return nil
	}
}

// WriteObjectOutput writes a single object with optional help hints.
// JSON is explicit compatibility, --format text keeps real newlines; everything
// else uses TOON.
func WriteObjectOutput(w io.Writer, data any, format OutputFormat, hints []HelpHint) error {
	switch format {
	case FormatJSON:
		if err := writeJSON(w, data); err != nil {
			return err
		}
		writeHints(w, hints)
		return nil
	case FormatText:
		out, err := toon.MarshalObjectText(data)
		if err != nil {
			return err
		}
		fmt.Fprint(w, out)
		writeHints(w, hints)
		return nil
	default:
		out, err := toon.MarshalObject(data)
		if err != nil {
			return err
		}
		fmt.Fprint(w, out)
		writeHints(w, hints)
		return nil
	}
}

// FormatHelpHints renders help[] lines.
func FormatHelpHints(hints []HelpHint) string {
	if len(hints) == 0 {
		return ""
	}
	commands := make([]string, 0, len(hints))
	for _, h := range hints {
		if strings.TrimSpace(h.Command) != "" {
			commands = append(commands, h.Command)
		}
	}
	if len(commands) == 0 {
		return ""
	}
	return fmt.Sprintf("help[%d]: %s\n", len(commands), strings.Join(commands, " | "))
}

func writeHints(w io.Writer, hints []HelpHint) {
	if !isAgentMode() || len(hints) == 0 {
		return
	}
	fmt.Fprint(w, FormatHelpHints(hints))
}
