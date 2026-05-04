package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ManagedConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config.d", "jump.conf"), nil
}

func backupDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jump", "backups"), nil
}

func backupFile(path string) error {
	dir, err := backupDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	ts := time.Now().Format("2006-01-02T150405")
	dst := filepath.Join(dir, fmt.Sprintf("%s.%s.backup", filepath.Base(path), ts))
	return os.WriteFile(dst, data, 0600)
}

// InitConfig ensures the managed SSH config structure is in place.
func InitConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sshDir := filepath.Join(home, ".ssh")
	configDDir := filepath.Join(sshDir, "config.d")
	sshConfigPath := filepath.Join(sshDir, "config")

	// ensure dirs
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(configDDir, 0700); err != nil {
		return err
	}

	// ensure ~/.ssh/config exists
	if _, err := os.Stat(sshConfigPath); os.IsNotExist(err) {
		if err := os.WriteFile(sshConfigPath, nil, 0600); err != nil {
			return err
		}
		fmt.Println("created ~/.ssh/config")
	}

	// ensure Include line is present
	data, err := os.ReadFile(sshConfigPath)
	if err != nil {
		return err
	}
	includeLine := "Include ~/.ssh/config.d/*.conf"
	if !strings.Contains(string(data), includeLine) {
		if err := backupFile(sshConfigPath); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		prefix := includeLine + "\n\n"
		updated := prefix + string(data)
		if err := os.WriteFile(sshConfigPath, []byte(updated), 0600); err != nil {
			return err
		}
		fmt.Printf("added %q to ~/.ssh/config\n", includeLine)
	}

	// ensure jump.conf exists
	jumpConf, err := ManagedConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(jumpConf); os.IsNotExist(err) {
		if err := os.WriteFile(jumpConf, nil, 0600); err != nil {
			return err
		}
		fmt.Printf("created %s\n", jumpConf)
	}

	return nil
}

// WriteHost adds or updates a managed host in jump.conf.
func WriteHost(h Host) error {
	path, err := ManagedConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := backupFile(path); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	blocks, err := readBlocks(path)
	if err != nil {
		return err
	}

	nb := hostToBlock(h)
	found := false
	for i, b := range blocks {
		if b.alias == h.Alias {
			blocks[i] = nb
			found = true
			break
		}
	}
	if !found {
		blocks = append(blocks, nb)
	}

	return writeBlocks(path, blocks)
}

// DeleteHost removes a managed host from jump.conf.
func DeleteHost(alias string) error {
	path, err := ManagedConfigPath()
	if err != nil {
		return err
	}
	if err := backupFile(path); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	blocks, err := readBlocks(path)
	if err != nil {
		return err
	}

	filtered := blocks[:0]
	found := false
	for _, b := range blocks {
		if b.alias == alias {
			found = true
			continue
		}
		filtered = append(filtered, b)
	}
	if !found {
		return fmt.Errorf("alias %q not found in managed config", alias)
	}

	return writeBlocks(path, filtered)
}

// RenameAlias renames a managed host alias in jump.conf.
func RenameAlias(oldAlias, newAlias string) error {
	path, err := ManagedConfigPath()
	if err != nil {
		return err
	}
	if err := backupFile(path); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	blocks, err := readBlocks(path)
	if err != nil {
		return err
	}
	found := false
	for i, b := range blocks {
		if b.alias != oldAlias {
			continue
		}
		found = true
		blocks[i].alias = newAlias
		for j, line := range b.lines {
			trimmed := strings.TrimSpace(line)
			key, _ := splitDirective(trimmed)
			if strings.ToLower(key) == "host" {
				blocks[i].lines[j] = "Host " + newAlias
			}
		}
		break
	}
	if !found {
		return fmt.Errorf("alias %q not found in managed config", oldAlias)
	}
	return writeBlocks(path, blocks)
}

// UpdateHostMeta merges the given meta keys into an existing managed host.
func UpdateHostMeta(alias string, updates map[string]string) error {
	path, err := ManagedConfigPath()
	if err != nil {
		return err
	}

	hosts, err := ParseFile(path)
	if err != nil {
		return err
	}

	var target *Host
	for i := range hosts {
		if hosts[i].Alias == alias {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("alias %q not found in managed config", alias)
	}

	if target.Meta == nil {
		target.Meta = map[string]string{}
	}

	// For tags, append rather than replace
	for k, v := range updates {
		if k == "tags" && target.Meta["tags"] != "" {
			existing := target.Meta["tags"]
			newTags := parseTags(v)
			existingTags := parseTags(existing)
			merged := mergeTags(existingTags, newTags)
			target.Meta["tags"] = strings.Join(merged, ",")
		} else {
			target.Meta[k] = v
		}
	}

	return WriteHost(*target)
}

func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}

// UpdateHostMetaByApp updates meta keys on all managed hosts with matching app code.
// Returns number of hosts updated.
func UpdateHostMetaByApp(appCode string, updates map[string]string) (int, error) {
	path, err := ManagedConfigPath()
	if err != nil {
		return 0, err
	}
	if err := backupFile(path); err != nil {
		return 0, fmt.Errorf("backup failed: %w", err)
	}
	hosts, err := ParseFile(path)
	if err != nil {
		return 0, err
	}
	blocks, err := readBlocks(path)
	if err != nil {
		return 0, err
	}
	blockByAlias := map[string]int{}
	for i, b := range blocks {
		blockByAlias[b.alias] = i
	}
	count := 0
	for _, h := range hosts {
		if h.Meta["app"] != appCode {
			continue
		}
		for k, v := range updates {
			h.Meta[k] = v
		}
		if idx, ok := blockByAlias[h.Alias]; ok {
			blocks[idx] = hostToBlock(h)
		}
		count++
	}
	if count == 0 {
		return 0, nil
	}
	return count, writeBlocks(path, blocks)
}

func parseTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func mergeTags(existing, newTags []string) []string {
	seen := map[string]bool{}
	result := append([]string{}, existing...)
	for _, t := range result {
		seen[t] = true
	}
	for _, t := range newTags {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

type block struct {
	alias string
	lines []string
}

func hostToBlock(h Host) block {
	var lines []string

	// Build # jump: comment
	meta := "# jump:"
	if v := h.Meta["app"]; v != "" {
		meta += " app=" + v
	}
	if v := h.Meta["client"]; v != "" {
		meta += " client=" + quoteIfNeeded(v)
	}
	if v := h.Meta["env"]; v != "" {
		meta += " env=" + v
	}
	if v := h.Meta["tags"]; v != "" {
		meta += " tags=" + v
	}
	if v := h.Meta["description"]; v != "" {
		meta += " description=" + quoteIfNeeded(v)
	}
	if meta != "# jump:" {
		lines = append(lines, meta)
	}

	lines = append(lines, "Host "+h.Alias)
	if h.HostName != "" {
		lines = append(lines, "  HostName "+h.HostName)
	}
	if h.User != "" {
		lines = append(lines, "  User "+h.User)
	}
	if h.Port != "" {
		lines = append(lines, "  Port "+h.Port)
	}
	if h.Identity != "" {
		lines = append(lines, "  IdentityFile "+h.Identity)
	}

	return block{alias: h.Alias, lines: lines}
}

func readBlocks(path string) ([]block, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var blocks []block
	var current *block
	var pending []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if current != nil {
				blocks = append(blocks, *current)
				current = nil
			}
			pending = nil
			continue
		}

		key, value := splitDirective(trimmed)
		if strings.ToLower(key) == "host" {
			if current != nil {
				blocks = append(blocks, *current)
			}
			aliases := strings.Fields(value)
			alias := ""
			if len(aliases) > 0 {
				alias = aliases[0]
			}
			b := block{alias: alias}
			b.lines = append(b.lines, pending...)
			b.lines = append(b.lines, line)
			current = &b
			pending = nil
		} else if current != nil {
			current.lines = append(current.lines, line)
		} else {
			pending = append(pending, line)
		}
	}

	if current != nil {
		blocks = append(blocks, *current)
	}

	return blocks, scanner.Err()
}

func writeBlocks(path string, blocks []block) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i, b := range blocks {
		if i > 0 {
			fmt.Fprintln(w)
		}
		for _, line := range b.lines {
			fmt.Fprintln(w, line)
		}
	}
	return w.Flush()
}
