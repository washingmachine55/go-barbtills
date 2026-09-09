package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// The --json contract, shared by every command that honours the flag.
//
// A command's result is encoded to stdout as exactly one JSON value, so
// `barbtils bst -fm 16291s -j | jq` works with no stream redirection. The
// logger is left where it belongs, on stderr: routing it to stdout instead
// would corrupt the document the moment --debug adds a line, and would wrap
// every payload in the time/level/msg envelope a consumer then has to unwrap.
//
// A payload carries values, never a rendered table: column padding belongs to
// the styled output, and a padded string in a JSON field is only work for the
// consumer to undo.

// emitJSON writes one JSON document to stdout. Indented, since jq does not care
// and a bare `barbtils tasks ls -j` is still meant to be readable.
func emitJSON(v any) error {
	return encodeJSON(os.Stdout, v)
}

func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// errJSONUnsupported is what an interactive command answers when asked for JSON:
// a TUI has no serialisable output, and silently ignoring the flag would leave a
// script waiting forever on a full-screen program.
func errJSONUnsupported(what string) error {
	return fmt.Errorf("--json has no output for %s — use the non-interactive commands (tasks ls, tasks show) instead", what)
}
