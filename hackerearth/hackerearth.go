// Package hackerearth is the library behind the he command: the HTTP client,
// request shaping, and the typed data models for HackerEarth.
//
// HackerEarth is a competitive programming and developer assessment platform.
// This library scrapes the public practice and challenge pages — no API key
// required. Problems come from the practice topic pages; challenges come from
// the competitive and hackathon listing pages.
package hackerearth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// BaseURL is the canonical root for all HackerEarth pages.
	BaseURL = "https://www.hackerearth.com"
)

// DefaultUserAgent identifies the client to HackerEarth.
const DefaultUserAgent = "he/dev (+https://github.com/tamnd/hackerearth-cli)"

// Config holds constructor parameters for the client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to HackerEarth over HTTP.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// Get fetches url and returns the response body.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// ─── Problems ────────────────────────────────────────────────────────────────

// Problem is a single practice problem on HackerEarth.
type Problem struct {
	Title       string `json:"title"`
	Difficulty  string `json:"difficulty"`
	Topic       string `json:"topic"`
	AttemptedBy int    `json:"attempted_by"`
	SuccessRate string `json:"success_rate"`
	URL         string `json:"url"`
}

// practiceTopic maps the slug used in URLs to a human-friendly name.
// The URL pattern is /practice/<category>/<topic>/practice-problems/
// e.g. /practice/algorithms/sorting/bubble-sort/practice-problems/
type topicPath struct {
	Category string
	Slug     string
}

// KnownTopics lists popular practice topics that have public problem lists.
// Category is the first path segment, Slug is the topic leaf.
var KnownTopics = []topicPath{
	{"algorithms", "sorting/bubble-sort"},
	{"algorithms", "sorting/merge-sort"},
	{"algorithms", "sorting/quick-sort"},
	{"algorithms", "sorting/heap-sort"},
	{"algorithms", "searching/binary-search"},
	{"algorithms", "searching/linear-search"},
	{"algorithms", "dynamic-programming/introduction-to-dynamic-programming-1"},
	{"algorithms", "dynamic-programming/2-dimensional"},
	{"algorithms", "graphs/breadth-first-search"},
	{"algorithms", "graphs/depth-first-search"},
	{"algorithms", "graphs/shortest-path-algorithms"},
	{"algorithms", "greedy/basics-of-greedy-algorithms"},
	{"algorithms", "string-algorithm/basics-of-string-manipulation"},
	{"data-structures", "trees/binary-search-tree"},
	{"data-structures", "trees/binary-and-nary-trees"},
	{"data-structures", "stacks-and-queues/basics-of-stacks"},
	{"data-structures", "stacks-and-queues/basics-of-queues"},
	{"data-structures", "linked-lists/singly-linked-list"},
	{"data-structures", "hash-tables/basics-of-hash-tables"},
	{"math", "basic-number-theory/basic-number-theory-1"},
	{"math", "combinatorics/basics-of-combinatorics"},
}

// regexes for parsing the practice problems list page.
var (
	reProbTitle   = regexp.MustCompile(`class="prob-title[^"]*">\s*<a[^>]+href="(/problem/[^"]+)"[^>]*>([^<]+)</a>`)
	reProbAttempt = regexp.MustCompile(`ATTEMPTED BY:\s*<b[^>]*>(\d+)</b>`)
	reProbRate    = regexp.MustCompile(`SUCCESS RATE:\s*<b[^>]*>([^<]+)</b>`)
	reProbLevel   = regexp.MustCompile(`LEVEL:\s*<b[^>]*>([^<]+)</b>`)
)

// Problems fetches practice problems for the given topic path (e.g.
// "algorithms/sorting/bubble-sort"). Pass an empty topic to fetch from the
// first known topic. limit=0 returns all found on the page.
func (c *Client) Problems(ctx context.Context, topic string, limit int) ([]Problem, error) {
	if topic == "" {
		topic = "algorithms/sorting/bubble-sort"
	}
	// Build URL: /practice/<category>/<topic>/practice-problems/
	// topic may already be full path like "algorithms/sorting/bubble-sort"
	// or a partial like "bubble-sort" — we guess full if no slash present.
	practiceURL := c.baseURL + "/practice/" + strings.Trim(topic, "/") + "/practice-problems/"

	body, err := c.Get(ctx, practiceURL)
	if err != nil {
		return nil, fmt.Errorf("problems %q: %w", topic, err)
	}
	html := string(body)

	// Derive a clean topic name from the slug (last path component, de-hyphenated).
	slug := topic
	if idx := strings.LastIndex(slug, "/"); idx >= 0 {
		slug = slug[idx+1:]
	}
	topicName := strings.ReplaceAll(slug, "-", " ")
	topicName = strings.Title(topicName) //nolint:staticcheck

	return parseProblems(html, topicName, c.baseURL, limit), nil
}

func parseProblems(html, topic, base string, limit int) []Problem {
	// Find the prob-list-container section only.
	start := strings.Index(html, `id="prob-list-container"`)
	if start < 0 {
		start = strings.Index(html, `id="prob-list"`)
	}
	if start < 0 {
		return nil
	}
	end := strings.Index(html[start:], `</ul>`)
	if end > 0 {
		html = html[start : start+end]
	} else {
		html = html[start:]
	}

	titleMatches := reProbTitle.FindAllStringSubmatch(html, -1)
	attemptMatches := reProbAttempt.FindAllStringSubmatch(html, -1)
	rateMatches := reProbRate.FindAllStringSubmatch(html, -1)
	levelMatches := reProbLevel.FindAllStringSubmatch(html, -1)

	var out []Problem
	for i, tm := range titleMatches {
		if limit > 0 && len(out) >= limit {
			break
		}
		p := Problem{
			Title: strings.TrimSpace(tm[2]),
			URL:   base + tm[1],
			Topic: topic,
		}
		if i < len(attemptMatches) {
			n, _ := strconv.Atoi(attemptMatches[i][1])
			p.AttemptedBy = n
		}
		if i < len(rateMatches) {
			p.SuccessRate = strings.TrimSpace(rateMatches[i][1])
		}
		if i < len(levelMatches) {
			p.Difficulty = strings.TrimSpace(levelMatches[i][1])
		}
		out = append(out, p)
	}
	return out
}

// ─── Challenges ──────────────────────────────────────────────────────────────

// Challenge is a competitive event on HackerEarth.
type Challenge struct {
	Title        string `json:"title"`
	Type         string `json:"type"`
	Participants int    `json:"participants"`
	Slug         string `json:"slug"`
	URL          string `json:"url"`
}

// ChallengeKind selects which page to fetch.
type ChallengeKind string

const (
	KindCompetitive ChallengeKind = "competitive"
	KindHackathon   ChallengeKind = "hackathon"
	KindAll         ChallengeKind = "all" // fetch both pages
)

var (
	reChalName = regexp.MustCompile(`challenge-name ellipsis dark[^>]*title="([^"]+)"`)
	// Match exactly COMPETITIVE/HACKATHON/SPRINT/SCHOOL/UNIVERSITY in the type div.
	reChalType = regexp.MustCompile(`challenge-type[^>]*>\s*\n?\s*(COMPETITIVE|HACKATHON|SPRINT|SCHOOL|UNIVERSITY)\s*\n?\s*</div>`)
	reChalPart = regexp.MustCompile(`fa fa-user"></i>\s*(\d+)`)
	// Match any challenge-card-link href (absolute or relative, competitive or hackathon).
	reChalHref = regexp.MustCompile(`challenge-card-link[^"]*"\s+[^>]*href="(?:https?://[^/]+)?(/challenges/[^/"]+/([^/"]+)/)"`)
)

// Challenges fetches active challenge listings. kind selects competitive,
// hackathon, or both.
func (c *Client) Challenges(ctx context.Context, kind ChallengeKind, limit int) ([]Challenge, error) {
	if kind == KindAll {
		comp, err1 := c.fetchChallenges(ctx, KindCompetitive, limit)
		hack, err2 := c.fetchChallenges(ctx, KindHackathon, limit)
		if err1 != nil && err2 != nil {
			return nil, err1
		}
		out := append(comp, hack...)
		if limit > 0 && len(out) > limit {
			out = out[:limit]
		}
		return out, nil
	}
	return c.fetchChallenges(ctx, kind, limit)
}

func (c *Client) fetchChallenges(ctx context.Context, kind ChallengeKind, limit int) ([]Challenge, error) {
	url := c.baseURL + "/challenges/" + string(kind) + "/"
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("challenges %q: %w", kind, err)
	}
	return parseChallenges(string(body), c.baseURL, limit), nil
}

func parseChallenges(html, base string, limit int) []Challenge {
	names := reChalName.FindAllStringSubmatch(html, -1)
	parts := reChalPart.FindAllStringSubmatch(html, -1)
	hrefs := reChalHref.FindAllStringSubmatch(html, -1)

	// Build a type lookup: for cards that have a challenge-type div, record what type.
	// The type count may be less than name count (some cards omit it).
	// We use a positional approach: pair types with names using their string positions.
	typeByPos := buildTypeByPosition(html)

	var out []Challenge
	for i, nm := range names {
		if limit > 0 && len(out) >= limit {
			break
		}
		ch := Challenge{
			Title: strings.TrimSpace(unescapeHTML(nm[1])),
		}
		if i < len(parts) {
			ch.Participants, _ = strconv.Atoi(parts[i][1])
		}
		if i < len(hrefs) {
			ch.URL = base + hrefs[i][1]
			ch.Slug = hrefs[i][2]
		}
		// Look up type by name index position in the page.
		if i < len(typeByPos) {
			ch.Type = typeByPos[i]
		}
		out = append(out, ch)
	}
	return out
}

// buildTypeByPosition returns a slice of challenge types aligned with the
// challenge-name positions. Cards without a type div get an empty string.
func buildTypeByPosition(html string) []string {
	// Collect (position, type) pairs for challenge-type divs.
	typeMatches := reChalType.FindAllStringSubmatchIndex(html, -1)
	nameMatches := reChalName.FindAllStringSubmatchIndex(html, -1)

	// For each name, find the closest preceding type div.
	result := make([]string, len(nameMatches))
	for i, nm := range nameMatches {
		namePos := nm[0]
		// Search backwards through type positions for one before this name.
		for _, tm := range typeMatches {
			if tm[0] < namePos {
				result[i] = strings.TrimSpace(html[tm[2]:tm[3]])
			}
		}
	}
	// De-duplicate: if the same type position was used for multiple names, keep
	// only the first assignment. Walk forward and clear repeats.
	lastType := ""
	for i, t := range result {
		if t == lastType {
			result[i] = ""
		} else if t != "" {
			lastType = t
		}
	}
	return result
}

// ─── Topics ──────────────────────────────────────────────────────────────────

// Topic is a practice topic (category + slug).
type Topic struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Slug     string `json:"slug"`
	URL      string `json:"url"`
}

// Topics returns the list of known practice topics.
func (c *Client) Topics() []Topic {
	out := make([]Topic, 0, len(KnownTopics))
	for _, t := range KnownTopics {
		slug := t.Slug
		name := slug
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		name = strings.ReplaceAll(name, "-", " ")
		name = strings.Title(name) //nolint:staticcheck
		out = append(out, Topic{
			Name:     name,
			Category: t.Category,
			Slug:     t.Category + "/" + t.Slug,
			URL:      c.baseURL + "/practice/" + t.Category + "/" + t.Slug + "/practice-problems/",
		})
	}
	return out
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func unescapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}
