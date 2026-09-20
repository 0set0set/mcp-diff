package output

import (
	"encoding/json"
	"io"

	"github.com/0set0set/mcp-diff/internal/diff"
)

type jsonDocument struct {
	SchemaVersion int          `json:"schema_version"`
	Format        string       `json:"format"`
	Servers       []jsonServer `json:"servers"`
	Summary       jsonSummary  `json:"summary"`
}

type jsonServer struct {
	Name      string       `json:"name"`
	Status    diff.Status  `json:"status"`
	Transport string       `json:"transport"`
	Changes   []jsonChange `json:"changes"`
}

type jsonChange struct {
	Path string      `json:"path"`
	Op   diff.Status `json:"op"`
	Old  string      `json:"old,omitempty"`
	New  string      `json:"new,omitempty"`
}

type jsonSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
}

func WriteJSON(w io.Writer, result diff.Result) error {
	servers := make([]jsonServer, 0, len(result.Servers))
	for _, server := range result.Servers {
		changes := make([]jsonChange, 0, len(server.Changes))
		for _, change := range server.Changes {
			changes = append(changes, jsonChange{
				Path: change.Path,
				Op:   change.Op,
				Old:  change.Old,
				New:  change.New,
			})
		}
		servers = append(servers, jsonServer{
			Name:      server.Name,
			Status:    server.Status,
			Transport: server.Transport,
			Changes:   changes,
		})
	}

	document := jsonDocument{
		SchemaVersion: 1,
		Format:        string(result.Format),
		Servers:       servers,
		Summary: jsonSummary{
			Added:   result.Added,
			Removed: result.Removed,
			Changed: result.Changed,
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}
