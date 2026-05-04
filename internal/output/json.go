package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/denniseilander/jump/internal/search"
	"github.com/denniseilander/jump/internal/sshconfig"
)

type jsonHost struct {
	Alias    string   `json:"alias"`
	Hostname string   `json:"hostname,omitempty"`
	User     string   `json:"user,omitempty"`
	Port     string   `json:"port,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Score    int      `json:"score,omitempty"`
}

type jsonSearchResult struct {
	Query   string     `json:"query"`
	Matches []jsonHost `json:"matches"`
}

type jsonScanResult struct {
	ConfigPath    string   `json:"config_path"`
	IncludedFiles []string `json:"included_files"`
	HostCount     int      `json:"host_count"`
	ManagedCount  int      `json:"managed_count"`
	MetaPath      string   `json:"meta_path"`
	HistoryPath   string   `json:"history_path"`
}

func HostsJSON(results []search.Result, query string) error {
	hosts := make([]jsonHost, 0, len(results))
	for _, r := range results {
		hosts = append(hosts, toJSONHost(r))
	}
	out := jsonSearchResult{Query: query, Matches: hosts}
	return printJSON(out)
}

func ScanJSON(hosts []sshconfig.Host, includedFiles []string, managedPath, metaPath, historyPath string) error {
	managed := 0
	for _, h := range hosts {
		if h.SourceFile == managedPath {
			managed++
		}
	}
	out := jsonScanResult{
		ConfigPath:    sshconfig.DefaultConfigPath(),
		IncludedFiles: includedFiles,
		HostCount:     len(hosts),
		ManagedCount:  managed,
		MetaPath:      metaPath,
		HistoryPath:   historyPath,
	}
	return printJSON(out)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "jump: json encode error:", err)
		return err
	}
	return nil
}

func toJSONHost(r search.Result) jsonHost {
	h := r.Host
	var tags []string
	if raw := h.Meta["tags"]; raw != "" {
		for _, t := range strings.Split(raw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	port := h.Port
	if port == "" {
		port = "22"
	}
	return jsonHost{
		Alias:    h.Alias,
		Hostname: h.HostName,
		User:     h.User,
		Port:     port,
		Tags:     tags,
		Score:    r.Score,
	}
}
