package diff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0set0set/mcp-diff/internal/config"
)

type Status string

const (
	StatusAdded   Status = "added"
	StatusRemoved Status = "removed"
	StatusChanged Status = "changed"
)

type Result struct {
	Format  config.Format
	Servers []ServerDiff
	Added   int
	Removed int
	Changed int
}

type ServerDiff struct {
	Name      string
	Status    Status
	Transport string
	Changes   []Change
}

type Change struct {
	Path string
	Op   Status
	Old  string
	New  string
}

func (result Result) HasChanges() bool {
	return len(result.Servers) > 0
}

func Compute(before, after config.Snapshot, format config.Format) Result {
	result := Result{Format: format, Servers: []ServerDiff{}}

	names := make(map[string]struct{}, len(before)+len(after))
	for name := range before {
		names[name] = struct{}{}
	}
	for name := range after {
		names[name] = struct{}{}
	}

	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		beforeServer, existedBefore := before[name]
		afterServer, existsAfter := after[name]

		server := ServerDiff{
			Name:    name,
			Changes: []Change{},
		}

		switch {
		case !existedBefore:
			server.Status = StatusAdded
			server.Transport = afterServer.Transport
			server.Changes = describeServer(afterServer, StatusAdded)
			result.Added++
		case !existsAfter:
			server.Status = StatusRemoved
			server.Transport = beforeServer.Transport
			server.Changes = describeServer(beforeServer, StatusRemoved)
			result.Removed++
		default:
			server.Status = StatusChanged
			server.Transport = afterServer.Transport
			server.Changes = compareServers(beforeServer, afterServer)
			if len(server.Changes) == 0 {
				continue
			}
			result.Changed++
		}

		result.Servers = append(result.Servers, server)
	}

	return result
}

func compareServers(before, after config.Server) []Change {
	changes := make([]Change, 0)
	changes = compareScalar(changes, "transport", before.Transport, after.Transport)
	changes = compareScalar(changes, "command", before.Command, after.Command)
	changes = compareScalar(changes, "url", before.URL, after.URL)
	changes = compareScalar(changes, "headersHelper", before.Helper, after.Helper)
	changes = compareArgs(changes, before.Args, after.Args)
	changes = compareSet(changes, "includeTools", before.Include, after.Include)
	changes = compareSet(changes, "excludeTools", before.Exclude, after.Exclude)
	changes = compareValues(changes, "env", before.Env, after.Env)
	changes = compareValues(changes, "headers", before.Headers, after.Headers)

	flagNames := unionKeys(before.Flags, after.Flags)
	for _, name := range flagNames {
		if name == "oauth.scopes" {
			changes = compareSet(
				changes,
				name,
				strings.Fields(before.Flags[name]),
				strings.Fields(after.Flags[name]),
			)
			continue
		}
		changes = compareScalar(changes, name, before.Flags[name], after.Flags[name])
	}

	sortChanges(changes)
	return changes
}

func describeServer(server config.Server, op Status) []Change {
	changes := make([]Change, 0)
	changes = appendValue(changes, "command", op, server.Command)
	for index, value := range server.Args {
		changes = append(changes, changeFor(op, fmt.Sprintf("args[%d]", index), value))
	}
	changes = appendValue(changes, "url", op, server.URL)
	changes = appendValue(changes, "headersHelper", op, server.Helper)
	for _, value := range server.Include {
		changes = append(changes, changeFor(op, "includeTools", value))
	}
	for _, value := range server.Exclude {
		changes = append(changes, changeFor(op, "excludeTools", value))
	}
	for _, name := range sortedValueKeys(server.Env) {
		changes = append(changes, changeFor(op, "env."+name, valueLabel(server.Env[name])))
	}
	for _, name := range sortedValueKeys(server.Headers) {
		changes = append(changes, changeFor(op, "headers."+name, valueLabel(server.Headers[name])))
	}
	for _, name := range sortedStringKeys(server.Flags) {
		if name == "oauth.scopes" {
			for _, scope := range strings.Fields(server.Flags[name]) {
				changes = append(changes, changeFor(op, name, scope))
			}
			continue
		}
		changes = appendValue(changes, name, op, server.Flags[name])
	}
	sortChanges(changes)
	return changes
}

func compareScalar(changes []Change, path, before, after string) []Change {
	if before == after {
		return changes
	}
	return append(changes, Change{
		Path: path,
		Op:   StatusChanged,
		Old:  displayValue(before),
		New:  displayValue(after),
	})
}

func compareArgs(changes []Change, before, after []string) []Change {
	maximum := max(len(before), len(after))
	for index := 0; index < maximum; index++ {
		path := fmt.Sprintf("args[%d]", index)
		switch {
		case index >= len(before):
			changes = append(changes, changeFor(StatusAdded, path, after[index]))
		case index >= len(after):
			changes = append(changes, changeFor(StatusRemoved, path, before[index]))
		case before[index] != after[index]:
			changes = append(changes, Change{
				Path: path,
				Op:   StatusChanged,
				Old:  before[index],
				New:  after[index],
			})
		}
	}
	return changes
}

func compareSet(changes []Change, path string, before, after []string) []Change {
	beforeSet := stringSet(before)
	afterSet := stringSet(after)
	for _, value := range sortedSetDifference(afterSet, beforeSet) {
		changes = append(changes, changeFor(StatusAdded, path, value))
	}
	for _, value := range sortedSetDifference(beforeSet, afterSet) {
		changes = append(changes, changeFor(StatusRemoved, path, value))
	}
	return changes
}

func compareValues(
	changes []Change,
	path string,
	before, after map[string]config.Value,
) []Change {
	for _, name := range unionValueKeys(before, after) {
		beforeValue, existedBefore := before[name]
		afterValue, existsAfter := after[name]
		field := path + "." + name
		switch {
		case !existedBefore:
			changes = append(changes, changeFor(StatusAdded, field, valueLabel(afterValue)))
		case !existsAfter:
			changes = append(changes, changeFor(StatusRemoved, field, valueLabel(beforeValue)))
		case beforeValue != afterValue:
			changes = append(changes, Change{
				Path: field,
				Op:   StatusChanged,
				Old:  valueLabel(beforeValue),
				New:  valueLabel(afterValue),
			})
		}
	}
	return changes
}

func appendValue(changes []Change, path string, op Status, value string) []Change {
	if value == "" {
		return changes
	}
	return append(changes, changeFor(op, path, value))
}

func changeFor(op Status, path, value string) Change {
	change := Change{Path: path, Op: op}
	if op == StatusAdded {
		change.New = value
	} else {
		change.Old = value
	}
	return change
}

func displayValue(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

func valueLabel(value config.Value) string {
	var label string
	switch value.Kind {
	case config.ValueInput:
		label = "input reference"
	case config.ValueReference:
		label = "environment reference"
	default:
		return "literal value"
	}
	if value.Ref != "" {
		return fmt.Sprintf("%s (%s)", label, value.Ref)
	}
	return label
}

func sortChanges(changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Op < changes[j].Op
	})
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func sortedSetDifference(left, right map[string]struct{}) []string {
	result := make([]string, 0)
	for value := range left {
		if _, exists := right[value]; !exists {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func unionKeys(left, right map[string]string) []string {
	keys := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		keys[key] = struct{}{}
	}
	for key := range right {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

func unionValueKeys(left, right map[string]config.Value) []string {
	keys := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		keys[key] = struct{}{}
	}
	for key := range right {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

func sortedValueKeys(values map[string]config.Value) []string {
	keys := make(map[string]struct{}, len(values))
	for key := range values {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

func sortedStringKeys(values map[string]string) []string {
	keys := make(map[string]struct{}, len(values))
	for key := range values {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

func sortedKeys(keys map[string]struct{}) []string {
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
