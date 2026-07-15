import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P32_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P32_EVIDENCE_DIR || "docs/process/evidence");
const articlePath = "/p/gin-request-lifecycle/";
const preferencesKey = "zoking:reading-preferences:v1";

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

const articleTypography = (page) => page.locator(".article-content").evaluate((element) => {
  const styles = getComputedStyle(element);
  return { fontSize: Number.parseFloat(styles.fontSize), lineHeight: Number.parseFloat(styles.lineHeight) };
});

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", reducedMotion: "reduce" });
  await context.addInitScript(() => {
    sessionStorage.setItem("zoking-blog:splash-seen", "1");
    localStorage.setItem("StackColorScheme", "light");
    document.addEventListener("DOMContentLoaded", () => {
      window.__p32ReadingDatasetAtDOMContent = {
        font: document.documentElement.dataset.readingFont || "",
        spacing: document.documentElement.dataset.readingSpacing || "",
        links: document.documentElement.dataset.readingLinks || "",
      };
    }, { once: true });
  });
  await context.route("**/api/v1/public/posts/**/comments", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ data: [] }),
  }));
  const page = await context.newPage();
  const errors = collectErrors(page);

  await page.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });
  assert((await page.locator('link[href*="reading-settings"]').count()) === 1, "article page did not load its scoped reading settings stylesheet");
  const settingsButton = page.getByRole("button", { name: "打开阅读设置" });
  assert(await settingsButton.isVisible(), "article topbar reading settings button is missing");
  const initial = await articleTypography(page);

  await settingsButton.click();
  const dialog = page.getByRole("dialog", { name: "阅读设置" });
  assert(await dialog.isVisible(), "reading settings dialog did not open");
  assert((await dialog.locator("[data-reading-font]").count()) === 3, "font segmented control must contain three options");
  assert((await dialog.locator('[data-reading-font="default"]').getAttribute("aria-pressed")) === "true", "default font option is not selected initially");

  await dialog.locator('[data-reading-font="large"]').click();
  const large = await articleTypography(page);
  assert(large.fontSize > initial.fontSize, `large font did not increase size: ${JSON.stringify({ initial, large })}`);
  await dialog.locator('[data-reading-font="xlarge"]').click();
  const xlarge = await articleTypography(page);
  assert(xlarge.fontSize > large.fontSize, `xlarge font did not increase size: ${JSON.stringify({ large, xlarge })}`);

  await dialog.locator("[data-reading-spacing]").check();
  const relaxed = await articleTypography(page);
  assert(relaxed.lineHeight > initial.lineHeight, `relaxed spacing did not increase line height: ${JSON.stringify({ initial, relaxed })}`);
  await dialog.locator("[data-reading-links]").check();
  const articleLink = page.locator('.article-content a[href^="https://"]').first();
  assert((await articleLink.evaluate((element) => getComputedStyle(element).textDecorationLine)).includes("underline"), "article links are not underlined");

  const stored = await page.evaluate((key) => JSON.parse(localStorage.getItem(key) || "null"), preferencesKey);
  assert(stored?.font === "xlarge" && stored.relaxedSpacing === true && stored.underlineLinks === true, `stored preferences are invalid: ${JSON.stringify(stored)}`);
  await page.screenshot({ path: path.join(evidenceDir, "site-p32-reading-settings-desktop-1280x900.png"), fullPage: false });

  await page.getByRole("button", { name: "关闭阅读设置" }).click();
  assert(await dialog.isHidden(), "dialog did not close");
  assert(await settingsButton.evaluate((element) => element === document.activeElement), "focus did not return to reading settings button");
  await page.reload({ waitUntil: "networkidle" });
  const restoredAtDOMContent = await page.evaluate(() => window.__p32ReadingDatasetAtDOMContent);
  assert(restoredAtDOMContent?.font === "xlarge" && restoredAtDOMContent.spacing === "relaxed" && restoredAtDOMContent.links === "underlined", `preferences were not restored before DOMContentLoaded: ${JSON.stringify(restoredAtDOMContent)}`);

  await settingsButton.click();
  assert((await dialog.locator('[data-reading-font="xlarge"]').getAttribute("aria-pressed")) === "true", "xlarge selection did not persist after reload");
  assert(await dialog.locator("[data-reading-spacing]").isChecked() && await dialog.locator("[data-reading-links]").isChecked(), "binary preferences did not persist after reload");
  await dialog.getByRole("button", { name: "恢复默认" }).click();
  assert(await page.evaluate((key) => localStorage.getItem(key) === null, preferencesKey), "reset did not remove stored preferences");
  assert(await page.evaluate(() => !document.documentElement.dataset.readingFont && !document.documentElement.dataset.readingSpacing && !document.documentElement.dataset.readingLinks), "reset left reading data attributes behind");
  await page.keyboard.press("Escape");
  assert(await dialog.isHidden(), "Escape did not close reading settings dialog");

  await page.goto(`${siteBase}/now/`, { waitUntil: "networkidle" });
  assert((await page.locator("[data-reading-settings-open], [data-reading-settings-dialog]").count()) === 0, "reading settings leaked onto a non-article page");
  assert((await page.locator('link[href*="reading-settings"]').count()) === 0, "non-article page loaded the reading settings stylesheet");
  assert(errors.length === 0, `desktop runtime errors: ${errors.join(" | ")}`);
  await context.close();

  const mobileContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", reducedMotion: "reduce" });
  await mobileContext.addInitScript((key) => {
    sessionStorage.setItem("zoking-blog:splash-seen", "1");
    localStorage.setItem(key, JSON.stringify({ font: "xlarge", relaxedSpacing: true, underlineLinks: true }));
  }, preferencesKey);
  await mobileContext.route("**/api/v1/public/posts/**/comments", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ data: [] }),
  }));
  const mobilePage = await mobileContext.newPage();
  const mobileErrors = collectErrors(mobilePage);
  await mobilePage.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });
  await mobilePage.getByRole("button", { name: "打开阅读设置" }).click();
  const mobileDialog = mobilePage.getByRole("dialog", { name: "阅读设置" });
  const bounds = await mobileDialog.boundingBox();
  assert(bounds && bounds.x >= 0 && bounds.x + bounds.width <= 390, `mobile dialog is outside viewport: ${JSON.stringify(bounds)}`);
  const dimensions = await mobilePage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(dimensions.scrollWidth <= dimensions.viewportWidth + 1, `mobile article overflows with xlarge font: ${JSON.stringify(dimensions)}`);
  assert(mobileErrors.length === 0, `mobile runtime errors: ${mobileErrors.join(" | ")}`);
  await mobilePage.screenshot({ path: path.join(evidenceDir, "site-p32-reading-settings-mobile-390x844.png"), fullPage: false });
  await mobilePage.keyboard.press("Escape");
  await mobilePage.setViewportSize({ width: 320, height: 568 });
  await mobilePage.reload({ waitUntil: "networkidle" });
  await mobilePage.getByRole("button", { name: "打开阅读设置" }).click();
  const narrowBounds = await mobileDialog.boundingBox();
  assert(narrowBounds && narrowBounds.x >= 0 && narrowBounds.x + narrowBounds.width <= 320, `320px dialog is outside viewport: ${JSON.stringify(narrowBounds)}`);
  const narrowDimensions = await mobilePage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(narrowDimensions.scrollWidth <= narrowDimensions.viewportWidth + 1, `320px article overflows with xlarge font: ${JSON.stringify(narrowDimensions)}`);
  await mobileContext.close();

  process.stdout.write("[site-reading-settings-p32] PASS article-only dialog, typography controls, persistence, reset, focus, 1280/390/320\n");
} finally {
  await browser.close();
}
