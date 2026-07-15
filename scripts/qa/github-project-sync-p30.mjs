import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import http from "node:http";
import os from "node:os";
import path from "node:path";

const repoRoot = path.resolve(import.meta.dirname, "../..");
const scriptPath = path.join(repoRoot, "scripts", "sync-github-projects.mjs");
const temporaryDirectory = await fs.mkdtemp(path.join(os.tmpdir(), "zoking-github-sync-"));
const outputPath = path.join(temporaryDirectory, "github_projects.json");
let responseStatus = 200;
let receivedAuthorization = "";

const repository = (name, pushedAt, overrides = {}) => ({
  name,
  description: `${name} description`,
  language: "Go",
  stargazers_count: 3,
  forks_count: 1,
  updated_at: pushedAt,
  pushed_at: pushedAt,
  html_url: `https://github.com/zo-king/${name}`,
  fork: false,
  archived: false,
  owner: { login: "must-not-be-written" },
  clone_url: "https://example.invalid/must-not-be-written",
  ...overrides,
});

const fixture = [
  repository("older", "2026-01-01T00:00:00Z"),
  repository("latest", "2026-07-15T00:00:00Z", { language: "TypeScript", stargazers_count: 8 }),
  repository("unsafe-url", "2026-07-14T00:00:00Z", { html_url: "javascript:alert(1)" }),
  repository("fork-hidden", "2026-07-16T00:00:00Z", { fork: true }),
  repository("archived-hidden", "2026-07-17T00:00:00Z", { archived: true }),
];

const server = http.createServer((request, response) => {
  receivedAuthorization = request.headers.authorization || "";
  if (responseStatus !== 200) {
    response.writeHead(responseStatus, { "content-type": "application/json" });
    response.end(JSON.stringify({ message: "fixture failure" }));
    return;
  }
  assert.equal(request.url, "/users/zo-king/repos?type=owner&sort=pushed&direction=desc&per_page=100");
  response.writeHead(200, { "content-type": "application/json" });
  response.end(JSON.stringify(fixture));
});

await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const address = server.address();
if (!address || typeof address === "string") throw new Error("failed to start fixture server");

const runSync = () => new Promise((resolve) => {
  const child = spawn(process.execPath, [scriptPath, "--username", "zo-king", "--output", outputPath], {
    cwd: repoRoot,
    env: {
      ...process.env,
      GITHUB_API_BASE: `http://127.0.0.1:${address.port}`,
      GITHUB_TOKEN: "fixture-token",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let stdout = "";
  let stderr = "";
  child.stdout.on("data", (chunk) => { stdout += chunk; });
  child.stderr.on("data", (chunk) => { stderr += chunk; });
  child.on("close", (code) => resolve({ code, stdout, stderr }));
});

try {
  const success = await runSync();
  assert.equal(success.code, 0, success.stderr);
  assert.equal(receivedAuthorization, "Bearer fixture-token");

  const serialized = await fs.readFile(outputPath, "utf8");
  const snapshot = JSON.parse(serialized);
  assert.equal(snapshot.schema_version, 1);
  assert.equal(snapshot.username, "zo-king");
  assert.deepEqual(snapshot.repositories.map((item) => item.name), ["latest", "unsafe-url", "older"]);
  assert.equal(snapshot.repositories[1].html_url, "https://github.com/zo-king");
  assert(!serialized.includes("owner") && !serialized.includes("clone_url") && !serialized.includes("must-not-be-written"));

  responseStatus = 500;
  const beforeFailure = await fs.readFile(outputPath, "utf8");
  const failure = await runSync();
  assert.notEqual(failure.code, 0, "failed API request unexpectedly succeeded");
  assert.equal(await fs.readFile(outputPath, "utf8"), beforeFailure, "failed sync overwrote the last successful snapshot");

  process.stdout.write("[github-project-sync-p30] PASS auth, filter, sort, whitelist, safe URL, atomic failure retention\n");
} finally {
  await new Promise((resolve) => server.close(resolve));
  await fs.rm(temporaryDirectory, { recursive: true, force: true });
}
