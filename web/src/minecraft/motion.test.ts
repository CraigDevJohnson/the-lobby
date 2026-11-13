import assert from "node:assert/strict";
import { test } from "node:test";
import { RISE_AFTER_MS, SETTLE_MS, settles, tracks } from "./motion.ts";

test("a grid's rows are read as the browser reports the ones in use", () => {
  assert.deepEqual(tracks("137px 79.2031px 82px 155px"), [137, 79.2031, 82, 155]);
  assert.deepEqual(tracks("64px"), [64]);
  assert.deepEqual(tracks(" 0px 20.5px "), [0, 20.5]);
});

test("rows the page can't read are never guessed at", () => {
  // A world's column shares the board's rows side by side: Chrome reports
  // "subgrid" for it and Firefox "subgrid [] [] [] [] []".
  const unread = ["none", "subgrid", "subgrid [] [] [] [] []", "", "auto auto", "[head] 137px 79px", "repeat(4, auto)", "12em 4px"];
  for (const value of unread) assert.equal(tracks(value), null, value);
});

test("a row that grew or shrank settles", () => {
  // Stop's question takes the button's place.
  assert.equal(settles("137px 50px 64px 155px", "137px 50px 126px 155px"), true);
  // Bedrock adds a port under every address.
  assert.equal(settles("137px 79px 82px 155px", "137px 79px 82px 272px"), true);
  // A box settles its height the same way.
  assert.equal(settles("28px", "56px"), true);
});

test("rows that did not change stay still", () => {
  assert.equal(settles("137px 79px 82px 155px", "137px 79px 82px 155px"), false);
  // Less than the eye can see.
  assert.equal(settles("137px 79.2px 82px 155px", "137px 79.6px 82px 155px"), false);
});

test("a change the page can't follow row by row cuts", () => {
  // A world was added or taken away, so the rows are not the same rows.
  assert.equal(settles("137px 79px", "137px 79px 82px"), false);
  // The worlds went from side by side to stacked, or the reverse.
  assert.equal(settles("137px 79px 82px 155px", "none"), false);
  assert.equal(settles("subgrid", "97px 78px 82px 155px"), false);
  assert.equal(settles("subgrid", "subgrid"), false);
});

test("a control rises only as its row finishes settling", () => {
  assert.ok(RISE_AFTER_MS < SETTLE_MS);
  assert.ok(RISE_AFTER_MS > SETTLE_MS / 2);
});
