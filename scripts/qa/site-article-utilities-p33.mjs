import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P33_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P33_EVIDENCE_DIR || "docs/process/evidence");
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

const installStablePageState = async (context, captureScroll = false) => {
  await context.addInitScript(({ shouldCaptureScroll }) => {
    sessionStorage.setItem("zoking-blog:splash-seen", "1");
    localStorage.setItem("StackColorScheme", "light");
    window.__p33ClipboardWrites = [];
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: async (value) => {
          window.__p33ClipboardWrites.push(value);
        },
      },
    });
    if (shouldCaptureScroll) {
      window.__p33ScrollCalls = [];
      window.scrollTo = (options) => window.__p33ScrollCalls.push(options);
    }
  }, { shouldCaptureScroll: captureScroll });
  await context.route("**/api/v1/public/posts/**/comments", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ data: [] }),
  }));
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", reducedMotion: "no-preference" });
  await installStablePageState(context);
  const page = await context.newPage();
  const errors = collectErrors(page);
  await page.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });

  const headingLink = page.locator(".article-content h2 .header-anchor").first();
  assert((await headingLink.getAttribute("aria-label"))?.startsWith("定位并复制"), "heading link does not describe both navigation and copy behavior");
  await headingLink.focus();
  await page.waitForTimeout(180);
  const focusedStyle = await headingLink.evaluate((element) => {
    const styles = getComputedStyle(element);
    return { opacity: Number.parseFloat(styles.opacity), outlineStyle: styles.outlineStyle };
  });
  assert(focusedStyle.opacity >= 0.9 && focusedStyle.outlineStyle !== "none", `focused heading link is not discoverable: ${JSON.stringify(focusedStyle)}`);

  const headingTitle = await headingLink.getAttribute("data-heading-title");
  await headingLink.click();
  await page.waitForFunction(() => window.__p33ClipboardWrites.length === 1);
  const headingCopy = await page.evaluate(() => window.__p33ClipboardWrites[0]);
  const copiedURL = new URL(headingCopy);
  assert(copiedURL.pathname === articlePath && decodeURIComponent(copiedURL.hash.slice(1)) === headingTitle, `heading copy is not a full chapter URL: ${headingCopy}`);
  assert(await headingLink.evaluate((element) => element.classList.contains("is-copied")), "heading link did not expose visual copied state");
  assert((await page.locator("[data-heading-link-status]").textContent())?.includes("章节链接已复制"), "heading copy was not announced");

  const toolbars = page.locator(".article-content .code-block-toolbar");
  assert((await toolbars.count()) === 2, `expected two code toolbars, found ${await toolbars.count()}`);
  assert((await toolbars.locator(".code-block-language").allTextContents()).every((value) => value.trim() === "Go"), "Go code blocks do not expose their language");
  const firstCode = page.locator(".article-content code[data-lang='go']").first();
  const expectedCode = await firstCode.textContent();
  assert(expectedCode?.trimStart().startsWith("package main"), "code fixture no longer starts with package main");
  const firstCopyButton = toolbars.first().locator(".copyCodeButton");
  assert((await firstCopyButton.getAttribute("aria-label")) === "复制 Go 代码", "code copy button lacks a descriptive accessible name");
  await firstCopyButton.click();
  await page.waitForFunction(() => window.__p33ClipboardWrites.length === 2);
  await page.waitForFunction(() => document.querySelector(".code-block-toolbar .copyCodeButton")?.textContent?.includes("已复制"));
  const copiedCode = await page.evaluate(() => window.__p33ClipboardWrites[1]);
  assert(copiedCode === expectedCode, "code copy did not use the code element text exactly");
  assert(!copiedCode.trimStart().startsWith("1\n"), "code copy included rendered line numbers");
  assert((await firstCopyButton.textContent())?.includes("已复制"), "code copy button did not show success feedback");

  await page.evaluate(() => {
    navigator.clipboard.writeText = async () => {
      throw new DOMException("denied", "NotAllowedError");
    };
  });
  const failedCopyButton = toolbars.nth(1).locator(".copyCodeButton");
  await failedCopyButton.click();
  assert((await failedCopyButton.textContent())?.trim() === "复制失败", "clipboard failure did not produce localized button feedback");
  assert((await toolbars.nth(1).locator("[role='status']").textContent())?.includes("手动选择代码"), "clipboard failure did not announce a recovery path");

  await page.evaluate(() => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    document.execCommand = (command) => {
      window.__p33FallbackCommand = command;
      window.__p33FallbackValue = document.activeElement?.value || "";
      return true;
    };
  });
  await failedCopyButton.click();
  await page.waitForFunction(() => window.__p33FallbackCommand === "copy");
  const fallbackCopy = await page.evaluate(() => ({ command: window.__p33FallbackCommand, value: window.__p33FallbackValue }));
  assert(fallbackCopy.command === "copy" && fallbackCopy.value === await page.locator(".article-content code[data-lang='go']").nth(1).textContent(), "legacy clipboard fallback did not copy the code text");
  assert((await failedCopyButton.textContent())?.includes("已复制"), "legacy clipboard fallback did not return to the success state");

  await toolbars.first().scrollIntoViewIfNeeded();
  await page.screenshot({ path: path.join(evidenceDir, "site-p33-article-tools-desktop-1280x900.png"), fullPage: false });
  assert(errors.length === 0, `desktop runtime errors: ${errors.join(" | ")}`);
  await context.close();

  const reducedContext = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installStablePageState(reducedContext, true);
  const reducedPage = await reducedContext.newPage();
  await reducedPage.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });
  await reducedPage.locator(".article-content h2 .header-anchor").first().click();
  const reducedScroll = await reducedPage.evaluate(() => window.__p33ScrollCalls.at(-1));
  assert(reducedScroll?.behavior === "auto", `reduced-motion anchor used ${JSON.stringify(reducedScroll)}`);
  await reducedContext.close();

  const noScriptContext = await browser.newContext({ viewport: { width: 1280, height: 900 }, javaScriptEnabled: false });
  const noScriptPage = await noScriptContext.newPage();
  await noScriptPage.goto(`${siteBase}${articlePath}`, { waitUntil: "domcontentloaded" });
  const fallbackHref = await noScriptPage.locator(".article-content h2 .header-anchor").first().getAttribute("href");
  assert(fallbackHref?.startsWith("#"), "heading link lost its no-JavaScript hash fallback");
  await noScriptContext.close();

  const mobileContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", reducedMotion: "reduce", hasTouch: true, isMobile: true });
  await installStablePageState(mobileContext);
  const mobilePage = await mobileContext.newPage();
  const mobileErrors = collectErrors(mobilePage);
  await mobilePage.goto(`${siteBase}${articlePath}`, { waitUntil: "networkidle" });
  const mobileHeadingLink = mobilePage.locator(".article-content h2 .header-anchor").first();
  const mobileTarget = await mobileHeadingLink.boundingBox();
  const mobileOpacity = await mobileHeadingLink.evaluate((element) => Number.parseFloat(getComputedStyle(element).opacity));
  assert(mobileTarget && mobileTarget.width >= 44 && mobileTarget.height >= 44, `mobile heading target is too small: ${JSON.stringify(mobileTarget)}`);
  assert(mobileOpacity >= 0.6, `mobile heading link is not persistently visible: ${mobileOpacity}`);
  const mobileToolbar = mobilePage.locator(".article-content .code-block-toolbar").first();
  await mobileToolbar.scrollIntoViewIfNeeded();
  const mobileDimensions = await mobilePage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(mobileDimensions.scrollWidth <= mobileDimensions.viewportWidth + 1, `390px article overflows: ${JSON.stringify(mobileDimensions)}`);
  await mobilePage.screenshot({ path: path.join(evidenceDir, "site-p33-article-tools-mobile-390x844.png"), fullPage: false });
  await mobilePage.setViewportSize({ width: 320, height: 568 });
  await mobilePage.reload({ waitUntil: "networkidle" });
  const narrowDimensions = await mobilePage.locator("html").evaluate((element) => ({ scrollWidth: element.scrollWidth, viewportWidth: innerWidth }));
  assert(narrowDimensions.scrollWidth <= narrowDimensions.viewportWidth + 1, `320px article overflows: ${JSON.stringify(narrowDimensions)}`);
  assert(mobileErrors.length === 0, `mobile runtime errors: ${mobileErrors.join(" | ")}`);
  await mobileContext.close();

  process.stdout.write("[site-article-utilities-p33] PASS chapter copy, code toolbar, failure recovery, no-JS, reduced motion, 1280/390/320\n");
} finally {
  await browser.close();
}
