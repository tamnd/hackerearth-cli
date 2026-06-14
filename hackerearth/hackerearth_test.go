package hackerearth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/hackerearth-cli/hackerearth"
)

func TestGetSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cfg := hackerearth.DefaultConfig()
	cfg.Rate = 0
	c := hackerearth.NewClient(cfg)

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	cfg := hackerearth.DefaultConfig()
	cfg.Rate = 0
	cfg.Retries = 5
	c := hackerearth.NewClient(cfg)

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestParseProblems(t *testing.T) {
	// Minimal HTML that mirrors what HackerEarth returns for a practice topic page.
	html := `
<div id="prob-list-container">
<ul class="prob-list" id="prob-list">
<li class="prob">
  <h4 class="prob-title dark weight-600"><a class="dark" href="/problem/algorithm/bubble-sort-15/">Bubble Sort</a></h4>
  <p class="prob-desc smaller light">
    <span>ATTEMPTED BY: <b class="weight-500 small">9258</b></span>
    <span>SUCCESS RATE: <b class="weight-500 small">86%</b></span>
    <span>LEVEL: <b class="weight-500 small">Easy</b></span>
  </p>
</li>
<li class="prob">
  <h4 class="prob-title dark weight-600"><a class="dark" href="/problem/algorithm/ants-on-circle/">Ants on a circle</a></h4>
  <p class="prob-desc smaller light">
    <span>ATTEMPTED BY: <b class="weight-500 small">2055</b></span>
    <span>SUCCESS RATE: <b class="weight-500 small">85%</b></span>
    <span>LEVEL: <b class="weight-500 small">Easy</b></span>
  </p>
</li>
</ul>
</div>
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	cfg := hackerearth.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := hackerearth.NewClient(cfg)

	probs, err := c.Problems(context.Background(), "algorithms/sorting/bubble-sort", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(probs) != 2 {
		t.Fatalf("got %d problems, want 2", len(probs))
	}
	if probs[0].Title != "Bubble Sort" {
		t.Errorf("probs[0].Title = %q, want %q", probs[0].Title, "Bubble Sort")
	}
	if probs[0].Difficulty != "Easy" {
		t.Errorf("probs[0].Difficulty = %q, want %q", probs[0].Difficulty, "Easy")
	}
	if probs[0].AttemptedBy != 9258 {
		t.Errorf("probs[0].AttemptedBy = %d, want 9258", probs[0].AttemptedBy)
	}
	if probs[0].SuccessRate != "86%" {
		t.Errorf("probs[0].SuccessRate = %q, want %q", probs[0].SuccessRate, "86%")
	}
	if !strings.HasPrefix(probs[0].URL, srv.URL) {
		t.Errorf("probs[0].URL = %q, should start with server URL", probs[0].URL)
	}
}

func TestParseChallenges(t *testing.T) {
	html := `
<div class="challenge-card-modern">
  <a class="challenge-card-wrapper challenge-card-link" href="/challenges/competitive/go-daddy-challenge/">
  </a>
  <div class="challenge-content align-center">
    <div class="challenge-type light smaller caps weight-600">
    COMPETITIVE
    </div>
    <div class="challenge-name ellipsis dark" title="GoDaddy Hiring Challenge">
      <span class="challenge-list-title">GoDaddy Hiring Challenge</span>
    </div>
    <div class="registrations">
      <i class="fa fa-user"></i> 1265
    </div>
  </div>
</div>
<div class="challenge-card-modern">
  <a class="challenge-card-wrapper challenge-card-link" href="/challenges/competitive/acme-challenge/">
  </a>
  <div class="challenge-content align-center">
    <div class="challenge-type light smaller caps weight-600">
    COMPETITIVE
    </div>
    <div class="challenge-name ellipsis dark" title="ACME Coding Challenge">
      <span class="challenge-list-title">ACME Coding Challenge</span>
    </div>
    <div class="registrations">
      <i class="fa fa-user"></i> 500
    </div>
  </div>
</div>
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	cfg := hackerearth.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := hackerearth.NewClient(cfg)

	chal, err := c.Challenges(context.Background(), hackerearth.KindCompetitive, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chal) != 2 {
		t.Fatalf("got %d challenges, want 2", len(chal))
	}
	if chal[0].Title != "GoDaddy Hiring Challenge" {
		t.Errorf("chal[0].Title = %q", chal[0].Title)
	}
	if chal[0].Participants != 1265 {
		t.Errorf("chal[0].Participants = %d, want 1265", chal[0].Participants)
	}
}

func TestProblemsLimit(t *testing.T) {
	html := `
<div id="prob-list-container">
<ul class="prob-list" id="prob-list">
<li class="prob">
  <h4 class="prob-title dark weight-600"><a class="dark" href="/problem/algorithm/p1/">Problem One</a></h4>
  <p class="prob-desc smaller light">
    <span>ATTEMPTED BY: <b class="weight-500 small">100</b></span>
    <span>SUCCESS RATE: <b class="weight-500 small">90%</b></span>
    <span>LEVEL: <b class="weight-500 small">Easy</b></span>
  </p>
</li>
<li class="prob">
  <h4 class="prob-title dark weight-600"><a class="dark" href="/problem/algorithm/p2/">Problem Two</a></h4>
  <p class="prob-desc smaller light">
    <span>ATTEMPTED BY: <b class="weight-500 small">200</b></span>
    <span>SUCCESS RATE: <b class="weight-500 small">80%</b></span>
    <span>LEVEL: <b class="weight-500 small">Medium</b></span>
  </p>
</li>
<li class="prob">
  <h4 class="prob-title dark weight-600"><a class="dark" href="/problem/algorithm/p3/">Problem Three</a></h4>
  <p class="prob-desc smaller light">
    <span>ATTEMPTED BY: <b class="weight-500 small">50</b></span>
    <span>SUCCESS RATE: <b class="weight-500 small">70%</b></span>
    <span>LEVEL: <b class="weight-500 small">Hard</b></span>
  </p>
</li>
</ul>
</div>
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	cfg := hackerearth.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := hackerearth.NewClient(cfg)

	probs, err := c.Problems(context.Background(), "algorithms/sorting/bubble-sort", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(probs) != 2 {
		t.Fatalf("got %d problems with limit=2, want 2", len(probs))
	}
}

func TestTopics(t *testing.T) {
	cfg := hackerearth.DefaultConfig()
	c := hackerearth.NewClient(cfg)

	topics := c.Topics()
	if len(topics) == 0 {
		t.Fatal("Topics() returned empty slice")
	}
	for _, tp := range topics {
		if tp.Name == "" {
			t.Error("topic with empty Name")
		}
		if tp.URL == "" {
			t.Error("topic with empty URL")
		}
		if tp.Slug == "" {
			t.Error("topic with empty Slug")
		}
	}
}
