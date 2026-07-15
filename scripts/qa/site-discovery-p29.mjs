import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P29_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P29_EVIDENCE_DIR || "docs/process/evidence");
const faviconFixture = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="#34745f"/></svg>');

const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

const assertNoOverflow = async (page, label) => {
  const dimensions = await page.locator("html").evaluate((element) => ({
    scrollWidth: element.scrollWidth,
    viewportWidth: window.innerWidth,
  }));
  assert(dimensions.scrollWidth <= dimensions.viewportWidth + 1, `${label} overflows: ${JSON.stringify(dimensions)}`);
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN" });
  await context.addInitScript(() => {
    sessionStorage.setItem("zoking-blog:splash-seen", "1");
    localStorage.setItem("StackColorScheme", "light");
  });
  await context.route(/(?:favicon\.ico|icons\.duckduckgo\.com\/ip3\/)/, async (route) => {
    await route.fulfill({ status: 200, contentType: "image/svg+xml", body: faviconFixture });
  });
  await context.route(/https:\/\/(?:www\.)?(?:joshwcomeau\.com|cassie\.codes)\//, async (route) => {
    await route.fulfill({ status: 200, contentType: "text/html", body: "<!doctype html><title>External fixture</title>" });
  });

  const page = await context.newPage();
  const runtimeErrors = [];
  page.on("pageerror", (error) => runtimeErrors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") runtimeErrors.push(`console: ${message.text()}`);
  });

  await page.goto(`${siteBase}/links/`, { waitUntil: "networkidle" });
  assert((await page.locator("[data-link-card]").count()) === 9, "link directory must render 9 entries");
  await assertNoOverflow(page, "1280px links");

  await page.locator("[data-links-search]").fill("Josh");
  const searchMatches = page.locator("[data-link-card]:visible");
  assert((await searchMatches.count()) === 1, "link search should return one Josh entry");
  assert((await searchMatches.first().textContent())?.includes("Josh W. Comeau"), "link search returned the wrong entry");

  await page.locator("[data-links-search]").fill("");
  await page.getByRole("button", { name: "前端动效" }).click();
  const filteredTitles = await page.locator("[data-link-card]:visible .links-card__title").allTextContents();
  assert(filteredTitles.length === 2, `front-end animation filter returned ${filteredTitles.length} entries`);
  assert(filteredTitles.includes("Josh W. Comeau") && filteredTitles.includes("Cassie Evans"), `unexpected filtered entries: ${filteredTitles.join(", ")}`);

  const popupPromise = page.waitForEvent("popup");
  await page.getByRole("button", { name: "随机拜访" }).click();
  const popup = await popupPromise;
  const randomHost = new URL(popup.url()).hostname.replace(/^www\./, "");
  assert(["joshwcomeau.com", "cassie.codes"].includes(randomHost), `random visit escaped visible entries: ${popup.url()}`);
  await popup.close();

  await page.getByRole("button", { name: "全部" }).click();
  await page.screenshot({ path: path.join(evidenceDir, "site-discovery-links-p29-1280x900.png"), fullPage: false });

  await page.locator("[data-command-open]").click();
  assert(await page.locator("[data-command-palette]").isVisible(), "Ctrl+K did not open the command palette");
  assert((await page.evaluate(() => document.activeElement?.id)) === "command-palette-input", "command input did not receive focus");
  await page.keyboard.press("Shift+Tab");
  assert(await page.locator("[data-command-close]").last().evaluate((element) => element === document.activeElement), "reverse Tab did not stay inside the command palette");
  await page.keyboard.press("Tab");
  assert((await page.evaluate(() => document.activeElement?.id)) === "command-palette-input", "forward Tab did not loop to the command input");
  await page.screenshot({ path: path.join(evidenceDir, "site-command-palette-p29-1280x900.png"), fullPage: false });

  await page.locator("[data-command-input]").fill("项目");
  await page.keyboard.press("Enter");
  await page.waitForURL("**/projects/");

  await page.goto(`${siteBase}/links/`, { waitUntil: "networkidle" });
  await page.keyboard.press("Control+k");
  await page.locator("[data-command-theme]").click();
  await page.waitForTimeout(200);
  assert((await page.locator("html").getAttribute("data-scheme")) === "dark", "theme command did not switch the color scheme");
  assert(await page.locator("[data-command-palette]").isHidden(), "theme command did not close the palette");

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${siteBase}/links/`, { waitUntil: "networkidle" });
  await assertNoOverflow(page, "390px links");
  const mobileColumns = await page.locator(".links-directory__grid").evaluate((element) => getComputedStyle(element).gridTemplateColumns);
  assert(!mobileColumns.includes(" "), `mobile link directory is not one column: ${mobileColumns}`);
  await page.locator("[data-command-open]").click();
  const paletteBounds = await page.locator(".command-palette__dialog").boundingBox();
  assert(paletteBounds && paletteBounds.x >= 0 && paletteBounds.x + paletteBounds.width <= 390, `mobile palette is outside viewport: ${JSON.stringify(paletteBounds)}`);
  await page.screenshot({ path: path.join(evidenceDir, "site-command-palette-p29-390x844.png"), fullPage: false });
  await page.keyboard.press("Escape");

  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.reload({ waitUntil: "networkidle" });
  const revealAnimations = await page.evaluate(() => document.getAnimations().filter((animation) => animation.effect?.target?.matches?.("[data-reveal], .article-list > article")).length);
  assert(revealAnimations === 0, `reduced motion still has ${revealAnimations} reveal animations`);
  assert(runtimeErrors.length === 0, `browser errors: ${runtimeErrors.join(" | ")}`);

  console.log("P29 discovery QA passed: links search/filter/random, Ctrl+K navigation/theme, desktop/mobile/reduced-motion checks passed.");
} finally {
  await browser.close();
}
