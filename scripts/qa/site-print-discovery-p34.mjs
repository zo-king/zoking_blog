import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P34_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P34_EVIDENCE_DIR || "docs/process/evidence");
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

const installStablePageState = async (context, scheme = "light") => {
  await context.addInitScript((colorScheme) => {
    sessionStorage.setItem("zoking-blog:splash-seen", "1");
    localStorage.setItem("StackColorScheme", colorScheme);
  }, scheme);
  await context.route("**/api/v1/public/posts/**/comments", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ data: [] }),
  }));
};

const rssLinks = (page) => page.locator('head link[rel="alternate"][type="application/rss+xml"]');

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 794, height: 1123 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installStablePageState(context);
  const page = await context.newPage();
  const errors = collectErrors(page);
  await page.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });

  assert((await page.locator('link[media="print"][href*="/scss/print"]').count()) === 1, "article page did not load one print-only stylesheet");
  assert((await rssLinks(page).count()) === 1, "article page must expose exactly one RSS discovery link");
  const articleFeed = await rssLinks(page).first().evaluate((element) => ({ href: element.href, title: element.title, type: element.type }));
  assert(new URL(articleFeed.href).pathname === "/index.xml" && articleFeed.title === "Zoking 博客 RSS" && articleFeed.type === "application/rss+xml", `article RSS discovery is invalid: ${JSON.stringify(articleFeed)}`);

  const posting = await page.locator('script[type="application/ld+json"]').evaluateAll((scripts) => scripts
    .map((script) => JSON.parse(script.textContent || "null"))
    .find((entry) => entry?.["@type"] === "BlogPosting"));
  assert(posting?.author?.name === "Zoking" && posting?.author?.url === "https://github.com/zo-king", `BlogPosting author is invalid: ${JSON.stringify(posting?.author)}`);
  assert(posting?.publisher?.name === "Zoking" && posting?.publisher?.["@type"] === "Person", `BlogPosting publisher is invalid: ${JSON.stringify(posting?.publisher)}`);
  assert(posting?.datePublished === "2026-07-13T10:30:00+08:00" && posting?.dateModified === "2026-07-13T10:30:00+08:00", "BlogPosting dates changed unexpectedly");

  assert(await page.locator(".left-sidebar").isVisible(), "screen layout lost the left sidebar");
  assert(await page.locator(".article-print-byline").isHidden(), "print byline leaked into screen layout");
  await page.emulateMedia({ media: "print" });

  for (const selector of [
    ".site-splash",
    ".site-topbar",
    ".left-sidebar",
    ".right-sidebar",
    ".reading-progress",
    ".article-navigation",
    ".public-comments",
    ".site-footer",
    ".code-block-toolbar",
  ]) {
    const locator = page.locator(selector).first();
    if ((await locator.count()) === 0) continue;
    const display = await locator.evaluate((element) => getComputedStyle(element).display);
    assert(display === "none", `${selector} remains visible in print: ${display}`);
  }

  assert(await page.locator(".main-article > .article-header .article-title").isVisible(), "article title is hidden in print");
  assert(await page.locator(".article-content").isVisible(), "article content is hidden in print");
  assert(await page.locator(".article-copyright").isVisible(), "article license is hidden in print");
  assert(await page.locator(".article-print-byline").isVisible(), "print author byline is missing");
  assert((await page.locator(".article-print-byline").textContent())?.trim() === "作者：Zoking", "print author byline is incorrect");
  assert((await page.locator(".article-time--published").textContent())?.includes("2026年7月13日"), "published date is missing from print");

  const printStyles = await page.evaluate(() => {
    const body = getComputedStyle(document.body);
    const article = getComputedStyle(document.querySelector(".main-article"));
    const code = getComputedStyle(document.querySelector(".article-content .highlight code[data-lang]"));
    return {
      bodyBackground: body.backgroundColor,
      bodyColor: body.color,
      articleBackground: article.backgroundColor,
      articleShadow: article.boxShadow,
      codeWhiteSpace: code.whiteSpace,
      codeOverflow: code.overflow,
    };
  });
  assert(printStyles.bodyBackground === "rgb(255, 255, 255)" && printStyles.articleBackground === "rgb(255, 255, 255)", `print background is not white: ${JSON.stringify(printStyles)}`);
  assert(printStyles.articleShadow === "none", `article shadow remains in print: ${printStyles.articleShadow}`);
  assert(printStyles.codeWhiteSpace === "pre-wrap" && printStyles.codeOverflow === "visible", `code is not print-safe: ${JSON.stringify(printStyles)}`);
  const dimensions = await page.locator("html").evaluate((element) => ({
    scrollWidth: element.scrollWidth,
    viewportWidth: innerWidth,
    overflowers: Array.from(document.querySelectorAll("body *"))
      .map((node) => ({ node, bounds: node.getBoundingClientRect() }))
      .filter(({ bounds }) => bounds.right > innerWidth + 1 || bounds.left < -1)
      .slice(0, 8)
      .map(({ node, bounds }) => ({ tag: node.tagName, className: node.className?.toString() || "", left: bounds.left, right: bounds.right, width: bounds.width })),
  }));
  assert(dimensions.scrollWidth <= dimensions.viewportWidth + 1, `A4 print layout overflows: ${JSON.stringify(dimensions)}`);
  assert(errors.length === 0, `print page runtime errors: ${errors.join(" | ")}`);
  await page.screenshot({ path: path.join(evidenceDir, "site-p34-print-a4-794x1123.png"), fullPage: false });

  await page.emulateMedia({ media: "screen" });
  await page.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  assert((await page.locator('link[media="print"][href*="/scss/print"]').count()) === 0, "home page loads article-only print CSS");
  assert((await rssLinks(page).count()) === 1, "home page RSS discovery was duplicated or removed");
  await page.goto(`${siteBase}/categories/technology/`, { waitUntil: "networkidle" });
  assert((await rssLinks(page).count()) === 1, "taxonomy RSS discovery was duplicated or removed");
  assert(new URL(await rssLinks(page).first().getAttribute("href"), siteBase).pathname === "/categories/technology/index.xml", "taxonomy feed was replaced by the site feed");
  await context.close();

  const darkContext = await browser.newContext({ viewport: { width: 794, height: 1123 }, locale: "zh-CN", colorScheme: "dark", reducedMotion: "reduce" });
  await installStablePageState(darkContext, "dark");
  const darkPage = await darkContext.newPage();
  await darkPage.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });
  await darkPage.emulateMedia({ media: "print", colorScheme: "dark" });
  const darkPrint = await darkPage.evaluate(() => ({
    scheme: document.documentElement.dataset.scheme,
    background: getComputedStyle(document.body).backgroundColor,
    color: getComputedStyle(document.body).color,
  }));
  assert(darkPrint.scheme === "dark" && darkPrint.background === "rgb(255, 255, 255)" && darkPrint.color === "rgb(17, 17, 17)", `dark theme leaked into print: ${JSON.stringify(darkPrint)}`);
  await darkContext.close();

  const narrowContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", javaScriptEnabled: false });
  const narrowPage = await narrowContext.newPage();
  await narrowPage.goto(`${siteBase}${articlePath}`, { waitUntil: "domcontentloaded" });
  assert((await rssLinks(narrowPage).count()) === 1, "RSS discovery depends on JavaScript");
  await narrowPage.emulateMedia({ media: "print" });
  assert((await narrowPage.locator(".site-splash").evaluate((element) => getComputedStyle(element).display)) === "none", "no-JavaScript splash remains visible in print");
  assert(await narrowPage.locator(".article-print-byline").isVisible(), "print byline depends on JavaScript");
  const narrowDimensions = await narrowPage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(narrowDimensions.scrollWidth <= narrowDimensions.viewportWidth + 1, `390px print layout overflows: ${JSON.stringify(narrowDimensions)}`);
  await narrowPage.screenshot({ path: path.join(evidenceDir, "site-p34-print-narrow-390x844.png"), fullPage: false });
  await narrowContext.close();

  process.stdout.write("[site-print-discovery-p34] PASS print isolation, A4/390, dark theme, no-JS, RSS discovery, author JSON-LD\n");
} finally {
  await browser.close();
}
