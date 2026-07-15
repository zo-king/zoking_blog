import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P31_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P31_EVIDENCE_DIR || "docs/process/evidence");

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

const installNavigationProbe = async (context) => {
  await context.addInitScript(() => {
    sessionStorage.setItem("zoking-blog:splash-seen", "1");
    localStorage.setItem("StackColorScheme", "light");
    addEventListener("pageswap", (event) => {
      sessionStorage.setItem("p31:pageswap", String(Boolean(event.viewTransition)));
    });
    addEventListener("pagereveal", (event) => {
      sessionStorage.setItem("p31:pagereveal", String(Boolean(event.viewTransition)));
    });
  });
};

const hasViewTransitionRule = (page) => page.evaluate(() => Array.from(document.styleSheets).some((sheet) => {
  try {
    return Array.from(sheet.cssRules).some((rule) => rule.cssText.includes("@view-transition"));
  } catch {
    return false;
  }
}));

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", reducedMotion: "no-preference" });
  await installNavigationProbe(context);
  const page = await context.newPage();
  const errors = collectErrors(page);

  await page.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  assert(await hasViewTransitionRule(page), "cross-document view-transition opt-in is missing");
  assert((await page.locator('#main-menu a[href="/now/"]').count()) === 1, "main navigation must contain one 近况 entry");

  await page.locator('#main-menu a[href="/now/"]').click();
  await page.waitForURL("**/now/");
  await page.waitForTimeout(260);
  const transitionState = await page.evaluate(() => ({
    supported: "PageRevealEvent" in window,
    pageSwap: sessionStorage.getItem("p31:pageswap"),
    pageReveal: sessionStorage.getItem("p31:pagereveal"),
    animationName: getComputedStyle(document.documentElement, "::view-transition-new(root)").animationName,
  }));
  if (transitionState.supported) {
    assert(transitionState.pageSwap === "true" && transitionState.pageReveal === "true", `same-origin navigation did not use a view transition: ${JSON.stringify(transitionState)}`);
  }
  assert(transitionState.animationName === "site-page-in", `unexpected page transition animation: ${transitionState.animationName}`);
  assert((await page.locator("main h1").count()) === 1 && (await page.locator("main h1").textContent())?.trim() === "近况", "近况 page must have one matching H1");
  assert((await page.locator(".article-content h2").allTextContents()).map((value) => value.trim()).join("|") === "正在开发|正在学习|最近关注", "近况 page sections are incomplete");
  assert((await page.locator('link[rel="canonical"]').getAttribute("href"))?.endsWith("/now/"), "近况 canonical URL is incorrect");
  await page.screenshot({ path: path.join(evidenceDir, "site-p31-now-desktop-1280x900.png"), fullPage: false });

  await page.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  await page.keyboard.press("Control+k");
  await page.locator("[data-command-input]").fill("近况");
  assert((await page.locator("[data-command-item]:visible").count()) === 1, "command palette did not narrow to one 近况 result");
  await page.keyboard.press("Enter");
  await page.waitForURL("**/now/");
  await page.goBack({ waitUntil: "networkidle" });
  assert(new URL(page.url()).pathname === "/", `browser back returned to ${page.url()}`);
  await page.goForward({ waitUntil: "networkidle" });
  assert(new URL(page.url()).pathname === "/now/", `browser forward returned to ${page.url()}`);
  assert(errors.length === 0, `desktop runtime errors: ${errors.join(" | ")}`);
  await context.close();

  const reducedContext = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installNavigationProbe(reducedContext);
  const reducedPage = await reducedContext.newPage();
  await reducedPage.goto(`${siteBase}/now/`, { waitUntil: "networkidle" });
  const reducedAnimation = await reducedPage.evaluate(() => getComputedStyle(document.documentElement, "::view-transition-new(root)").animationName);
  assert(reducedAnimation === "none", `reduced motion still enables page animation: ${reducedAnimation}`);
  await reducedContext.close();

  const mobileContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installNavigationProbe(mobileContext);
  const mobilePage = await mobileContext.newPage();
  const mobileErrors = collectErrors(mobilePage);
  await mobilePage.goto(`${siteBase}/`, { waitUntil: "networkidle" });
  await mobilePage.getByRole("button", { name: /菜单/ }).click();
  const mobileNow = mobilePage.locator('#main-menu a[href="/now/"]');
  assert(await mobileNow.isVisible(), "mobile menu does not expose 近况");
  await mobileNow.click();
  await mobilePage.waitForURL("**/now/");
  const dimensions = await mobilePage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(dimensions.scrollWidth <= dimensions.viewportWidth + 1, `mobile 近况 page overflows: ${JSON.stringify(dimensions)}`);
  assert(mobileErrors.length === 0, `mobile runtime errors: ${mobileErrors.join(" | ")}`);
  await mobilePage.screenshot({ path: path.join(evidenceDir, "site-p31-now-mobile-390x844.png"), fullPage: false });
  await mobileContext.close();

  process.stdout.write("[site-navigation-now-p31] PASS view transitions, reduced motion, 近况 menu/command navigation, back-forward, 1280/390\n");
} finally {
  await browser.close();
}
