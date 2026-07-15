import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P36_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P36_EVIDENCE_DIR || "docs/process/evidence");

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

const assertNoOverflow = async (page, label) => {
  const dimensions = await page.locator("html").evaluate((element) => ({
    scrollWidth: element.scrollWidth,
    viewportWidth: window.innerWidth,
  }));
  assert(dimensions.scrollWidth <= dimensions.viewportWidth + 1, `${label} overflows: ${JSON.stringify(dimensions)}`);
};

const assertArchiveStructure = async (page) => {
  assert((await page.locator("main h1").count()) === 1, "post archive must have exactly one main H1");
  assert((await page.locator("main h1").innerText()) === "文章", "post archive H1 is not Chinese");
  assert((await page.locator(".post-archive-list").count()) === 1, "post archive list is missing");

  const years = await page.locator(".post-archive-year__header h2").allTextContents();
  assert(years.length > 0, "post archive has no year groups");
  assert(years.every((year) => /^20\d{2}$/.test(year.trim())), `invalid year labels: ${JSON.stringify(years)}`);
  const numericYears = years.map(Number);
  assert(numericYears.every((year, index) => index === 0 || year <= numericYears[index - 1]), `years are not descending: ${years}`);

  const navYears = await page.locator(".post-archive-years a").evaluateAll((links) =>
    links.map((link) => link.getAttribute("href")),
  );
  assert(JSON.stringify(navYears) === JSON.stringify(years.map((year) => `#year-${year.trim()}`)), "year navigation does not match groups");

  const semantics = await page.locator(".post-archive-entries").evaluateAll((lists) => lists.map((list) => ({
    tag: list.tagName,
    children: [...list.children].map((child) => child.tagName),
    articles: list.querySelectorAll(":scope > li > article").length,
  })));
  assert(semantics.every((item) => item.tag === "OL"), "year entries must use ordered lists");
  assert(semantics.every((item) => item.children.every((tag) => tag === "LI")), "archive lists must contain direct list items");
  assert(semantics.every((item) => item.articles === item.children.length), "each archive item must contain an article");

  const entries = page.locator(".post-archive-entry");
  const entryCount = await entries.count();
  assert(entryCount >= 10, `expected a populated archive, received ${entryCount} entries`);
  const summaryText = await page.locator(".post-archive-header p").innerText();
  assert(summaryText.includes(`共 ${entryCount} 篇`), `archive total does not match entries: ${summaryText}`);

  for (const group of await page.locator(".post-archive-year").all()) {
    const dates = await group.locator("time").evaluateAll((elements) => elements.map((element) => element.dateTime));
    const timestamps = dates.map((date) => Date.parse(date));
    assert(timestamps.every((value) => Number.isFinite(value)), `invalid archive dates: ${JSON.stringify(dates)}`);
    assert(timestamps.every((value, index) => index === 0 || value <= timestamps[index - 1]), `entries are not descending: ${JSON.stringify(dates)}`);
  }

  const destinations = await entries.locator("h3 a").evaluateAll((links) => links.map((link) => link.getAttribute("href")));
  assert(destinations.every((href) => href?.startsWith("/p/")), `invalid article destinations: ${JSON.stringify(destinations)}`);
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const desktop = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    locale: "zh-CN",
    colorScheme: "light",
    reducedMotion: "no-preference",
  });
  await desktop.addInitScript(() => sessionStorage.setItem("zoking-blog:splash-seen", "1"));
  const page = await desktop.newPage();
  const errors = collectErrors(page);
  const thirdPartyRequests = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.origin !== new URL(siteBase).origin) thirdPartyRequests.push(request.url());
  });
  const response = await page.goto(`${siteBase}/post/`, { waitUntil: "networkidle" });
  assert(response?.status() === 200, "/post/ did not return 200");
  await assertArchiveStructure(page);
  await assertNoOverflow(page, "desktop post archive");
  assert(thirdPartyRequests.length === 0, `post archive made third-party requests: ${thirdPartyRequests.join(" | ")}`);

  const firstYearLink = page.locator(".post-archive-years a").first();
  const firstYearTarget = await firstYearLink.getAttribute("href");
  await firstYearLink.click();
  assert(new URL(page.url()).hash === firstYearTarget, "year navigation did not update the URL hash");
  const animationName = await page.locator(firstYearTarget).evaluate((element) => getComputedStyle(element).animationName);
  assert(animationName.includes("post-archive-focus"), `year target animation is missing: ${animationName}`);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.waitForTimeout(950);
  await page.screenshot({ path: path.join(evidenceDir, "site-p36-post-archive-desktop-1280x900.png"), fullPage: true });
  assert(errors.length === 0, `desktop runtime errors: ${errors.join(" | ")}`);
  await desktop.close();

  for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 568 }]) {
    const mobile = await browser.newContext({ viewport, locale: "zh-CN", reducedMotion: "reduce" });
    await mobile.addInitScript(() => sessionStorage.setItem("zoking-blog:splash-seen", "1"));
    const mobilePage = await mobile.newPage();
    const mobileErrors = collectErrors(mobilePage);
    await mobilePage.goto(`${siteBase}/post/`, { waitUntil: "networkidle" });
    await assertArchiveStructure(mobilePage);
    await assertNoOverflow(mobilePage, `${viewport.width}px post archive`);
    const minimumTitleHeight = await mobilePage.locator(".post-archive-entry h3 a").evaluateAll((links) =>
      Math.min(...links.map((link) => link.getBoundingClientRect().height)),
    );
    assert(minimumTitleHeight >= 44, `${viewport.width}px title touch target is ${minimumTitleHeight}px`);
    const reducedAnimation = await mobilePage.locator(".post-archive-year").first().evaluate((element) => getComputedStyle(element).animationName);
    assert(reducedAnimation === "none", `reduced-motion still animates year groups: ${reducedAnimation}`);
    if (viewport.width === 390) {
      await mobilePage.screenshot({ path: path.join(evidenceDir, "site-p36-post-archive-mobile-390x844.png"), fullPage: true });
    }
    assert(mobileErrors.length === 0, `${viewport.width}px runtime errors: ${mobileErrors.join(" | ")}`);
    await mobile.close();
  }

  const noJavaScript = await browser.newContext({
    viewport: { width: 390, height: 844 },
    locale: "zh-CN",
    javaScriptEnabled: false,
  });
  const noJsPage = await noJavaScript.newPage();
  const noJsResponse = await noJsPage.goto(`${siteBase}/post/`, { waitUntil: "load" });
  assert(noJsResponse?.status() === 200, "no-JS /post/ did not return 200");
  await assertArchiveStructure(noJsPage);
  await assertNoOverflow(noJsPage, "no-JS post archive");
  await noJavaScript.close();

  console.log("P36 post archive QA passed: semantic year groups, descending dates, same-origin requests, target animation, reduced motion, no-JS, 1280/390/320 layouts.");
} finally {
  await browser.close();
}
