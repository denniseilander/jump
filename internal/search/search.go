package search

import (
	"sort"
	"strings"
	"time"

	"github.com/denniseilander/jump/internal/sshconfig"
	"github.com/denniseilander/jump/internal/store"
)

type Result struct {
	Host  sshconfig.Host
	Score int
}

type MatchReason struct {
	Token string
	Field string
	Score int
}

type DetailedResult struct {
	Result
	Reasons []MatchReason
}

var synonyms = map[string][]string{
	"production":  {"production", "prod", "prd", "productie"},
	"prod":        {"production", "prod", "prd", "productie"},
	"productie":   {"production", "prod", "prd", "productie"},
	"prd":         {"production", "prod", "prd", "productie"},
	"acceptance":  {"acceptance", "acc", "acceptatie", "staging"},
	"acc":         {"acceptance", "acc", "acceptatie", "staging"},
	"acceptatie":  {"acceptance", "acc", "acceptatie", "staging"},
	"staging":     {"acceptance", "acc", "acceptatie", "staging"},
	"development": {"development", "dev", "ontwikkeling"},
	"dev":         {"development", "dev", "ontwikkeling"},
	"ontwikkeling": {"development", "dev", "ontwikkeling"},
	"test":        {"test", "tst"},
	"tst":         {"test", "tst"},
}

func Find(hosts []sshconfig.Host, query string) []Result {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return all(hosts)
	}

	history := store.Load()

	tokens := expandTokens(strings.Fields(query))
	results := make([]Result, 0, len(hosts))
	for _, h := range hosts {
		score := scoreHost(h, tokens)
		if score > 0 {
			score += historyBonus(h.Alias, history)
			results = append(results, Result{Host: h, Score: score})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Host.Alias < results[j].Host.Alias
		}
		return results[i].Score > results[j].Score
	})
	return results
}

func all(hosts []sshconfig.Host) []Result {
	history := store.Load()
	results := make([]Result, 0, len(hosts))
	for _, h := range hosts {
		score := 1 + historyBonus(h.Alias, history)
		results = append(results, Result{Host: h, Score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Host.Alias < results[j].Host.Alias
		}
		return results[i].Score > results[j].Score
	})
	return results
}

func historyBonus(alias string, history store.History) int {
	entry, ok := history[alias]
	if !ok {
		return 0
	}
	bonus := 0

	// recency: +20 today, +15 this week, +10 this month, +5 older
	since := time.Since(entry.LastUsedAt)
	switch {
	case since < 24*time.Hour:
		bonus += 20
	case since < 7*24*time.Hour:
		bonus += 15
	case since < 30*24*time.Hour:
		bonus += 10
	default:
		bonus += 5
	}

	// frequency: up to +20, capped
	freq := entry.ConnectCount * 2
	if freq > 20 {
		freq = 20
	}
	bonus += freq

	return bonus
}

func expandTokens(tokens []string) [][]string {
	groups := make([][]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.ToLower(token)
		if values, ok := synonyms[token]; ok {
			groups = append(groups, values)
		} else {
			groups = append(groups, []string{token})
		}
	}
	return groups
}

func scoreHost(h sshconfig.Host, tokenGroups [][]string) int {
	total := 0
	for _, group := range tokenGroups {
		best := 0
		for _, token := range group {
			best = max(best, scoreToken(h, token))
		}
		if best == 0 {
			return 0
		}
		total += best
	}
	return total
}

func scoreToken(h sshconfig.Host, token string) int {
	alias := strings.ToLower(h.Alias)
	hostname := strings.ToLower(h.HostName)
	user := strings.ToLower(h.User)

	if alias == token {
		return 100
	}
	if strings.HasPrefix(alias, token) {
		return 75
	}
	if strings.Contains(alias, token) {
		return 60
	}

	for key, value := range h.Meta {
		key = strings.ToLower(key)
		value = strings.ToLower(value)
		switch key {
		case "env", "environment":
			if value == token {
				return 70
			}
		case "client":
			if value == token {
				return 65
			}
			if strings.Contains(value, token) {
				return 45
			}
		case "app", "application":
			if value == token {
				return 60
			}
			if strings.Contains(value, token) {
				return 40
			}
		case "tags":
			for _, tag := range strings.Split(value, ",") {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				if tag == token {
					return 65
				}
			}
		case "description":
			if strings.Contains(value, token) {
				return 15
			}
		}
	}

	if hostname == token {
		return 55
	}
	if strings.Contains(hostname, token) {
		return 35
	}
	if user == token {
		return 30
	}
	if strings.Contains(user, token) {
		return 20
	}
	return 0
}

// FindWithReasons is like Find but also returns per-token match explanations.
func FindWithReasons(hosts []sshconfig.Host, query string) []DetailedResult {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil
	}

	history := store.Load()
	tokens := expandTokens(strings.Fields(query))

	var results []DetailedResult
	for _, h := range hosts {
		score, reasons := scoreHostDetailed(h, tokens)
		if score > 0 {
			bonus := historyBonus(h.Alias, history)
			score += bonus
			results = append(results, DetailedResult{
				Result:  Result{Host: h, Score: score},
				Reasons: reasons,
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Host.Alias < results[j].Host.Alias
		}
		return results[i].Score > results[j].Score
	})
	return results
}

func scoreHostDetailed(h sshconfig.Host, tokenGroups [][]string) (int, []MatchReason) {
	total := 0
	var reasons []MatchReason
	for _, group := range tokenGroups {
		bestScore := 0
		var bestReason MatchReason
		for _, token := range group {
			s, field := scoreTokenDetailed(h, token)
			if s > bestScore {
				bestScore = s
				bestReason = MatchReason{Token: token, Field: field, Score: s}
			}
		}
		if bestScore == 0 {
			return 0, nil
		}
		total += bestScore
		reasons = append(reasons, bestReason)
	}
	return total, reasons
}

func scoreTokenDetailed(h sshconfig.Host, token string) (int, string) {
	alias := strings.ToLower(h.Alias)
	hostname := strings.ToLower(h.HostName)
	user := strings.ToLower(h.User)

	if alias == token {
		return 100, "alias exact"
	}
	if strings.HasPrefix(alias, token) {
		return 75, "alias prefix"
	}
	if strings.Contains(alias, token) {
		return 60, "alias contains"
	}

	for key, value := range h.Meta {
		key = strings.ToLower(key)
		value = strings.ToLower(value)
		switch key {
		case "env", "environment":
			if value == token {
				return 70, "environment exact"
			}
		case "client":
			if value == token {
				return 65, "client exact"
			}
			if strings.Contains(value, token) {
				return 45, "client contains"
			}
		case "app", "application":
			if value == token {
				return 60, "application exact"
			}
			if strings.Contains(value, token) {
				return 40, "application contains"
			}
		case "tags":
			for _, tag := range strings.Split(value, ",") {
				tag = strings.TrimSpace(tag)
				if tag == token {
					return 65, "tag exact"
				}
			}
		case "description":
			if strings.Contains(value, token) {
				return 15, "description contains"
			}
		}
	}

	if hostname == token {
		return 55, "hostname exact"
	}
	if strings.Contains(hostname, token) {
		return 35, "hostname contains"
	}
	if user == token {
		return 30, "user exact"
	}
	if strings.Contains(user, token) {
		return 20, "user contains"
	}
	return 0, ""
}

