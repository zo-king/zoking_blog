import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P35_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P35_EVIDENCE_DIR || "docs/process/evidence");
const articlePath = "/p/gin-production-hardening/";

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

const transparentPng = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64");

const installStablePageState = async (context) => {
  await context.addInitScript(() => {
    sessionStorage.setItem("zoking-blog:splash-seen", "1");
    localStorage.setItem("StackColorScheme", "light");
    window.__p35ShareCalls = [];
    Object.defineProperty(navigator, "share", {
      configurable: true,
      value: async (payload) => window.__p35ShareCalls.push(payload),
    });
  });
  await context.route("**/api/v1/public/posts/**/comments", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ data: [] }),
  }));
  await context.route("**/favicon.ico", (route) => route.fulfill({ status: 200, contentType: "image/png", body: transparentPng }));
  await context.route("https://icons.duckduckgo.com/**", (route) => route.fulfill({ status: 200, contentType: "image/png", body: transparentPng }));
};

const blogrollPath = path.resolve("apps/site/data/blogroll.json");
const blogroll = JSON.parse(await fs.readFile(blogrollPath, "utf8"));
const linksMarkdown = await fs.readFile(path.resolve("apps/site/content/page/links/index.md"), "utf8");
const correctionTemplate = await fs.readFile(path.resolve(".github/ISSUE_TEMPLATE/content_correction.yml"), "utf8");

assert(blogroll.schema_version === 1 && Array.isArray(blogroll.links) && blogroll.links.length === 9, "blogroll data schema or link count is invalid");
assert(!/^links\s*:/m.test(linksMarkdown), "links page still stores structured links in publishable front matter");
assert(["article_url", "section", "problem", "suggestion", "checks"].every((id) => correctionTemplate.includes(`id: ${id}`)), "content correction issue form is incomplete");

const feedLinks = blogroll.links.filter((entry) => typeof entry.feed_url === "string" && entry.feed_url.length > 0);
assert(feedLinks.length === 4, `expected four verified feeds, found ${feedLinks.length}`);
for (const entry of blogroll.links) {
  const website = new URL(entry.website);
  assert(website.protocol === "https:" && !website.username && !website.password, `unsafe blogroll website: ${entry.website}`);
  if (entry.feed_url) {
    const feed = new URL(entry.feed_url);
    assert(feed.protocol === "https:" && !feed.username && !feed.password, `unsafe blogroll feed: ${entry.feed_url}`);
  }
}

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installStablePageState(context);
  const page = await context.newPage();
  const errors = collectErrors(page);
  const requests = [];
  page.on("request", (request) => requests.push(request.url()));

  await page.goto(`${siteBase}/links/`, { waitUntil: "networkidle" });
  const cardTitles = await page.locator("[data-link-card] .links-card__title").allTextContents();
  assert(cardTitles.join("|") === blogroll.links.map((entry) => entry.title).join("|"), "links page and blogroll data diverged");
  const opmlLink = page.locator(".links-directory__opml");
  assert(await opmlLink.isVisible(), "OPML link is missing from the blogroll toolbar");
  assert((await opmlLink.getAttribute("href")) === "/blogroll.opml" && (await opmlLink.getAttribute("download")) === "zoking-blogroll.opml", "OPML download metadata is invalid");
  assert(feedLinks.every((entry) => !requests.includes(entry.feed_url)), "visitor browser requested a third-party Feed URL");

  const opml = await page.evaluate(async () => {
    const response = await fetch("/blogroll.opml");
    const text = await response.text();
    const documentXML = new DOMParser().parseFromString(text, "application/xml");
    return {
      ok: response.ok,
      contentType: response.headers.get("content-type"),
      parserErrors: documentXML.querySelectorAll("parsererror").length,
      version: documentXML.documentElement.getAttribute("version"),
      title: documentXML.querySelector("head > title")?.textContent,
      owner: documentXML.querySelector("ownerName")?.textContent,
      dateModified: documentXML.querySelector("dateModified")?.textContent,
      outlines: Array.from(documentXML.querySelectorAll("body > outline")).map((outline) => ({
        text: outline.getAttribute("text"),
        title: outline.getAttribute("title"),
        type: outline.getAttribute("type"),
        htmlUrl: outline.getAttribute("htmlUrl"),
        xmlUrl: outline.getAttribute("xmlUrl"),
        category: outline.getAttribute("category"),
      })),
    };
  });
  assert(opml.ok && opml.contentType?.startsWith("text/x-opml") && opml.parserErrors === 0 && opml.version === "2.0", `OPML response is invalid: ${JSON.stringify(opml)}`);
  assert(opml.title === "Zoking 博客 常读站点" && opml.owner === "Zoking" && opml.dateModified === "Wed, 15 Jul 2026 19:30:00 +0800", "OPML metadata is invalid");
  assert(opml.outlines.length === feedLinks.length, "OPML contains entries without verified feeds");
  opml.outlines.forEach((outline, index) => {
    const source = feedLinks[index];
    assert(outline.text === source.title && outline.title === source.title && outline.type === "rss" && outline.htmlUrl === source.website && outline.xmlUrl === source.feed_url && outline.category === source.category, `OPML entry diverged: ${JSON.stringify({ outline, source })}`);
  });
  await page.screenshot({ path: path.join(evidenceDir, "site-p35-blogroll-opml-desktop-1280x900.png"), fullPage: false });

  await page.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });
  const feedbackLink = page.locator("[data-article-feedback]");
  assert(await feedbackLink.isVisible(), "article correction link is missing");
  const feedback = new URL(await feedbackLink.getAttribute("href"));
  assert(feedback.origin === "https://github.com" && feedback.pathname === "/zo-king/zoking_blog/issues/new", `correction target is invalid: ${feedback}`);
  assert(feedback.searchParams.get("title") === "[内容纠错] Gin 生产加固：超时、恢复与可观测性", "correction title is not prefilled");
  const feedbackBody = feedback.searchParams.get("body") || "";
  assert(feedbackBody.includes(`${siteBase}${articlePath}`) && feedbackBody.includes("页面更新时间：2026-07-13") && feedbackBody.includes("章节（可选）：") && feedbackBody.includes("问题描述：") && feedbackBody.includes("建议修改："), `correction body is incomplete: ${feedbackBody}`);
  assert((await feedbackLink.getAttribute("target")) === "_blank" && (await feedbackLink.getAttribute("rel")) === "noopener noreferrer", "correction link lacks safe external-link attributes");
  assert(!requests.some((url) => url.startsWith("https://github.com/zo-king/zoking_blog/issues/new")), "GitHub was contacted before the correction link was activated");

  const shareButton = page.locator("[data-article-share]");
  const shareStatus = page.locator("[data-article-share-status]");
  await shareButton.click();
  await page.waitForFunction(() => window.__p35ShareCalls.length === 1);
  assert((await shareStatus.textContent())?.trim() === "分享操作已完成", "native share success wording is inaccurate");
  const sharePayload = await page.evaluate(() => window.__p35ShareCalls[0]);
  assert(sharePayload.title === "Gin 生产加固：超时、恢复与可观测性" && sharePayload.url === `${siteBase}${articlePath}`, `native share payload is invalid: ${JSON.stringify(sharePayload)}`);

  await page.evaluate(() => {
    Object.defineProperty(navigator, "share", { configurable: true, value: undefined });
    window.__p35ClipboardWrites = [];
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: async (value) => window.__p35ClipboardWrites.push(value) },
    });
  });
  await shareButton.click();
  await page.waitForFunction(() => window.__p35ClipboardWrites.length === 1);
  assert((await shareStatus.textContent())?.trim() === "文章链接已复制", "clipboard share fallback feedback is invalid");
  assert((await page.evaluate(() => window.__p35ClipboardWrites[0])) === `${siteBase}${articlePath}`, "clipboard share fallback copied the wrong URL");
  await feedbackLink.scrollIntoViewIfNeeded();
  await page.screenshot({ path: path.join(evidenceDir, "site-p35-article-feedback-desktop-1280x900.png"), fullPage: false });

  await page.emulateMedia({ media: "print" });
  assert((await feedbackLink.evaluate((element) => getComputedStyle(element.closest(".article-navigation")).display)) === "none", "correction action leaked into print");
  await page.emulateMedia({ media: "screen" });
  await page.goto(`${siteBase}/about/`, { waitUntil: "networkidle" });
  assert((await page.locator("[data-article-feedback]").count()) === 0, "correction action leaked onto about page");
  assert(errors.length === 0, `desktop runtime errors: ${errors.join(" | ")}`);
  await context.close();

  const mobileContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", reducedMotion: "reduce", hasTouch: true, isMobile: true });
  await installStablePageState(mobileContext);
  const mobilePage = await mobileContext.newPage();
  const mobileErrors = collectErrors(mobilePage);
  await mobilePage.goto(`${siteBase}/links/`, { waitUntil: "networkidle" });
  const mobileOpml = await mobilePage.locator(".links-directory__opml").boundingBox();
  assert(mobileOpml && mobileOpml.height >= 44, `mobile OPML target is too small: ${JSON.stringify(mobileOpml)}`);
  await mobilePage.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });
  const mobileFeedback = await mobilePage.locator("[data-article-feedback]").boundingBox();
  assert(mobileFeedback && mobileFeedback.height >= 44, `mobile correction target is too small: ${JSON.stringify(mobileFeedback)}`);
  const mobileDimensions = await mobilePage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(mobileDimensions.scrollWidth <= mobileDimensions.viewportWidth + 1, `390px article actions overflow: ${JSON.stringify(mobileDimensions)}`);
  await mobilePage.locator("[data-article-feedback]").scrollIntoViewIfNeeded();
  await mobilePage.screenshot({ path: path.join(evidenceDir, "site-p35-article-feedback-mobile-390x844.png"), fullPage: false });
  await mobilePage.setViewportSize({ width: 320, height: 568 });
  await mobilePage.reload({ waitUntil: "networkidle" });
  const narrowDimensions = await mobilePage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(narrowDimensions.scrollWidth <= narrowDimensions.viewportWidth + 1, `320px article actions overflow: ${JSON.stringify(narrowDimensions)}`);
  assert(mobileErrors.length === 0, `mobile runtime errors: ${mobileErrors.join(" | ")}`);
  await mobileContext.close();

  const noScriptContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", javaScriptEnabled: false });
  const noScriptPage = await noScriptContext.newPage();
  await noScriptPage.goto(`${siteBase}/links/`, { waitUntil: "domcontentloaded" });
  assert((await noScriptPage.locator("[data-link-card]").count()) === 9 && await noScriptPage.locator(".links-directory__opml").isVisible(), "blogroll or OPML depends on JavaScript");
  await noScriptPage.goto(`${siteBase}${articlePath}`, { waitUntil: "domcontentloaded" });
  assert(await noScriptPage.locator("[data-article-feedback]").isVisible(), "correction link depends on JavaScript");
  await noScriptContext.close();

  process.stdout.write("[site-blogroll-feedback-p35] PASS data migration, OPML XML, no Feed requests, correction link, share paths, no-JS, 1280/390/320\n");
} finally {
  await browser.close();
}
