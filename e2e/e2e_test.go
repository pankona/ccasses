//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const testPort = "18080"
const baseURL = "http://localhost:" + testPort

var browser *rod.Browser

func TestMain(m *testing.M) {
	cmd := exec.Command("../ccasses", "serve", "--port", testPort)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start server: %v\n", err)
		os.Exit(1)
	}
	defer cmd.Process.Kill()

	if !waitForServer(baseURL+"/api/sessions", 10*time.Second) {
		fmt.Fprintln(os.Stderr, "server did not start in time")
		cmd.Process.Kill()
		os.Exit(1)
	}

	u := launcher.New().Headless(true).MustLaunch()
	browser = rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	os.Exit(m.Run())
}

func waitForServer(url string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-tick.C:
			resp, err := http.Get(url) //nolint:gosec
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return true
			}
		}
	}
}

// newPage はトップページを開いてセッション一覧が表示されるまで待つ。
func newPage(t *testing.T) *rod.Page {
	t.Helper()
	page := browser.MustPage(baseURL)
	page.MustElement("#loading").MustWaitInvisible()
	page.MustElement("#view-sessions").MustWaitVisible()
	return page
}

// navigateToDetail はトップページからセッション詳細に遷移する。
func navigateToDetail(t *testing.T) *rod.Page {
	t.Helper()
	page := newPage(t)
	page.MustElement("#sessions-tbody tr").MustClick()
	page.MustElement("#view-detail").MustWaitVisible()
	return page
}

// TestTopPageInitialDisplay: トップページの初期表示
func TestTopPageInitialDisplay(t *testing.T) {
	page := newPage(t)
	defer page.MustClose()

	// ヘッダー
	h1 := page.MustElement("header h1").MustText()
	if h1 != "ccasses" {
		t.Errorf("header h1 = %q, want %q", h1, "ccasses")
	}
	p := page.MustElement("header p").MustText()
	if !strings.Contains(p, "Claude Code Session Assessment") {
		t.Errorf("header p = %q, want to contain 'Claude Code Session Assessment'", p)
	}

	// 統計カード4つ、値が空でない
	cards := page.MustElements("#stats-row .stat-card")
	if len(cards) != 4 {
		t.Errorf("stat cards = %d, want 4", len(cards))
	}
	for i, card := range cards {
		val := card.MustElement(".value").MustText()
		if val == "" {
			t.Errorf("stat card[%d] .value is empty", i)
		}
	}
}

// TestChartDisplay: チャート表示
func TestChartDisplay(t *testing.T) {
	page := newPage(t)
	defer page.MustClose()

	// Tool Usage セクション（子要素あり）
	children := page.MustElements("#chart-tools *")
	if len(children) == 0 {
		t.Error("#chart-tools has no children")
	}

	// Model Usage canvas
	page.MustElement("#chart-models")
}

// TestSessionTable: セッションテーブル
func TestSessionTable(t *testing.T) {
	page := newPage(t)
	defer page.MustClose()

	// ヘッダー列確認
	wantHeaders := []string{"ID", "Project", "Branch", "Start", "Duration", "Turns", "Output Tokens", "Cache Read", "Models"}
	headers := page.MustElements("thead th")
	for i, want := range wantHeaders {
		if i >= len(headers) {
			t.Errorf("header[%d] missing, want %q", i, want)
			continue
		}
		got := headers[i].MustText()
		if !strings.EqualFold(got, want) {
			t.Errorf("header[%d] = %q, want %q", i, got, want)
		}
	}

	// データ行1件以上
	rows := page.MustElements("#sessions-tbody tr")
	if len(rows) == 0 {
		t.Error("sessions table has no data rows")
	}
}

// TestSessionDetailNavigation: セッション詳細への遷移
func TestSessionDetailNavigation(t *testing.T) {
	page := navigateToDetail(t)
	defer page.MustClose()

	// Back ボタン
	back := page.MustElement(".back-btn").MustText()
	if !strings.Contains(back, "Back to Sessions") {
		t.Errorf("back-btn text = %q, want to contain 'Back to Sessions'", back)
	}

	// 詳細ヘッダー
	h2 := page.MustElement("#detail-header h2").MustText()
	if h2 == "" {
		t.Error("#detail-header h2 is empty")
	}

	// 統計カード5つ
	cards := page.MustElements("#detail-stats-row .stat-card")
	if len(cards) != 5 {
		t.Errorf("detail stat cards = %d, want 5", len(cards))
	}
}

// TestSessionDetailCharts: セッション詳細のチャート
func TestSessionDetailCharts(t *testing.T) {
	page := navigateToDetail(t)
	defer page.MustClose()

	// Tool Usage セクション
	children := page.MustElements("#detail-chart-tools *")
	if len(children) == 0 {
		t.Error("#detail-chart-tools has no children")
	}

	// Model Usage canvas、Timeline canvas、Reset Zoom ボタン
	page.MustElement("#detail-chart-models")
	page.MustElement("#detail-chart-timeline")
	page.MustElement(".reset-zoom-btn")
}

// TestBackNavigation: セッション一覧への戻り
func TestBackNavigation(t *testing.T) {
	page := navigateToDetail(t)
	defer page.MustClose()

	page.MustElement(".back-btn").MustClick()
	page.MustElement("#view-sessions").MustWaitVisible()

	visible, err := page.MustElement("#view-detail").Visible()
	if err != nil {
		t.Fatalf("checking #view-detail visibility: %v", err)
	}
	if visible {
		t.Error("#view-detail should be hidden after clicking back")
	}
}

// TestToolTreeExpandCollapse: ツール使用のツリー展開
func TestToolTreeExpandCollapse(t *testing.T) {
	page := newPage(t)
	defer page.MustClose()

	// expandable な行の存在確認
	result, err := page.Eval(`() => {
		const row = document.querySelector('#chart-tools .t-row.expandable');
		if (!row) return null;
		const ul = row.parentElement.querySelector('.t-children');
		return ul ? ul.style.display : null;
	}`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if result.Value.Nil() {
		t.Skip("no expandable row found in #chart-tools")
	}

	// クリック前のトグルテキスト
	toggle := page.MustElement("#chart-tools .t-row.expandable .t-toggle")
	before := toggle.MustText()

	// クリックして展開 → t-children 表示とトグルテキスト変化を確認
	page.MustElement("#chart-tools .t-row.expandable").MustClick()

	after := toggle.MustText()
	if after == before {
		t.Error("toggle text did not change after expand")
	}

	display, err := page.Eval(`() => {
		const row = document.querySelector('#chart-tools .t-row.expandable');
		const ul = row.parentElement.querySelector('.t-children');
		return ul ? ul.style.display : null;
	}`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if display.Value.String() != "block" {
		t.Errorf("t-children display after expand = %q, want %q", display.Value.String(), "block")
	}

	// 再クリックで折りたたむ
	page.MustElement("#chart-tools .t-row.expandable").MustClick()
	if toggle.MustText() != before {
		t.Errorf("toggle text after collapse = %q, want %q", toggle.MustText(), before)
	}
}

// TestHashRouting: URL ハッシュによる直接アクセス
func TestHashRouting(t *testing.T) {
	page := navigateToDetail(t)
	defer page.MustClose()

	hash, err := page.Eval(`() => location.hash`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	hashStr := hash.Value.String()
	if !strings.HasPrefix(hashStr, "#session/") {
		t.Errorf("location.hash = %q, want prefix '#session/'", hashStr)
	}

	// ハッシュ付き URL に直接アクセス
	page2 := browser.MustPage(baseURL + "/" + hashStr)
	defer page2.MustClose()
	page2.MustElement("#loading").MustWaitInvisible()
	page2.MustElement("#view-detail").MustWaitVisible()
}

// TestAgentToolDetails: Agent ツール使用の詳細表示
func TestAgentToolDetails(t *testing.T) {
	page := newPage(t)
	defer page.MustClose()

	// Agent 行を探してクリックし、子項目数を返す（1回の eval で完結）
	result, err := page.Eval(`() => {
		const rows = document.querySelectorAll('#chart-tools .t-row.expandable');
		for (const row of rows) {
			const name = row.querySelector('.t-name');
			if (name && name.textContent.trim() === 'Agent') {
				row.click();
				const ul = row.parentElement.querySelector('.t-children');
				return { found: true, childCount: ul ? ul.querySelectorAll('li').length : 0 };
			}
		}
		return { found: false, childCount: 0 };
	}`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !result.Value.Get("found").Bool() {
		t.Skip("Agent tool not found in tool usage (no Agent calls in data)")
	}
	if result.Value.Get("childCount").Int() == 0 {
		t.Error("Agent children not expanded or empty")
	}
}

// TestSubAgentTimeline: SubAgent タイムライン線グラフ表示
func TestSubAgentTimeline(t *testing.T) {
	page := navigateToDetail(t)
	defer page.MustClose()

	// Timeline canvas が存在し Chart.js インスタンスが初期化されるまで待つ
	// (showDetail は #view-detail 表示後に非同期フェッチしてチャートを初期化する)
	page.MustElement("#detail-chart-timeline")

	result, err := page.Eval(`async () => {
		for (let i = 0; i < 50; i++) {
			if (detailChartTimeline) {
				return { ok: true, datasets: detailChartTimeline.data.datasets.map(d => d.label || '') };
			}
			await new Promise(r => setTimeout(r, 100));
		}
		return { ok: false, datasets: [] };
	}`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !result.Value.Get("ok").Bool() {
		t.Error("detailChartTimeline did not initialize within 5s")
	}
	t.Logf("timeline datasets: %s", result.Value.Get("datasets").Raw())
}
