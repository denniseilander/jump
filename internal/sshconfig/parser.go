package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Host struct {
	Alias      string            `json:"alias"`
	HostName   string            `json:"hostname,omitempty"`
	User       string            `json:"user,omitempty"`
	Port       string            `json:"port,omitempty"`
	Identity   string            `json:"identity_file,omitempty"`
	SourceFile string            `json:"source_file"`
	Line       int               `json:"line"`
	Meta       map[string]string `json:"meta,omitempty"`
	RawOptions map[string]string `json:"raw_options,omitempty"`
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/config"
	}
	return filepath.Join(home, ".ssh", "config")
}

// ParseResult holds hosts and all included file paths.
type ParseResult struct {
	Hosts         []Host
	IncludedFiles []string
}

func ParseDefault() ([]Host, error) {
	return ParseFile(DefaultConfigPath())
}

func ParseFile(path string) ([]Host, error) {
	visited := map[string]bool{}
	return parseFile(path, visited, nil)
}

// ScanDefault parses SSH config and returns hosts plus all included file paths.
func ScanDefault() (ParseResult, error) {
	visited := map[string]bool{}
	var included []string
	hosts, err := parseFile(DefaultConfigPath(), visited, &included)
	return ParseResult{Hosts: hosts, IncludedFiles: included}, err
}

func parseFile(path string, visited map[string]bool, includedFiles *[]string) ([]Host, error) {
	path = expandHome(path)
	abs, _ := filepath.Abs(path)
	if visited[abs] {
		return nil, nil
	}
	visited[abs] = true

	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var hosts []Host
	var current *Host
	var pendingMeta map[string]string

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			meta := parseJumpMeta(line)
			if len(meta) > 0 {
				pendingMeta = meta
			}
			continue
		}

		key, value := splitDirective(line)
		lowerKey := strings.ToLower(key)

		if lowerKey == "include" {
			patterns := strings.Fields(value)
			for _, pattern := range patterns {
				resolved := resolveInclude(pattern, filepath.Dir(abs))
				matches, _ := filepath.Glob(resolved)
				for _, match := range matches {
					if includedFiles != nil {
						*includedFiles = append(*includedFiles, match)
					}
					includedHosts, includeErr := parseFile(match, visited, includedFiles)
					if includeErr != nil {
						return nil, includeErr
					}
					hosts = append(hosts, includedHosts...)
				}
			}
			continue
		}

		if lowerKey == "host" {
			aliases := strings.Fields(value)
			for _, alias := range aliases {
				if alias == "*" || strings.ContainsAny(alias, "?![]") {
					continue
				}
				h := Host{Alias: alias, SourceFile: abs, Line: lineNo, Meta: pendingMeta, RawOptions: map[string]string{}}
				hosts = append(hosts, h)
				current = &hosts[len(hosts)-1]
			}
			pendingMeta = nil
			continue
		}

		if current == nil {
			continue
		}

		switch lowerKey {
		case "hostname":
			current.HostName = value
		case "user":
			current.User = value
		case "port":
			current.Port = value
		case "identityfile":
			current.Identity = value
		default:
			current.RawOptions[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

func splitDirective(line string) (string, string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", ""
	}
	key := parts[0]
	value := strings.TrimSpace(strings.TrimPrefix(line, key))
	return key, value
}

func parseJumpMeta(line string) map[string]string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
	if !strings.HasPrefix(strings.ToLower(line), "jump:") {
		return nil
	}
	line = strings.TrimSpace(line[len("jump:"):])
	meta := map[string]string{}
	for len(line) > 0 {
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			break
		}
		key := strings.ToLower(strings.TrimSpace(line[:eqIdx]))
		line = line[eqIdx+1:]
		var value string
		if len(line) > 0 && (line[0] == '"' || line[0] == '\'') {
			quote := line[0]
			line = line[1:]
			end := strings.IndexByte(line, quote)
			if end < 0 {
				value = line
				line = ""
			} else {
				value = line[:end]
				line = line[end+1:]
			}
		} else {
			spaceIdx := strings.IndexAny(line, " \t")
			if spaceIdx < 0 {
				value = line
				line = ""
			} else {
				value = line[:spaceIdx]
				line = line[spaceIdx:]
			}
		}
		if key != "" {
			meta[key] = value
		}
	}
	return meta
}

func resolveInclude(pattern string, baseDir string) string {
	pattern = strings.Trim(pattern, `"'`)
	if strings.HasPrefix(pattern, "~/") {
		return expandHome(pattern)
	}
	if filepath.IsAbs(pattern) {
		return pattern
	}
	return filepath.Join(baseDir, pattern)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func FormatHost(h Host) string {
	target := h.HostName
	if target == "" {
		target = h.Alias
	}
	userPrefix := ""
	if h.User != "" {
		userPrefix = h.User + "@"
	}
	port := ""
	if h.Port != "" && h.Port != "22" {
		port = ":" + h.Port
	}
	return fmt.Sprintf("%s -> %s%s%s", h.Alias, userPrefix, target, port)
}
