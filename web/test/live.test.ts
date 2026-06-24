import { test } from "node:test";
import assert from "node:assert/strict";
import {
  parseFingerprint,
  shouldRefresh,
  liveLabel,
  liveTitle,
  liveChangeMessage,
  renderLiveIndicator,
  type FileFingerprint,
} from "../src/live.ts";

test("parseFingerprint reads a valid payload", () => {
  assert.deepEqual(parseFingerprint('{"mtime":123,"size":45}'), {
    mtime: 123,
    size: 45,
  });
});

test("parseFingerprint rejects malformed / partial payloads", () => {
  assert.equal(parseFingerprint("not json"), null);
  assert.equal(parseFingerprint('{"mtime":123}'), null);
  assert.equal(parseFingerprint('{"size":45}'), null);
  assert.equal(parseFingerprint('{"mtime":"x","size":1}'), null);
  assert.equal(parseFingerprint("{}"), null);
});

test("shouldRefresh: first sighting establishes baseline, no refresh", () => {
  const fp: FileFingerprint = { mtime: 1, size: 1 };
  assert.equal(shouldRefresh(null, fp), false);
});

test("shouldRefresh: identical fingerprint does not refresh", () => {
  const a: FileFingerprint = { mtime: 10, size: 20 };
  const b: FileFingerprint = { mtime: 10, size: 20 };
  assert.equal(shouldRefresh(a, b), false);
});

test("shouldRefresh: any field moving triggers a refresh", () => {
  const base: FileFingerprint = { mtime: 10, size: 20 };
  assert.equal(shouldRefresh(base, { mtime: 11, size: 20 }), true);
  assert.equal(shouldRefresh(base, { mtime: 10, size: 21 }), true);
});

test("liveLabel gives a distinct label per status", () => {
  const labels = new Set([
    liveLabel("live"),
    liveLabel("connecting"),
    liveLabel("offline"),
    liveLabel("paused"),
  ]);
  assert.equal(labels.size, 4);
});

test("liveTitle mentions the key idea per status", () => {
  assert.match(liveTitle("live"), /\.tsk\.md|auto-refresh/i);
  assert.match(liveTitle("connecting"), /connect/i);
  assert.match(liveTitle("offline"), /offline|reconnect/i);
  // F33: paused tells you it's muted + how to resume
  assert.match(liveTitle("paused"), /paus|resume/i);
});

test("liveChangeMessage describes the external edit, deferred variant differs", () => {
  assert.match(liveChangeMessage(false), /changed on disk/i);
  assert.match(liveChangeMessage(true), /deferred/i);
  assert.notEqual(liveChangeMessage(false), liveChangeMessage(true));
});

test("renderLiveIndicator carries the status class + label", () => {
  const html = renderLiveIndicator("live");
  assert.match(html, /live-live/);
  assert.match(html, /live-dot/);
  assert.match(html, />live</);
  // F33: paused renders its own class + label
  const paused = renderLiveIndicator("paused");
  assert.match(paused, /live-paused/);
  assert.match(paused, />paused</);
});
