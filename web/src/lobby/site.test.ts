import assert from "node:assert/strict";
import { test } from "node:test";
import { backendWords, calendarWords, parseSite, siteLead, versionWords } from "./site.ts";

test("parseSite reads the site's answer in the card's order", () => {
  const got = parseSite({
    environment: "dev",
    version: { commit: "4d0e5f7c0ffee4d0e5f7c0ffee4d0e5f7c0ffee4", built: "2026-11-15T10:21:08Z", modified: false },
    calendars: true,
    backends: [
      { tool: "foundry", configured: false, answered: false },
      { tool: "soccer", configured: true, answered: true },
      { tool: "weather", configured: true, answered: true },
      { tool: "minecraft", configured: true, answered: false },
      { tool: "soccer", configured: false, answered: false },
    ],
  });
  assert.ok(got);
  assert.equal(got.environment, "dev");
  assert.equal(got.version.commit, "4d0e5f7c0ffee4d0e5f7c0ffee4d0e5f7c0ffee4");
  assert.equal(got.version.built?.toISOString(), "2026-11-15T10:21:08.000Z");
  assert.equal(got.version.modified, false);
  assert.equal(got.calendars, true);
  assert.deepEqual(got.backends, [
    { tool: "soccer", configured: true, answered: true },
    { tool: "minecraft", configured: true, answered: false },
    { tool: "foundry", configured: false, answered: false },
  ]);
});

test("parseSite treats what's missing as not known", () => {
  const got = parseSite({ backends: [{ tool: "soccer" }, null, "minecraft", { tool: "foundry", configured: "yes" }] });
  assert.ok(got);
  assert.equal(got.environment, "");
  assert.deepEqual(got.version, { commit: "", built: null, modified: false });
  assert.equal(got.calendars, null);
  assert.deepEqual(got.backends, [
    { tool: "soccer", configured: null, answered: null },
    { tool: "foundry", configured: null, answered: null },
  ]);
  assert.deepEqual(parseSite({ version: { built: "not a date", modified: "true" } })?.version, { commit: "", built: null, modified: false });
  assert.deepEqual(parseSite({ backends: "soccer" })?.backends, []);
  assert.equal(parseSite(null), null);
  assert.equal(parseSite([]), null);
  assert.equal(parseSite("dev"), null);
});

test("each backend says whether it answered, isn't set up, or isn't known", () => {
  assert.equal(backendWords({ tool: "soccer", configured: true, answered: true }), "Answering");
  assert.equal(backendWords({ tool: "soccer", configured: true, answered: false }), "Not answering");
  assert.equal(backendWords({ tool: "soccer", configured: false, answered: false }), "Not set up");
  assert.equal(backendWords({ tool: "soccer", configured: true, answered: null }), "Not known");
  assert.equal(backendWords({ tool: "soccer", configured: null, answered: false }), "Not known");
  assert.equal(calendarWords(true), "Offered");
  assert.equal(calendarWords(false), "Not set up");
  assert.equal(calendarWords(null), "Not known");
});

test("the version is the short commit and when it was made, in the reader's time", () => {
  const built = new Date("2026-11-15T10:21:08Z");
  const when = built.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
  const commit = "4d0e5f7c0ffee4d0e5f7c0ffee4d0e5f7c0ffee4";
  assert.equal(versionWords({ commit, built, modified: false }), `4d0e5f7, committed ${when}`);
  assert.equal(versionWords({ commit, built, modified: true }), `4d0e5f7, committed ${when}, with local changes`);
  assert.equal(versionWords({ commit, built: null, modified: false }), "4d0e5f7");
  assert.equal(versionWords({ commit: "", built: null, modified: true }), "Not known");
});

test("siteLead names the site by its address and environment", () => {
  assert.equal(siteLead("lobby.example", "prod"), "This is lobby.example, the production site.");
  assert.equal(siteLead("dev.lobby.example", "dev"), "This is dev.lobby.example, the dev site.");
  assert.equal(siteLead("localhost:8080", "local"), "This is localhost:8080, running on a developer’s computer.");
  assert.equal(siteLead("example.com", ""), "This is example.com.");
});
