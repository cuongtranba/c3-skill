package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lagz0ne/c3-design/cli/internal/toon"
)

const defaultTruncateLen = 1500

func isAgentMode() bool {
	return os.Getenv("C3X_MODE") == "agent"
}

// writeJSON is the structured-output path for commands that render their own
// payload. It is a --json path by construction, so ResolveFormat is asked with
// jsonExplicit=true: agent mode still downgrades to TOON exactly as before, and
// an explicit --format outranks both.
func writeJSON(w io.Writer, v any) error {
	switch ResolveFormat(true, isAgentMode()) {
	case FormatText:
		out, err := toon.MarshalAnyText(v)
		if err != nil {
			return err
		}
		fmt.Fprint(w, out)
		return nil
	case FormatJSON:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	default:
		out, err := toon.MarshalAny(v)
		if err != nil {
			return err
		}
		fmt.Fprint(w, out)
		return nil
	}
}
