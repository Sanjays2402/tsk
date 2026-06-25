import { test } from "node:test";
import assert from "node:assert/strict";
import {
  formatPct,
  streakLabel,
  ringDash,
  renderDonut,
  renderMetric,
  renderTopTags,
  renderDepStats,
  renderStatsPanel,
} from "../src/stats.ts";
import type { Stats } from "../src/api.ts";

function stats(over: Partial<Stats> = {}): Stats {
  return {
    total: 10,
    done: 4,
    undone: 6,
    overdue: 2,
    today: 1,
    completion: 40,
    streak: 3,
    top_tags: [
      { tag: "work", count: 5 },
      { tag: "home", count: 2 },
    ],
    ...over,
  };
}

test("formatPct rounds and guards non-finite", () => {
  assert.equal(formatPct(40), "40%");
  assert.equal(formatPct(66.6), "67%");
  assert.equal(formatPct(0), "0%");
  assert.equal(formatPct(NaN), "0%");
});

test("streakLabel pluralizes", () => {
  assert.equal(streakLabel(0), "no streak");
  assert.equal(streakLabel(1), "1 day");
  assert.equal(streakLabel(5), "5 days");
  assert.equal(streakLabel(-1), "no streak");
});

test("ringDash: 0% is empty ring, 100% is full", () => {
  const r = 26;
  const circ = 2 * Math.PI * r;
  const empty = ringDash(0, r);
  assert.ok(Math.abs(empty.offset - circ) < 1e-6); // full offset = empty visual
  const full = ringDash(100, r);
  assert.ok(Math.abs(full.offset) < 1e-6); // zero offset = full visual
  const half = ringDash(50, r);
  assert.ok(Math.abs(half.offset - circ / 2) < 1e-6);
});

test("ringDash clamps out-of-range completion", () => {
  const r = 10;
  const circ = 2 * Math.PI * r;
  assert.ok(Math.abs(ringDash(150, r).offset) < 1e-6); // clamps to 100
  assert.ok(Math.abs(ringDash(-20, r).offset - circ) < 1e-6); // clamps to 0
  assert.ok(Math.abs(ringDash(NaN, r).offset - circ) < 1e-6); // guards NaN -> 0
});

test("renderDonut shows the percentage label + aria", () => {
  const html = renderDonut(stats({ completion: 40 }));
  assert.match(html, /40%/);
  assert.match(html, /aria-label="40% complete"/);
  assert.match(html, /donut-fill/);
});

test("renderMetric: overdue with count gets an alert class", () => {
  assert.match(renderMetric(2, "Overdue", "overdue"), /is-alert/);
  assert.doesNotMatch(renderMetric(0, "Overdue", "overdue"), /is-alert/);
  assert.match(renderMetric(6, "Open"), /stat-num">6</);
});

test("renderTopTags renders rows with data-stat-tag + proportional bars", () => {
  const html = renderTopTags(stats());
  assert.match(html, /data-stat-tag="work"/);
  assert.match(html, /data-stat-tag="home"/);
  // The top tag (count 5, max 5) should be at 100%.
  assert.match(html, /width:100%/);
  // home is 2/5 = 40%.
  assert.match(html, /width:40%/);
});

test("renderTopTags empty state", () => {
  const html = renderTopTags(stats({ top_tags: [] }));
  assert.match(html, /No tags yet/);
  assert.doesNotMatch(html, /data-stat-tag/);
});

test("renderTopTags escapes tag names", () => {
  const html = renderTopTags(stats({ top_tags: [{ tag: "a<b", count: 1 }] }));
  assert.match(html, /a&lt;b/);
  assert.doesNotMatch(html, /data-stat-tag="a<b"/);
});

test("renderStatsPanel includes donut, metrics, streak, and tags", () => {
  const html = renderStatsPanel(stats());
  assert.match(html, /stats-donut/);
  assert.match(html, /3 days/); // streak
  assert.match(html, /Overdue/);
  assert.match(html, /Due today/);
  assert.match(html, /Top tags/);
  assert.match(html, /4 of 10 done/);
});

// --- F46: dependency-health row --------------------------------------------

test("renderDepStats shows blocked, pinned, and chain depth", () => {
  const html = renderDepStats({ blocked: 2, pinned: 3, longestChain: 4 });
  assert.match(html, /Dependencies/);
  assert.match(html, /Blocked/);
  assert.match(html, /Pinned/);
  assert.match(html, /Chain depth/);
  assert.match(html, /metric-blocked is-alert/); // alert since blocked > 0
});

// --- F64: blocked tile click-to-filter -------------------------------------

test("renderDepStats: the Blocked tile is a drill button when anything is blocked", () => {
  const html = renderDepStats({ blocked: 2, pinned: 0, longestChain: 0 });
  assert.match(html, /data-blocked-drill/);
  assert.match(html, /button[^>]*metric-blocked/);
});

test("renderDepStats: the Blocked tile is static (no drill) when nothing is blocked", () => {
  const html = renderDepStats({ blocked: 0, pinned: 2, longestChain: 0 });
  assert.doesNotMatch(html, /data-blocked-drill/);
  // still shows the (zero) Blocked tile as a plain div
  assert.match(html, /Blocked/);
});

test("renderDepStats omits the chain tile when the graph is flat", () => {
  const html = renderDepStats({ blocked: 0, pinned: 2, longestChain: 0 });
  assert.doesNotMatch(html, /Chain depth/);
  assert.match(html, /Pinned/);
  assert.doesNotMatch(html, /is-alert/); // nothing blocked
});

test("renderDepStats collapses to empty when there's nothing to show", () => {
  assert.equal(renderDepStats({ blocked: 0, pinned: 0, longestChain: 0 }), "");
});

test("renderStatsPanel includes the dep row only when dep stats are passed", () => {
  const withDep = renderStatsPanel(stats(), { blocked: 1, pinned: 0, longestChain: 0 });
  assert.match(withDep, /Dependencies/);
  // Without the dep arg, the panel renders exactly as before (back-compat).
  assert.doesNotMatch(renderStatsPanel(stats()), /Dependencies/);
});
