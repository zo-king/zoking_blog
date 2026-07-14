import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.GITHUB_P19_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.GITHUB_P19_EVIDENCE_DIR || "docs/process/evidence");
const apiPattern = "https://api.github.com/users/zo-king/repos**";

const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

const repository = (name, pushedAt, overrides = {}) => ({
  name,
  description: `${name} 的项目说明`,
  language: "Go",
  stargazers_count: 1,
  forks_count: 0,
  updated_at: pushedAt,
  pushed_at: pushedAt,
  html_url: `https://github.com/zo-king/${name}`,
  fork: false,
  archived: false,
  owner: { login: "should-not-enter-cache" },
  clone_url: "https://example.invalid/should-not-enter-cache",
  ...overrides,
});

const fixture = [
  repository("oldest-visible", "2026-01-01T00:00:00Z"),
  repository("alpha-latest", "2026-07-13T09:00:00Z", { language: "TypeScript", stargazers_count: 5 }),
  repository("<img src=x onerror=alert(1)>", "2026-07-12T09:00:00Z", { description: null, language: null, html_url: "javascript:alert(1)" }),
  repository("third", "2026-07-11T09:00:00Z", { language: "Python" }),
  repository("fourth", "2026-07-10T09:00:00Z", { language: "Vue" }),
  repository("fifth", "2026-07-09T09:00:00Z"),
  repository("sixth", "2026-07-08T09:00:00Z"),
  repository("fork-hidden", "2026-07-14T09:00:00Z", { fork: true }),
  repository("archived-hidden", "2026-07-15T09:00:00Z", { archived: true }),
];

const collectErrors = (page) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(`console: ${message.text()}`);
  });
  return errors;
};

const fulfillJSON = (route, data, status = 200) => route.fulfill({
  status,
  contentType: "application/json",
  body: JSON.stringify(data),
});

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN", reducedMotion: "reduce" });
  let githubRequests = 0;
  await context.route(apiPattern, async (route) => {
    githubRequests += 1;
    const headers = route.request().headers();
    assert(!headers.authorization, "GitHub request leaked an Authorization header");
    assert(!headers.cookie, "GitHub request leaked a Cookie header");
    assert(!headers.referer, "GitHub request leaked a Referer header");
    await fulfillJSON(route, fixture);
  });

  const page = await context.newPage();
  const errors = collectErrors(page);
  await page.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  assert(githubRequests === 0, "homepage unexpectedly called GitHub");
  const menuText = await page.locator("#main-menu").innerText();
  assert(menuText.includes("归档") && menuText.includes("项目"), `primary discovery links missing: ${menuText}`);
  assert(!menuText.includes("分类") && !menuText.includes("标签"), `taxonomy links still occupy primary navigation: ${menuText}`);
  assert((await page.locator('.menu-social a[href="https://github.com/zo-king"]').count()) === 1, "confirmed GitHub profile icon is missing");

  await page.goto(`${siteBase}/archives/`, { waitUntil: "networkidle" });
  assert((await page.getByRole("navigation", { name: "按分类浏览" }).count()) === 1, "archive category navigation is missing");
  assert((await page.getByRole("navigation", { name: "按标签浏览" }).count()) === 1, "archive tag navigation is missing");
  assert((await page.locator('.archives-year').count()) >= 1, "archive year groups are missing");
  await page.screenshot({ path: path.join(evidenceDir, "github-p19-archives-desktop-1280x800.png"), fullPage: false });

  for (const routePath of ["/categories/", "/tags/"]) {
    const response = await page.goto(`${siteBase}${routePath}`, { waitUntil: "networkidle" });
    assert(response?.status() === 200, `${routePath} no longer returns 200`);
  }

  await page.goto(`${siteBase}/projects/`, { waitUntil: "networkidle" });
  await page.getByText("已加载 6 个最近更新的公开仓库。", { exact: true }).waitFor({ state: "visible" });
  const cards = page.locator(".github-project");
  assert((await cards.count()) === 6, `expected 6 repository cards, got ${await cards.count()}`);
  const names = await cards.locator("h2").allTextContents();
  assert(names[0] === "alpha-latest", `repository order is unstable: ${names.join(",")}`);
  assert(!names.includes("fork-hidden") && !names.includes("archived-hidden") && !names.includes("oldest-visible"), `filter/max count failed: ${names.join(",")}`);
  assert((await cards.locator("img").count()) === 0, "GitHub text was interpreted as injected HTML");
  assert((await cards.getByText("暂无项目简介，可前往 GitHub 查看 README 和最新进展。", { exact: true }).count()) === 1, "missing-description fallback is absent");
  const unsafeCardLink = cards.filter({ hasText: "<img src=x onerror=alert(1)>" }).locator("h2 a");
  assert((await unsafeCardLink.getAttribute("href")) === "https://github.com/zo-king", "unsafe repository URL did not fall back to profile");
  assert((await unsafeCardLink.getAttribute("rel")) === "noopener noreferrer", "repository external link rel is unsafe");
  assert((await unsafeCardLink.getAttribute("aria-label"))?.includes("新窗口"), "repository external link does not announce new window");
  const cache = await page.evaluate(() => sessionStorage.getItem("zo-king:github-projects:v1:zo-king") || "");
  assert(cache.includes("alpha-latest"), "short-term repository cache was not written");
  assert(!cache.includes("owner") && !cache.includes("clone_url") && !cache.includes("should-not-enter-cache"), "cache contains non-whitelisted GitHub fields");
  assert(errors.length === 0, `project success runtime errors: ${errors.join("; ")}`);
  await page.screenshot({ path: path.join(evidenceDir, "github-p19-projects-desktop-1280x800.png"), fullPage: false });
  await context.close();

  const emptyContext = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN" });
  await emptyContext.route(apiPattern, (route) => fulfillJSON(route, []));
  const emptyPage = await emptyContext.newPage();
  await emptyPage.goto(`${siteBase}/projects/`, { waitUntil: "networkidle" });
  await emptyPage.getByText("暂时没有可展示的公开仓库", { exact: true }).waitFor({ state: "visible" });
  assert((await emptyPage.getByRole("link", { name: /GitHub 主页/ }).count()) >= 1, "empty state has no profile fallback");
  await emptyContext.close();

  const retryContext = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN" });
  let retryRequests = 0;
  await retryContext.route(apiPattern, (route) => {
    retryRequests += 1;
    return retryRequests === 1 ? fulfillJSON(route, { message: "rate limited" }, 403) : fulfillJSON(route, fixture);
  });
  const retryPage = await retryContext.newPage();
  const retryErrors = collectErrors(retryPage);
  await retryPage.goto(`${siteBase}/projects/`, { waitUntil: "networkidle" });
  await retryPage.getByText("暂时无法读取 GitHub 仓库", { exact: true }).waitFor({ state: "visible" });
  assert(retryRequests === 1, `failure state retried automatically: ${retryRequests}`);
  await retryPage.getByRole("button", { name: "重新加载" }).click();
  await retryPage.getByText("已加载 6 个最近更新的公开仓库。", { exact: true }).waitFor({ state: "visible" });
  assert(retryRequests === 2, `manual retry request count is wrong: ${retryRequests}`);
  const unexpectedRetryErrors = retryErrors.filter((message) => !message.includes("403 (Forbidden)"));
  assert(unexpectedRetryErrors.length === 0, `retry runtime errors: ${unexpectedRetryErrors.join("; ")}`);
  await retryContext.close();

  for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 568 }]) {
    const mobileContext = await browser.newContext({ viewport, locale: "zh-CN", reducedMotion: "reduce" });
    await mobileContext.route(apiPattern, (route) => fulfillJSON(route, fixture));
    const mobilePage = await mobileContext.newPage();
    await mobilePage.goto(`${siteBase}/projects/`, { waitUntil: "networkidle" });
    await mobilePage.getByText("已加载 6 个最近更新的公开仓库。", { exact: true }).waitFor({ state: "visible" });
    const width = await mobilePage.evaluate(() => ({ document: document.documentElement.scrollWidth, viewport: innerWidth }));
    assert(width.document <= width.viewport + 1, `${viewport.width}px project page overflows: ${JSON.stringify(width)}`);
    assert((await mobilePage.locator(".github-projects__grid").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)) === 1, `${viewport.width}px project grid is not single-column`);
    if (viewport.width === 390) {
      await mobilePage.screenshot({ path: path.join(evidenceDir, "github-p19-projects-mobile-390x844.png"), fullPage: false });
    }
    await mobileContext.close();
  }

  process.stdout.write("[github-p19] PASS navigation, archive discovery, GitHub success/cache/security, empty/retry, 390/320 responsive\n");
} finally {
  await browser.close();
}
