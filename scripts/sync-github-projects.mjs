import fs from "node:fs/promises";
import path from "node:path";

const args = process.argv.slice(2);

const readArgument = (name, fallback) => {
  const index = args.indexOf(name);
  if (index === -1) return fallback;
  const value = args[index + 1]?.trim();
  if (!value) throw new Error(`${name} requires a value`);
  return value;
};

const username = readArgument("--username", "zo-king");
const outputPath = path.resolve(readArgument("--output", "apps/site/data/github_projects.json"));
const apiBase = (process.env.GITHUB_API_BASE || "https://api.github.com").replace(/\/$/, "");
const token = (process.env.GITHUB_TOKEN || process.env.GH_TOKEN || "").trim();

if (!/^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$/i.test(username)) {
  throw new Error(`invalid GitHub username: ${username}`);
}

const profileUrl = `https://github.com/${username}`;
const endpoint = new URL(`${apiBase}/users/${encodeURIComponent(username)}/repos`);
endpoint.searchParams.set("type", "owner");
endpoint.searchParams.set("sort", "pushed");
endpoint.searchParams.set("direction", "desc");
endpoint.searchParams.set("per_page", "100");

const headers = {
  Accept: "application/vnd.github+json",
  "User-Agent": "zoking-blog-project-sync",
  "X-GitHub-Api-Version": "2022-11-28",
};
if (token) headers.Authorization = `Bearer ${token}`;

const response = await fetch(endpoint, { headers });
if (!response.ok) {
  const remaining = response.headers.get("x-ratelimit-remaining");
  throw new Error(`GitHub API returned ${response.status}${remaining ? ` (remaining: ${remaining})` : ""}`);
}

const payload = await response.json();
if (!Array.isArray(payload)) throw new Error("GitHub API returned an invalid repository list");

const validDate = (value) => typeof value === "string" && !Number.isNaN(Date.parse(value));
const safeRepositoryUrl = (value) => {
  try {
    const url = new URL(value);
    return url.protocol === "https:" && url.hostname === "github.com" ? url.toString() : profileUrl;
  } catch {
    return profileUrl;
  }
};

const repositories = payload
  .filter((repository) => repository && typeof repository === "object")
  .filter((repository) => repository.fork === false && repository.archived === false)
  .filter((repository) => typeof repository.name === "string" && validDate(repository.updated_at))
  .map((repository) => ({
    name: repository.name,
    description: typeof repository.description === "string" ? repository.description : null,
    language: typeof repository.language === "string" ? repository.language : null,
    stargazers_count: Number.isFinite(repository.stargazers_count) ? Math.max(0, repository.stargazers_count) : 0,
    forks_count: Number.isFinite(repository.forks_count) ? Math.max(0, repository.forks_count) : 0,
    updated_at: repository.updated_at,
    pushed_at: validDate(repository.pushed_at) ? repository.pushed_at : null,
    html_url: safeRepositoryUrl(repository.html_url),
  }))
  .sort((left, right) => {
    const leftActivity = Date.parse(left.pushed_at || left.updated_at);
    const rightActivity = Date.parse(right.pushed_at || right.updated_at);
    return rightActivity - leftActivity
      || Date.parse(right.updated_at) - Date.parse(left.updated_at)
      || left.name.localeCompare(right.name, "en");
  });

const snapshot = {
  schema_version: 1,
  username,
  profile_url: profileUrl,
  generated_at: new Date().toISOString(),
  repositories,
};

await fs.mkdir(path.dirname(outputPath), { recursive: true });
const temporaryPath = `${outputPath}.tmp`;
await fs.writeFile(temporaryPath, `${JSON.stringify(snapshot, null, 2)}\n`, "utf8");
await fs.rename(temporaryPath, outputPath);

process.stdout.write(`Wrote ${repositories.length} repositories to ${outputPath}\n`);
