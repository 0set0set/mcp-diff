package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/0set0set/mcp-diff/internal/diff"
)

const separator = "--------------------------------"

func WriteText(w io.Writer, result diff.Result) error {
	if !result.HasChanges() {
		_, err := fmt.Fprintln(w, "No MCP configuration changes detected.")
		return err
	}

	var buffer strings.Builder
	fmt.Fprintf(&buffer, "MCP configuration diff (%s)\n%s\n\n", result.Format, separator)

	for _, server := range result.Servers {
		fmt.Fprintln(&buffer, serverHeading(server))
		for _, change := range server.Changes {
			switch change.Op {
			case diff.StatusAdded:
				fmt.Fprintf(&buffer, "    %s: + %s\n", change.Path, change.New)
			case diff.StatusRemoved:
				fmt.Fprintf(&buffer, "    %s: - %s\n", change.Path, change.Old)
			default:
				fmt.Fprintf(&buffer, "    %s: %s -> %s\n", change.Path, change.Old, change.New)
			}
		}
		fmt.Fprintln(&buffer)
	}

	fmt.Fprintf(
		&buffer,
		"%s\n%d %s added, %d removed, %d changed\n",
		separator,
		result.Added,
		plural("server", result.Added),
		result.Removed,
		result.Changed,
	)
	_, err := io.WriteString(w, buffer.String())
	return err
}

func serverHeading(server diff.ServerDiff) string {
	switch server.Status {
	case diff.StatusAdded:
		return fmt.Sprintf("+ %s (%s)", server.Name, server.Transport)
	case diff.StatusRemoved:
		return fmt.Sprintf("- %s (%s)", server.Name, server.Transport)
	default:
		return "~ " + server.Name
	}
}

func plural(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
