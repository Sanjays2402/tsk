import { test } from "node:test";
import assert from "node:assert/strict";
import {
  normalizeNotes,
  resolveNotes,
  notesPreview,
  hasNotes,
  notesLineCount,
  renderNotesButton,
  renderNotesSnippet,
} from "../src/notes.ts";

test("normalizeNotes trims trailing whitespace per line", () => {
  assert.equal(normalizeNotes("a   \nb\t\n"), "a\nb");
});

test("normalizeNotes drops leading + trailing blank lines, keeps interior", () => {
  assert.equal(normalizeNotes("\n\nfirst\n\nsecond\n\n"), "first\n\nsecond");
});

test("normalizeNotes normalizes CRLF and CR to LF", () => {
  assert.equal(normalizeNotes("a\r\nb\rc"), "a\nb\nc");
});

test("resolveNotes: unchanged content is a noop", () => {
  assert.deepEqual(resolveNotes("hello", "hello"), { kind: "noop" });
});

test("resolveNotes: whitespace-only diff is a noop", () => {
  assert.deepEqual(resolveNotes("hello", "hello   \n\n"), { kind: "noop" });
});

test("resolveNotes: real change commits the normalized text", () => {
  assert.deepEqual(resolveNotes("hello", "hello\nworld  "), {
    kind: "commit",
    notes: "hello\nworld",
  });
});

test("resolveNotes: clearing notes commits an empty string", () => {
  assert.deepEqual(resolveNotes("hello", "   \n  "), { kind: "commit", notes: "" });
});

test("notesPreview returns the first non-blank line, truncated", () => {
  assert.equal(notesPreview("\n\nfirst line\nsecond"), "first line");
  assert.equal(notesPreview("x".repeat(60), 10), "x".repeat(9) + "\u2026");
});

test("hasNotes detects non-blank content", () => {
  assert.equal(hasNotes(undefined), false);
  assert.equal(hasNotes(""), false);
  assert.equal(hasNotes("   \n\n"), false);
  assert.equal(hasNotes("note"), true);
});

test("notesLineCount counts non-blank lines", () => {
  assert.equal(notesLineCount(undefined), 0);
  assert.equal(notesLineCount("a\n\nb\nc"), 3);
});

test("renderNotesButton reflects empty vs populated state", () => {
  const empty = renderNotesButton(undefined);
  assert.match(empty, /\+note/);
  assert.doesNotMatch(empty, /has-notes/);

  const filled = renderNotesButton("line one\nline two");
  assert.match(filled, /has-notes/);
  assert.match(filled, /notes-n">2</);
});

test("renderNotesButton escapes the preview in the title attribute", () => {
  const html = renderNotesButton('<script>alert(1)</script>');
  assert.doesNotMatch(html, /<script>/);
  assert.match(html, /&lt;script&gt;/);
});

test("renderNotesSnippet is empty for blank notes, populated otherwise", () => {
  assert.equal(renderNotesSnippet(undefined), "");
  assert.equal(renderNotesSnippet("   \n\n"), "");
  const html = renderNotesSnippet("remember the milk");
  assert.match(html, /notes-snippet/);
  assert.match(html, /data-notes/);
  assert.match(html, /remember the milk/);
});

test("renderNotesSnippet shows the first non-blank line, truncated", () => {
  const html = renderNotesSnippet("\n\nfirst line here\nsecond", 8);
  assert.match(html, /first l\u2026/);
  assert.doesNotMatch(html, /second/);
});

test("renderNotesSnippet notes a multi-line count in the tooltip", () => {
  const html = renderNotesSnippet("one\ntwo\nthree");
  assert.match(html, /\(3 lines\)/);
  // single-line notes get no count suffix
  assert.doesNotMatch(renderNotesSnippet("solo"), /lines\)/);
});

test("renderNotesSnippet escapes untrusted content", () => {
  const html = renderNotesSnippet("<b>x</b> & <i>y</i>");
  assert.doesNotMatch(html, /<b>x<\/b>/);
  assert.match(html, /&lt;b&gt;x/);
});
