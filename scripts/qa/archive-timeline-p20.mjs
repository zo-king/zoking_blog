import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P20_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P20_EVIDENCE_DIR || "docs/process/evidence");

const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

const collectErrors = (page) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") {
      const location = message.location();
      errors.push(`console: ${message.text()}${location.url ? ` (${location.url})` : ""}`);
    }
  });
  return errors;
};

const assertNoOverflow = async (page, label) => {
  const dimensions = await page.locator("html").evaluate((element) => ({
    scrollWidth: element.scrollWidth,
    viewportWidth: window.innerWidth,
  }));
  assert(
    dimensions.scrollWidth <= dimensions.viewportWidth + 1,
    `${label} overflows horizontally: ${JSON.stringify(dimensions)}`,
  );
};

const assertArchiveStructure = async (page) => {
  assert((await page.locator("main h1").count()) === 1, "archive must have exactly one main H1");
  assert((await page.locator(".archives-timeline").count()) === 1, "archive timeline is missing");
  assert((await page.locator(".archives-year").count()) === 1, "expected one archive year group");
  assert((await page.locator(".archives-entry").count()) === 3, "expected three timeline entries");

  const headings = await page.locator("main h1, main h2, main h3").evaluateAll((elements) =>
    elements.map((element) => ({ level: Number(element.tagName.slice(1)), text: element.textContent.trim() })),
  );
  assert(headings[0]?.level === 1 && headings[0]?.text === "归档", `invalid archive H1: ${JSON.stringify(headings[0])}`);
  for (let index = 1; index < headings.length; index += 1) {
    assert(headings[index].level <= headings[index - 1].level + 1, `heading level skipped: ${JSON.stringify(headings)}`);
  }

  const titleHeadings = await page.locator(".archives-entry__title").evaluateAll((elements) =>
    elements.map((element) => element.tagName),
  );
  assert(titleHeadings.every((tag) => tag === "H3"), `timeline titles are not H3: ${titleHeadings}`);

  const dates = await page.locator(".archives-entry__date").evaluateAll((elements) =>
    elements.map((element) => ({ text: element.textContent.trim(), datetime: element.getAttribute("datetime") })),
  );
  assert(
    JSON.stringify(dates.map(({ text }) => text)) === JSON.stringify(["07月10日", "07月08日", "07月05日"]),
    `timeline is not reverse chronological: ${JSON.stringify(dates)}`,
  );
  assert(dates.every(({ datetime }) => /^2026-07-\d{2}T/.test(datetime || "")), `invalid time datetime: ${JSON.stringify(dates)}`);
  assert((await page.getByText("3 篇", { exact: true }).count()) === 1, "year article count is incorrect");

  const listSemantics = await page.locator(".archives-year__entries").evaluate((list) => ({
    tag: list.tagName,
    children: [...list.children].map((child) => child.tagName),
    articles: list.querySelectorAll(":scope > li > article").length,
  }));
  assert(listSemantics.tag === "OL", "timeline must use an ordered list");
  assert(listSemantics.children.every((tag) => tag === "LI"), "timeline must contain direct list items");
  assert(listSemantics.articles === 3, "each timeline item must contain an article");

  const images = page.locator(".archives-entry__image img");
  assert((await images.count()) === 2, "expected two timeline cover images");
  const imageState = await images.evaluateAll((elements) => elements.map((image) => ({
    complete: image.complete,
    naturalWidth: image.naturalWidth,
    ariaHidden: image.getAttribute("aria-hidden"),
  })));
  assert(imageState.every((image) => image.complete && image.naturalWidth > 0), `timeline images failed: ${JSON.stringify(imageState)}`);
  assert(imageState.every((image) => image.ariaHidden === "true"), `decorative images are exposed: ${JSON.stringify(imageState)}`);
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const desktop = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    locale: "zh-CN",
    colorScheme: "light",
    reducedMotion: "reduce",
  });
  const page = await desktop.newPage();
  const desktopErrors = collectErrors(page);

  await page.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  const menuText = await page.locator("#main-menu").innerText();
  assert(!menuText.includes("搜索"), `search still appears in primary navigation: ${menuText}`);
  assert((await page.locator(".widget.search-form, form.widget.search-form").count()) === 0, "homepage search widget still appears");

  const archiveResponse = await page.goto(`${siteBase}/archives/`, { waitUntil: "networkidle" });
  assert(archiveResponse?.status() === 200, "archive route did not return 200");
  await assertArchiveStructure(page);
  await assertNoOverflow(page, "desktop archive");
  await page.screenshot({ path: path.join(evidenceDir, "archive-p20-desktop-1280x800.png"), fullPage: true });

  const searchIndexResponse = await desktop.request.get(`${siteBase}/search/index.json`);
  assert(searchIndexResponse.status() === 200, "/search/index.json compatibility route did not return 200");
  assert((await searchIndexResponse.headers())["content-type"]?.includes("application/json"), "search index is not JSON");
  for (const routePath of ["/search/", "/categories/", "/tags/"]) {
    const response = await page.goto(`${siteBase}${routePath}`, { waitUntil: "networkidle" });
    assert(response?.status() === 200, `${routePath} compatibility route did not return 200`);
  }
  assert(desktopErrors.length === 0, `desktop browser errors: ${desktopErrors.join(" | ")}`);
  await desktop.close();

  const dark = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN", colorScheme: "dark" });
  await dark.addInitScript(() => localStorage.setItem("StackColorScheme", "dark"));
  const darkPage = await dark.newPage();
  const darkErrors = collectErrors(darkPage);
  await darkPage.goto(`${siteBase}/archives/`, { waitUntil: "networkidle" });
  assert((await darkPage.locator("html").getAttribute("data-scheme")) === "dark", "dark scheme was not applied");
  const darkColors = await darkPage.locator(".archives-entry").first().evaluate((entry) => ({
    text: getComputedStyle(entry.querySelector(".archives-entry__summary")).color,
    line: getComputedStyle(entry.parentElement, "::before").backgroundColor,
  }));
  assert(darkColors.text !== darkColors.line, `dark text and timeline line colors collapsed: ${JSON.stringify(darkColors)}`);
  await darkPage.screenshot({ path: path.join(evidenceDir, "archive-p20-dark-1280x800.png"), fullPage: false });
  assert(darkErrors.length === 0, `dark browser errors: ${darkErrors.join(" | ")}`);
  await dark.close();

  for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 568 }]) {
    const mobile = await browser.newContext({ viewport, locale: "zh-CN", reducedMotion: "reduce" });
    const mobilePage = await mobile.newPage();
    const mobileErrors = collectErrors(mobilePage);
    await mobilePage.goto(`${siteBase}/archives/`, { waitUntil: "networkidle" });
    await assertArchiveStructure(mobilePage);
    await assertNoOverflow(mobilePage, `${viewport.width}px archive`);
    const minimumLinkHeight = await mobilePage.locator(".archives-entry__link").evaluateAll((links) =>
      Math.min(...links.map((link) => link.getBoundingClientRect().height)),
    );
    assert(minimumLinkHeight >= 44, `${viewport.width}px timeline touch target is ${minimumLinkHeight}px`);
    await mobilePage.screenshot({
      path: path.join(evidenceDir, `archive-p20-mobile-${viewport.width}x${viewport.height}.png`),
      fullPage: true,
    });
    assert(mobileErrors.length === 0, `${viewport.width}px browser errors: ${mobileErrors.join(" | ")}`);
    await mobile.close();
  }

  console.log("P20 archive timeline QA passed: navigation, compatibility routes, semantics, ordering, images, dark mode, 1280/390/320 responsive layouts.");
} finally {
  await browser.close();
}
