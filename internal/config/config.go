package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type Format string

const (
	FormatAuto   Format = "auto"
	FormatClaude Format = "claude"
	FormatCursor Format = "cursor"
	FormatVSCode Format = "vscode"
	FormatGemini Format = "gemini"
)

type ValueKind string

const (
	ValueLiteral   ValueKind = "literal"
	ValueReference ValueKind = "reference"
	ValueInput     ValueKind = "input"
)

type Value struct {
	Kind ValueKind
	Ref  string
}

type Server struct {
	Transport string
	Command   string
	Args      []string
	URL       string
	Env       map[string]Value
	Headers   map[string]Value
	Helper    string
	Include   []string
	Exclude   []string
	Flags     map[string]string
}

type Snapshot map[string]Server

func ParseFormat(value string) (Format, error) {
	format := Format(value)
	switch format {
	case FormatAuto, FormatClaude, FormatCursor, FormatVSCode, FormatGemini:
		return format, nil
	default:
		return "", fmt.Errorf("unknown format %q", value)
	}
}

func Detect(data []byte) (Format, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	if root == nil {
		return "", fmt.Errorf("document must be a JSON object")
	}

	if _, ok := root["servers"]; ok {
		return FormatVSCode, nil
	}
	serversJSON, ok := root["mcpServers"]
	if !ok {
		return "", fmt.Errorf("document must contain mcpServers or servers")
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(serversJSON, &servers); err != nil || servers == nil {
		return FormatClaude, nil
	}

	format := FormatClaude
	for _, serverJSON := range servers {
		var server map[string]json.RawMessage
		if json.Unmarshal(serverJSON, &server) != nil {
			continue
		}
		if hasAny(server, "includeTools", "excludeTools", "trust", "httpUrl") {
			return FormatGemini, nil
		}
		if hasAny(server, "envFile", "auth") || rawContains(serverJSON, "${env:") {
			format = FormatCursor
		}
	}
	return format, nil
}

func Load(data []byte, format Format) (Snapshot, error) {
	if format == FormatAuto {
		detected, err := Detect(data)
		if err != nil {
			return nil, err
		}
		format = detected
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("document must be a JSON object")
	}

	rootKey := "mcpServers"
	if format == FormatVSCode {
		rootKey = "servers"
	}
	serversJSON, ok := root[rootKey]
	if !ok {
		return nil, fmt.Errorf("%s is required for format %s", rootKey, format)
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(serversJSON, &servers); err != nil || servers == nil {
		if err != nil {
			return nil, fmt.Errorf("%s must be an object: %w", rootKey, err)
		}
		return nil, fmt.Errorf("%s must be an object", rootKey)
	}

	snapshot := make(Snapshot, len(servers))
	for name, serverJSON := range servers {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("server name must not be empty or whitespace")
		}

		server, err := parseServer(rootKey, name, serverJSON, format)
		if err != nil {
			return nil, err
		}
		snapshot[name] = server
	}

	return snapshot, nil
}

func parseServer(rootKey, name string, data json.RawMessage, format Format) (Server, error) {
	var raw map[string]json.RawMessage
	path := rootKey + "." + name
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		if err != nil {
			return Server{}, fmt.Errorf("%s must be an object: %w", path, err)
		}
		return Server{}, fmt.Errorf("%s must be an object", path)
	}

	command, err := optionalString(raw, "command", path)
	if err != nil {
		return Server{}, err
	}
	args, err := optionalStrings(raw, "args", path)
	if err != nil {
		return Server{}, err
	}
	for index := range args {
		args[index] = MaskURL(args[index])
	}
	serverURL, err := optionalString(raw, "url", path)
	if err != nil {
		return Server{}, err
	}
	if serverURL == "" {
		serverURL, err = optionalString(raw, "httpUrl", path)
		if err != nil {
			return Server{}, err
		}
	}
	serverURL = MaskURL(serverURL)
	env, err := optionalValues(raw, "env", path, format)
	if err != nil {
		return Server{}, err
	}
	headers, err := optionalValues(raw, "headers", path, format)
	if err != nil {
		return Server{}, err
	}
	helper, err := optionalString(raw, "headersHelper", path)
	if err != nil {
		return Server{}, err
	}
	include, err := optionalStrings(raw, "includeTools", path)
	if err != nil {
		return Server{}, err
	}
	exclude, err := optionalStrings(raw, "excludeTools", path)
	if err != nil {
		return Server{}, err
	}

	flags := make(map[string]string)
	for _, key := range []string{"trust", "alwaysLoad", "sandboxEnabled", "timeout", "cwd", "envFile"} {
		if value, ok := raw[key]; ok {
			scalar, err := scalarString(value)
			if err != nil {
				return Server{}, fmt.Errorf("%s.%s must be a string, number, or boolean", path, key)
			}
			flags[key] = scalar
		}
	}
	if scopes, ok, err := parseScopes(raw, path); err != nil {
		return Server{}, err
	} else if ok {
		flags["oauth.scopes"] = strings.Join(scopes, " ")
	}

	typeValue, err := optionalString(raw, "type", path)
	if err != nil {
		return Server{}, err
	}

	return Server{
		Transport: normalizeTransport(typeValue, command, serverURL),
		Command:   command,
		Args:      nonNilStrings(args),
		URL:       serverURL,
		Env:       nonNilValues(env),
		Headers:   nonNilValues(headers),
		Helper:    helper,
		Include:   sortedUnique(include),
		Exclude:   sortedUnique(exclude),
		Flags:     flags,
	}, nil
}

func optionalString(object map[string]json.RawMessage, key, path string) (string, error) {
	data, ok := object[key]
	if !ok || string(data) == "null" {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return "", fmt.Errorf("%s.%s must be a string", path, key)
	}
	return value, nil
}

func optionalStrings(object map[string]json.RawMessage, key, path string) ([]string, error) {
	data, ok := object[key]
	if !ok || string(data) == "null" {
		return []string{}, nil
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return nil, fmt.Errorf("%s.%s must be an array of strings", path, key)
	}
	result := make([]string, 0, len(values))
	for index, raw := range values {
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, fmt.Errorf("%s.%s[%d] must be a string", path, key, index)
		}
		result = append(result, *value)
	}
	return result, nil
}

func optionalValues(object map[string]json.RawMessage, key, path string, format Format) (map[string]Value, error) {
	data, ok := object[key]
	if !ok || string(data) == "null" {
		return map[string]Value{}, nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return nil, fmt.Errorf("%s.%s must be an object", path, key)
	}
	result := make(map[string]Value, len(values))
	for name, raw := range values {
		var scalar any
		if err := json.Unmarshal(raw, &scalar); err != nil {
			return nil, fmt.Errorf("%s.%s.%s must be a scalar value", path, key, name)
		}
		switch scalar.(type) {
		case nil, string, float64, bool:
		default:
			return nil, fmt.Errorf("%s.%s.%s must be a scalar value", path, key, name)
		}
		text, _ := scalar.(string)
		result[name] = classifyValue(text, format)
	}
	return result, nil
}

func classifyValue(value string, format Format) Value {
	switch format {
	case FormatCursor:
		if name, ok := embedded(value, "${env:", "}"); ok {
			return Value{Kind: ValueReference, Ref: name}
		}
	case FormatVSCode:
		if name, ok := embedded(value, "${input:", "}"); ok {
			return Value{Kind: ValueInput, Ref: name}
		}
		if name, ok := embedded(value, "${", "}"); ok {
			return Value{Kind: ValueReference, Ref: name}
		}
	case FormatGemini:
		if index := strings.Index(value, "$"); index >= 0 && index+1 < len(value) {
			name := environmentName(value[index+1:])
			if name != "" {
				return Value{Kind: ValueReference, Ref: name}
			}
		}
	case FormatClaude:
		if name, ok := embedded(value, "${", "}"); ok {
			name = strings.SplitN(name, ":-", 2)[0]
			return Value{Kind: ValueReference, Ref: name}
		}
	}
	return Value{Kind: ValueLiteral}
}

func embedded(value, prefix, suffix string) (string, bool) {
	start := strings.Index(value, prefix)
	if start < 0 {
		return "", false
	}
	start += len(prefix)
	end := strings.Index(value[start:], suffix)
	if end < 0 {
		return "", false
	}
	name := value[start : start+end]
	return name, name != ""
}

func environmentName(value string) string {
	if strings.HasPrefix(value, "{") {
		end := strings.Index(value, "}")
		if end > 1 {
			return value[1:end]
		}
		return ""
	}
	end := 0
	for end < len(value) {
		character := value[end]
		if (character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '_' {
			break
		}
		end++
	}
	return value[:end]
}

func normalizeTransport(explicit, command, serverURL string) string {
	switch explicit {
	case "streamable-http":
		return "http"
	case "stdio", "http", "sse", "ws":
		return explicit
	case "":
		switch {
		case command != "":
			return "stdio"
		case serverURL != "":
			return "http"
		default:
			return "unknown"
		}
	default:
		return explicit
	}
}

func parseScopes(raw map[string]json.RawMessage, path string) ([]string, bool, error) {
	for _, objectName := range []string{"oauth", "auth"} {
		data, ok := raw[objectName]
		if !ok || string(data) == "null" {
			continue
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil || object == nil {
			return nil, false, fmt.Errorf("%s.%s must be an object", path, objectName)
		}
		scopes, ok := object["scopes"]
		if !ok || string(scopes) == "null" {
			continue
		}
		var text string
		if json.Unmarshal(scopes, &text) == nil {
			return sortedUnique(strings.Fields(text)), true, nil
		}
		var list []string
		if err := json.Unmarshal(scopes, &list); err != nil || list == nil {
			return nil, false, fmt.Errorf("%s.%s.scopes must be a string or an array of strings", path, objectName)
		}
		return sortedUnique(list), true, nil
	}
	return nil, false, nil
}

func scalarString(data json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "", err
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(typed), nil
	default:
		return "", fmt.Errorf("not a scalar")
	}
}

func MaskURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return value
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		if username == "" {
			parsed.User = url.User("***")
		} else {
			parsed.User = url.UserPassword(username, "***")
		}
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	return strings.ReplaceAll(parsed.String(), "%2A%2A%2A", "***")
}

func sortedUnique(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilValues(values map[string]Value) map[string]Value {
	if values == nil {
		return map[string]Value{}
	}
	return values
}

func hasAny(object map[string]json.RawMessage, keys ...string) bool {
	for _, key := range keys {
		if _, ok := object[key]; ok {
			return true
		}
	}
	return false
}

func rawContains(data json.RawMessage, value string) bool {
	return strings.Contains(string(data), value)
}
