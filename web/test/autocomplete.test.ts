import { test } from "node:test";
import assert from "node:assert/strict";
import {
  activeToken,
  tagSuggestions,
  dueSuggestions,
  suggestFor,
  applySuggestion,
  renderAutocomplete,
  DUE_PRESETS,
} from "../src/autocomplete.ts";

const TAGS = [
  { tag: "dev", count: 9 },
  { tag: "devops", count: 3 },
  { tag: "design", count: 5 },
  { tag: "home", count: 2 },
];

test("activeToken finds a #tag token at the caret", () => {
  const text = "fix bug #de";
  const tok = activeToken(text, text.length);
  assert.ok(tok);
  assert.equal(tok!.kind, "tag");
  assert.equal(tok!.query, "de");
  assert.equal(tok!.start, 8);
  assert.equal(tok!.end, 11);
});

test("activeToken finds an @due token", () => {
  const text = "pay rent @tom";
  const tok = activeToken(text, text.length);
  assert.ok(tok);
  assert.equal(tok!.kind, "due");
  assert.equal(tok!.query, "tom");
});

test("activeToken returns null in plain title text", () => {
  const text = "just a title";
  assert.equal(activeToken(text, text.length), null);
});

test("activeToken returns null right after a space", () => {
  const text = "x #dev ";
  assert.equal(activeToken(text, text.length), null);
});

test("activeToken ignores a mid-word sigil (c# is not a token)", () => {
  const text = "learn c#";
  assert.equal(activeToken(text, text.length), null);
});

test("activeToken handles the caret mid-string", () => {
  const text = "a #dev b";
  const tok = activeToken(text, 6); // caret right after 'v'
  assert.ok(tok);
  assert.equal(tok!.query, "dev");
});

test("tagSuggestions ranks prefix matches then by count", () => {
  const out = tagSuggestions("de", TAGS);
  assert.deepEqual(
    out.map((s) => s.value),
    ["dev", "design", "devops"], // all prefix 'de': dev(9) > design(5) > devops(3)
  );
});

test("tagSuggestions substring match ranks below prefix", () => {
  const tags = [
    { tag: "frontend", count: 1 },
    { tag: "end", count: 10 },
  ];
  const out = tagSuggestions("end", tags);
  // 'end' is a prefix match; 'frontend' only a substring -> end first.
  assert.deepEqual(
    out.map((s) => s.value),
    ["end", "frontend"],
  );
});

test("tagSuggestions empty query returns most-used first", () => {
  const out = tagSuggestions("", TAGS);
  assert.equal(out[0].value, "dev"); // highest count
});

test("dueSuggestions filters presets by substring, prefix-first", () => {
  const out = dueSuggestions("to");
  assert.equal(out[0].value, "today"); // 'today' & 'tomorrow' start with 'to'
  assert.ok(out.every((s) => s.value.includes("to")));
});

test("dueSuggestions empty query returns the preset list", () => {
  const out = dueSuggestions("");
  assert.equal(out.length, Math.min(8, DUE_PRESETS.length));
});

test("suggestFor dispatches by token kind, [] when no token", () => {
  assert.equal(suggestFor(null, TAGS).length, 0);
  const tagTok = activeToken("x #de", 5)!;
  assert.ok(suggestFor(tagTok, TAGS).length > 0);
  const dueTok = activeToken("x @to", 5)!;
  assert.equal(suggestFor(dueTok, TAGS)[0].kind, "due");
});

test("applySuggestion splices the value in with a trailing space", () => {
  const text = "fix #de bug";
  const tok = activeToken("fix #de", 7)!; // token is '#de' at 4..7
  const { text: next, caret } = applySuggestion(text, tok, "dev");
  assert.equal(next, "fix #dev  bug");
  assert.equal(caret, "fix #dev ".length);
});

test("renderAutocomplete marks the active row and carries hooks", () => {
  const html = renderAutocomplete(dueSuggestions("to"), 0);
  assert.match(html, /is-active/);
  assert.match(html, /data-ac-value="today"/);
  assert.match(html, /data-ac-kind="due"/);
});

test("renderAutocomplete is empty when there are no suggestions", () => {
  assert.equal(renderAutocomplete([], 0), "");
});
