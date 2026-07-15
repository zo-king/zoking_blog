import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH;
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");

const playwrightEntry = path.join(packagePath, "index.js");
const playwrightModule = await import(pathToFileURL(playwrightEntry).href);
const { chromium } = playwrightModule.default ?? playwrightModule;
const siteBase = (process.env.SITE_P16_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.SITE_P16_EVIDENCE_DIR || "docs/process/evidence");
const results = [];

const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

const pass = (name, detail) => {
  results.push({ name, detail });
  process.stdout.write(`[site-p16] PASS ${name} - ${detail}\n`);
};

const parseColor = (value) => {
  const hex = value.trim().match(/^#([0-9a-f]{6})$/i);
  if (hex) {
    return [0, 2, 4].map((offset) => Number.parseInt(hex[1].slice(offset, offset + 2), 16));
  }
  const channels = value.match(/[\d.]+/g)?.map(Number) ?? [];
  assert(channels.length >= 3, `cannot parse color: ${value}`);
  return channels.slice(0, 3);
};

const luminance = (channels) => {
  const normalized = channels.map((value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return normalized[0] * 0.2126 + normalized[1] * 0.7152 + normalized[2] * 0.0722;
};

const contrast = (foreground, background) => {
  const first = luminance(parseColor(foreground));
  const second = luminance(parseColor(background));
  return (Math.max(first, second) + 0.05) / (Math.min(first, second) + 0.05);
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    locale: "zh-CN",
    reducedMotion: "reduce",
  });
  await context.route(/\/api\/v1\/public\/posts\/[^/]+\/comments$/, async (route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: [] }) });
      return;
    }
    await route.continue();
  });
  const page = await context.newPage();
  const runtimeErrors = [];
  page.on("pageerror", (error) => runtimeErrors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") runtimeErrors.push(`console: ${message.text()}`);
  });

  await page.goto(`${siteBase}/p/system-design-boundaries/`, { waitUntil: "networkidle" });
  const progress = page.getByRole("progressbar", { name: "阅读进度" });
  await progress.waitFor({ state: "visible" });
  const storageKey = await page.evaluate(() => `zoking:reading-progress:v1:${window.location.pathname}`);
  await page.evaluate(() => {
    const article = document.querySelector(".article-content");
    if (!article) throw new Error("article content missing");
    const start = article.getBoundingClientRect().top + window.scrollY;
    const distance = Math.max(article.scrollHeight - window.innerHeight * 0.65, 1);
    window.scrollTo(0, start + distance * 0.55);
  });
  await page.waitForTimeout(150);
  const saved = await page.evaluate((key) => JSON.parse(localStorage.getItem(key) || "null"), storageKey);
  assert(saved?.progress > 0.45 && saved?.progress < 0.7, `unexpected saved progress: ${JSON.stringify(saved)}`);
  assert(Number(await progress.getAttribute("aria-valuenow")) >= 45, "progressbar did not update");

  await page.evaluate(() => window.scrollTo(0, 0));
  await page.waitForTimeout(100);
  await page.reload({ waitUntil: "networkidle" });
  const resume = page.getByRole("button", { name: /继续阅读/ });
  await resume.waitFor({ state: "visible" });
  assert((await resume.textContent())?.includes("%"), "resume action does not expose saved percentage");
  await resume.click();
  await page.waitForTimeout(100);
  const resumedProgress = Number(await progress.getAttribute("aria-valuenow"));
  assert(resumedProgress >= 45, `resume position is too early: ${resumedProgress}%`);
  assert(await resume.isHidden(), "resume action remains visible after use");
  await page.screenshot({ path: path.join(evidenceDir, "site-p16-reading-progress-desktop.png"), fullPage: false });

  await page.evaluate(() => {
    const article = document.querySelector(".article-content");
    if (!article) throw new Error("article content missing");
    window.scrollTo(0, article.getBoundingClientRect().top + window.scrollY + article.scrollHeight);
  });
  await page.waitForTimeout(150);
  assert(await page.evaluate((key) => localStorage.getItem(key) === null, storageKey), "completed reading state was not cleared");
  await page.evaluate((key) => {
    localStorage.setItem(key, JSON.stringify({ progress: 0.5, updatedAt: Date.now() - 31 * 24 * 60 * 60 * 1000 }));
    window.scrollTo(0, 0);
  }, storageKey);
  await page.reload({ waitUntil: "networkidle" });
  assert(await resume.isHidden(), "expired reading state still exposes a resume action");
  assert(await page.evaluate((key) => localStorage.getItem(key) === null, storageKey), "expired reading state was not removed");
  pass("reading-progress-resume", "progress persists locally, resumes accurately, and clears on completion");

  await page.goto(`${siteBase}/search/?keyword=系统`, { waitUntil: "networkidle" });
  await page.locator(".search-result article").first().waitFor({ state: "visible" });
  const headings = await page.locator("main h1, main h2, main h3").evaluateAll((nodes) => nodes.map((node) => node.tagName));
  assert(headings[0] === "H1" && headings[1] === "H2" && headings.slice(2).every((tag) => tag === "H3"), `invalid search heading order: ${headings.join(",")}`);
  const missingAlt = await page.locator(".search-result img").evaluateAll((images) => images.filter((image) => !image.getAttribute("alt")?.trim()).length);
  assert(missingAlt === 0, `${missingAlt} search images have no alt text`);
  assert((await page.locator('.search-result--title[aria-live="polite"]').textContent())?.includes("结果"), "search result count is not announced");
  pass("search-accessibility", "dynamic results use H1/H2/H3 order, live status, and image alternatives");

  const failureContext = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN" });
  const failurePage = await failureContext.newPage();
  await failurePage.addInitScript(() => {
    document.addEventListener("DOMContentLoaded", () => {
      document.querySelector(".search-form")?.setAttribute("data-json", `/search/index.json?forced-failure=${Date.now()}`);
    }, { once: true });
  });
  await failurePage.route("**/*", (route) => {
    if (route.request().url().includes("/search/index.json")) return route.abort("failed");
    return route.continue();
  });
  await failurePage.goto(`${siteBase}/search/?keyword=系统`, { waitUntil: "domcontentloaded" });
  await failurePage.getByRole("heading", { name: "搜索暂时不可用" }).waitFor({ state: "visible" });
  await failurePage.unroute("**/*");
  await failurePage.getByRole("button", { name: "重新搜索" }).click();
  await failurePage.locator(".search-result article").first().waitFor({ state: "visible" });
  await failureContext.close();
  pass("search-recovery", "network failure is localized and retry recovers without reloading the page");

  await page.addInitScript(() => localStorage.setItem("StackColorScheme", "dark"));
  await page.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  const colors = await page.evaluate(() => {
    const styles = getComputedStyle(document.documentElement);
    return {
      secondary: styles.getPropertyValue("--card-text-color-secondary").trim(),
      tertiary: styles.getPropertyValue("--card-text-color-tertiary").trim(),
      background: styles.getPropertyValue("--card-background").trim(),
    };
  });
  assert(contrast(colors.secondary, colors.background) >= 4.5, `secondary contrast failed: ${JSON.stringify(colors)}`);
  assert(contrast(colors.tertiary, colors.background) >= 4.5, `tertiary contrast failed: ${JSON.stringify(colors)}`);
  const landmarkLabels = await page.locator("aside").evaluateAll((nodes) => nodes.map((node) => node.getAttribute("aria-label")));
  assert(landmarkLabels.length === 2 && new Set(landmarkLabels).size === 2 && landmarkLabels.every(Boolean), `aside labels are not unique: ${JSON.stringify(landmarkLabels)}`);
  pass("dark-contrast", "secondary and tertiary text tokens meet WCAG AA against card backgrounds");

  await page.goto(`${siteBase}/post/`, { waitUntil: "networkidle" });
  assert((await page.locator("main h1").first().textContent())?.trim() === "文章", "post section is not localized");
  await page.goto(`${siteBase}/page/`, { waitUntil: "networkidle" });
  assert((await page.locator("main h1").first().textContent())?.trim() === "页面", "page section is not localized");
  pass("localized-sections-landmarks", "section indexes are Chinese and complementary landmarks have unique names");

  const mobile = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN" });
  const mobilePage = await mobile.newPage();
  await mobilePage.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  const menuToggle = mobilePage.getByRole("button", { name: /菜单/ });
  await menuToggle.click();
  assert((await menuToggle.getAttribute("aria-expanded")) === "true", "mobile menu did not open");
  await mobilePage.keyboard.press("Escape");
  assert((await menuToggle.getAttribute("aria-expanded")) === "false", "Escape did not close mobile menu");
  assert(await menuToggle.evaluate((element) => element === document.activeElement), "focus did not return to menu toggle");
  await menuToggle.click();
  await mobilePage.evaluate(() => document.querySelector("main")?.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true })));
  assert((await menuToggle.getAttribute("aria-expanded")) === "false", "outside pointer did not close mobile menu");
  const overflow = await mobilePage.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(overflow <= 1, `mobile page overflows by ${overflow}px`);
  await mobilePage.screenshot({ path: path.join(evidenceDir, "site-p16-mobile-home-390x844.png"), fullPage: false });
  await mobile.close();
  pass("mobile-menu", "Escape and outside pointer close the menu, focus returns, and 390px layout does not overflow");

  assert(runtimeErrors.length === 0, `runtime errors before the intentional network abort: ${runtimeErrors.join(" | ")}`);
  runtimeErrors.length = 0;
  await page.goto(`${siteBase}/p/system-design-boundaries/`, { waitUntil: "networkidle" });
  await page.route(/\/api\/v1\/public\/posts\/[^/]+\/comments$/, async (route) => {
    if (route.request().method() === "POST") await route.abort("failed");
    else await route.continue();
  });
  await page.locator('input[name="author_name"]').fill("测试读者");
  await page.locator('textarea[name="content"]').fill("此请求会被浏览器测试拦截，不会写入数据库。");
  await page.getByRole("button", { name: "提交评论" }).click();
  const notice = page.locator("[data-comments-notice]");
  await notice.waitFor({ state: "visible" });
  assert((await notice.textContent())?.includes("网络连接失败"), `comment error is not localized: ${await notice.textContent()}`);
  assert(!(await notice.textContent())?.includes("Failed to fetch"), "raw browser error leaked into comment UI");
  pass("comment-network-error", "aborted POST writes no data and exposes a localized live-region message");

  const unexpectedErrors = runtimeErrors.filter((message) => !message.includes("net::ERR_FAILED"));
  assert(unexpectedErrors.length === 0, `unexpected runtime errors: ${unexpectedErrors.join(" | ")}`);
  await context.close();
} finally {
  await browser.close();
}

process.stdout.write(`${JSON.stringify({ ok: true, passed: results.length, results }, null, 2)}\n`);
