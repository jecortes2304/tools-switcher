import test from "node:test";
import assert from "node:assert/strict";
import {
  validTitle,
  consistentVersion,
  releasePlan,
  main,
} from "./release.mjs";

test("classifies feature PR titles and permits only develop promotions", () => {
  const pr = {
    base: { ref: "develop" },
    head: { ref: "feature/wizard" },
    title: "feat(updater): añadir asistente",
  };
  assert.equal(validTitle(pr), true);
  assert.equal(validTitle({ ...pr, title: "añadir asistente" }), false);
  assert.equal(validTitle({ ...pr, title: "feat!: cambiar formato" }), true);
  assert.equal(validTitle({ ...pr, base: { ref: "main" } }), false);
  assert.equal(
    validTitle({ ...pr, base: { ref: "main" }, head: { ref: "develop" } }),
    true,
  );
});

test("detects stale or partially edited versions", () => {
  const lock = { version: "0.2.0", packages: { "": { version: "0.2.0" } } };
  assert.equal(
    consistentVersion("0.2.0", { ".": "0.2.0" }, { version: "0.2.0" }, lock),
    true,
  );
  assert.equal(
    consistentVersion("0.2.0", { ".": "0.1.0" }, { version: "0.2.0" }, lock),
    false,
  );
  assert.equal(
    consistentVersion(
      "0.2.0",
      { ".": "0.2.0" },
      { version: "0.2.0" },
      { ...lock, packages: { "": { version: "0.1.0" } } },
    ),
    false,
  );
});

test("does not open another version before a prepared release is published", () => {
  assert.equal(releasePlan("0.2.0", "0.1.0", true).ready, false);
  assert.equal(releasePlan("0.1.0", "0.1.0", true).ready, true);
  assert.throws(() => releasePlan("0.1.0", "0.1.0", false), /Sincroniza/);
  assert.throws(() => releasePlan("0.1.0", "0.2.0", true), /detrás/);
});

test("sync preserves newer develop commits and creates a reviewable PR", async (t) => {
  const previous = globalThis.fetch;
  const variables = {
    GH_TOKEN: "test-token",
    GITHUB_REPOSITORY: "example/tools",
    RELEASE_VERSION: "0.2.0",
    GITHUB_SHA: "published-sha",
  };
  const oldEnv = Object.fromEntries(
    Object.keys(variables).map((key) => [key, process.env[key]]),
  );
  Object.assign(process.env, variables);
  t.after(() => {
    globalThis.fetch = previous;
    for (const [key, value] of Object.entries(oldEnv)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  });
  const calls = [];
  globalThis.fetch = async (url, options) => {
    const path = new URL(url).pathname.replace("/repos/example/tools", "");
    calls.push({
      path,
      method: options.method,
      body: options.body && JSON.parse(options.body),
    });
    let data = [];
    if (path.startsWith("/compare/")) data = { status: "diverged" };
    if (path === "/pulls" && options.method === "POST")
      data = { html_url: "https://example.com/pr/1" };
    return new Response(JSON.stringify(data), { status: 200 });
  };
  await main("sync");
  assert.equal(
    calls.some((call) => call.path.includes("/git/refs/")),
    false,
  );
  assert.deepEqual(
    calls.find((call) => call.path === "/pulls" && call.method === "POST").body
      .head,
    "main",
  );
});

test("sync advances develop without force when it has no new commits", async (t) => {
  const previous = globalThis.fetch;
  const values = {
    GH_TOKEN: "test-token",
    GITHUB_REPOSITORY: "example/tools",
    RELEASE_VERSION: "0.2.0",
    GITHUB_SHA: "published-sha",
  };
  const oldEnv = Object.fromEntries(
    Object.keys(values).map((key) => [key, process.env[key]]),
  );
  Object.assign(process.env, values);
  t.after(() => {
    globalThis.fetch = previous;
    for (const [key, value] of Object.entries(oldEnv)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  });
  const calls = [];
  globalThis.fetch = async (url, options) => {
    const path = new URL(url).pathname.replace("/repos/example/tools", "");
    calls.push({
      path,
      method: options.method,
      body: options.body && JSON.parse(options.body),
    });
    const data = path.startsWith("/compare/") ? { status: "ahead" } : [];
    return new Response(JSON.stringify(data), { status: 200 });
  };
  await main("sync");
  assert.deepEqual(calls.find((call) => call.method === "PATCH").body, {
    sha: "published-sha",
    force: false,
  });
  assert.equal(
    calls.some((call) => call.path.includes("/release-please.yml/dispatches")),
    true,
  );
});

test("publishing rejects changes after the prepared release candidate", async (t) => {
  const { readFile } = await import("node:fs/promises");
  const version = (await readFile("VERSION", "utf8")).trim();
  const previous = globalThis.fetch;
  t.after(() => {
    globalThis.fetch = previous;
  });
  globalThis.fetch = async (url) => {
    const path = new URL(url).pathname;
    const data = path.endsWith("/pulls")
      ? [
          {
            merged_at: "2026-11-01",
            head: { ref: "release-please--branches--develop" },
            merge_commit_sha: "candidate",
          },
        ]
      : path.endsWith("/contents/VERSION")
        ? { content: Buffer.from(version).toString("base64") }
        : { files: [{ filename: "unprepared-feature.go" }] };
    return new Response(JSON.stringify(data), { status: 200 });
  };
  await assert.rejects(main("publish"), /posteriores/);
});
