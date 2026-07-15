import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.GITHUB_P19_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.GITHUB_P19_EVIDENCE_DIR || "docs/process/evidence");
const snapshot = JSON.parse(await fs.readFile("apps/site/data/github_projects.json", "utf8"));
const expectedRepositories = snapshot.repositories.slice(0, 6);

const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

const collectErrors = (page) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(`console: ${message.text()}`);
  });
  return errors;
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN", reducedMotion: "reduce" });
  const page = await context.newPage();
  const errors = collectErrors(page);
  let githubApiRequests = 0;
  page.on("request", (request) => {
    if (request.url().startsWith("https://api.github.com/")) githubApiRequests += 1;
  });

  await page.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  const menuText = await page.locator("#main-menu").innerText();
  assert(menuText.includes("项目"), `project navigation is missing: ${menuText}`);
  assert(!menuText.includes("分类") && !menuText.includes("标签"), `taxonomy links still occupy primary navigation: ${menuText}`);
  assert((await page.locator('.menu-social a[href="https://github.com/zo-king"]').count()) === 1, "confirmed GitHub profile icon is missing");

  for (const routePath of ["/categories/", "/tags/"]) {
    const response = await page.goto(`${siteBase}${routePath}`, { waitUntil: "networkidle" });
    assert(response?.status() === 200, `${routePath} no longer returns 200`);
  }

  await page.goto(`${siteBase}/projects/`, { waitUntil: "networkidle" });
  const cards = page.locator(".github-project");
  assert((await cards.count()) === expectedRepositories.length, `expected ${expectedRepositories.length} repository cards, got ${await cards.count()}`);
  assert(githubApiRequests === 0, `visitor page made ${githubApiRequests} GitHub API requests`);
  assert((await page.locator('script[src*="githubProjects"]').count()) === 0, "legacy browser-side GitHub script is still present");
  assert((await page.getByText(/数据更新于 \d{4}年\d{1,2}月\d{1,2}日/).count()) === 1, "snapshot update date is missing");
  assert((await page.getByText("正在读取 GitHub 仓库...", { exact: true }).count()) === 0, "legacy loading state is still visible");

  const names = (await cards.locator("h2").allTextContents()).map((name) => name.trim());
  assert(JSON.stringify(names) === JSON.stringify(expectedRepositories.map((repository) => repository.name)), `snapshot order differs from data file: ${names.join(", ")}`);
  for (let index = 0; index < expectedRepositories.length; index += 1) {
    const link = cards.nth(index).locator("h2 a");
    assert((await link.getAttribute("href")) === expectedRepositories[index].html_url, `repository ${index} URL mismatch`);
    assert((await link.getAttribute("rel")) === "noopener noreferrer", `repository ${index} external link rel is unsafe`);
  }
  assert(errors.length === 0, `project runtime errors: ${errors.join("; ")}`);
  await page.screenshot({ path: path.join(evidenceDir, "github-p30-projects-static-1280x800.png"), fullPage: false });
  await context.close();

  for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 568 }]) {
    const mobileContext = await browser.newContext({ viewport, locale: "zh-CN", reducedMotion: "reduce" });
    const mobilePage = await mobileContext.newPage();
    await mobilePage.goto(`${siteBase}/projects/`, { waitUntil: "networkidle" });
    const width = await mobilePage.evaluate(() => ({ document: document.documentElement.scrollWidth, viewport: innerWidth }));
    assert(width.document <= width.viewport + 1, `${viewport.width}px project page overflows: ${JSON.stringify(width)}`);
    assert((await mobilePage.locator(".github-projects__grid").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)) === 1, `${viewport.width}px project grid is not single-column`);
    if (viewport.width === 390) {
      await mobilePage.screenshot({ path: path.join(evidenceDir, "github-p30-projects-static-390x844.png"), fullPage: false });
    }
    await mobileContext.close();
  }

  process.stdout.write(`[github-p19] PASS static snapshot, zero visitor API requests, ${expectedRepositories.length} cards, 390/320 responsive\n`);
} finally {
  await browser.close();
}
