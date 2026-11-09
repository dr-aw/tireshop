import { test } from "node:test";
import assert from "node:assert/strict";
import { publishContent } from "./render-content-deploy.mjs";
const config = {
  service: "srv-test",
  commit: "a".repeat(40),
  token: "synthetic-test-token",
};
function mock(values) {
  const calls = [];
  return {
    calls,
    request: async (url, options) => {
      calls.push({ url, options });
      return { ok: true, json: async () => values.shift() };
    },
    sleep: async () => {},
  };
}
test("pins the live code and waits through build until publication", async () => {
  const m = mock([
    [{ deploy: { status: "live", commit: { id: config.commit } } }],
    { id: "dep-test" },
    { status: "build_in_progress" },
    { status: "live", commit: { id: config.commit } },
  ]);
  assert.equal(await publishContent(config, m), "published");
  assert.equal(JSON.parse(m.calls[1].options.body).commitId, config.commit);
  assert.equal(m.calls.length, 4);
});
test("does not enqueue while an existing deploy is pending", async () => {
  const m = mock([[{ deploy: { status: "build_in_progress" } }]]);
  assert.equal(await publishContent(config, m), "busy");
  assert.equal(m.calls.length, 1);
});
test("refuses stale pin after a manual release", async () => {
  const m = mock([
    [{ deploy: { status: "live", commit: { id: "b".repeat(40) } } }],
  ]);
  await assert.rejects(publishContent(config, m), /differs/);
  assert.equal(m.calls.length, 1);
});
test("fails on failed or timed-out publication without duplicate POST", async () => {
  for (const status of ["build_failed", "build_in_progress"]) {
    const m = mock([
      [{ deploy: { status: "live", commit: { id: config.commit } } }],
      { id: "dep-test" },
      { status },
    ]);
    await assert.rejects(publishContent(config, { ...m, polls: 1 }));
    assert.equal(m.calls.filter((c) => c.options.method === "POST").length, 1);
  }
});
