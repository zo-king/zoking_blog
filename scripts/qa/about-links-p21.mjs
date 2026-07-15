import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P21_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P21_EVIDENCE_DIR || "docs/process/evidence");
const faviconFixture = Buffer.from("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"32\" height=\"32\"><rect width=\"32\" height=\"32\" fill=\"#4f6f52\"/></svg>");

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

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  for (const viewport of [{ width: 1280, height: 800 }, { width: 390, height: 844 }]) {
    const context = await browser.newContext({ viewport, locale: "zh-CN", reducedMotion: "reduce" });
    await context.route(/p21-favicon-ok\.svg$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: faviconFixture,
      });
    });
    await context.route(/icons\.duckduckgo\.com\/ip3\/.*\.ico$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: faviconFixture,
      });
    });
    await context.route(/\/favicon\.ico(?:\?.*)?$/, async (route) => {
      if (route.request().url().includes("gohugo.io")) {
        await route.fulfill({ status: 404, body: "" });
        return;
      }
      await route.fulfill({ status: 200, contentType: "image/svg+xml", body: faviconFixture });
    });

    const page = await context.newPage();
    const errors = collectErrors(page);
    const articleResponse = await context.request.get(`${siteBase}/p/city-walk/`);
    assert(articleResponse.status() === 200, "article regression route did not return 200");
    assert((await articleResponse.text()).includes('id="related-content-title"'), "article related content was removed unexpectedly");
    const aboutResponse = await page.goto(`${siteBase}/about/`, { waitUntil: "networkidle" });
    assert(aboutResponse?.status() === 200, "about route did not return 200");
    assert((await page.locator("main h1").count()) === 1, "about page must have one H1");
    assert((await page.locator(".related-content, #related-content-title").count()) === 0, "about page still contains related content");
    assert((await page.locator(".links-directory").count()) === 0, "about page unexpectedly contains link directory");
    await assertNoOverflow(page, `${viewport.width}px about`);
    await page.screenshot({ path: path.join(evidenceDir, `about-p21-${viewport.width}x${viewport.height}.png`), fullPage: true });

    const linksResponse = await page.goto(`${siteBase}/links/`, { waitUntil: "networkidle" });
    assert(linksResponse?.status() === 200, "links route did not return 200");
    await page.waitForTimeout(250);
    assert((await page.locator("main h1").count()) === 1, "links page must have one H1");
    assert((await page.locator(".related-content, #related-content-title").count()) === 0, "links page still contains related content");
    const cards = page.locator(".links-card");
    assert((await cards.count()) === 9, `expected 9 link cards, got ${await cards.count()}`);
    assert((await page.locator(".links-card__meta").count()) === 9, "link metadata is missing");
    assert((await page.locator(".links-card__arrow").count()) === 9, "external link indicators are missing");
    const avatarScript = await page.locator('script[src*="linksAvatars"]').getAttribute("src");
    assert(avatarScript, "links avatar script is missing");
    await page.waitForTimeout(250);
    const avatarState = await page.locator(".links-card__avatar").evaluateAll((elements) => elements.map((element) => ({
      fallback: element.classList.contains("links-card__avatar--fallback"),
      hasImage: Boolean(element.querySelector("img")),
    })));
    const initialErrors = [...errors];
    assert(avatarState.length === 9, `avatar containers are missing: ${JSON.stringify(avatarState)}`);
    const fixtureAvatar = page.locator(".links-card__avatar").last();
    await fixtureAvatar.evaluate((avatar) => {
      avatar.querySelector("img")?.remove();
      const image = document.createElement("img");
      image.alt = "";
      image.src = "/p21-favicon-ok.svg";
      avatar.append(image);
    });
    await page.addScriptTag({ url: avatarScript });
    await page.waitForTimeout(250);
    assert(await fixtureAvatar.evaluate((avatar) => avatar.classList.contains("links-card__avatar--loaded")), "successful favicon was not shown");
    await fixtureAvatar.locator("img").evaluate((image) => {
      image.src = "/p21-missing-favicon.ico";
    });
    await page.waitForTimeout(250);
    assert(await fixtureAvatar.evaluate((avatar) => avatar.classList.contains("links-card__avatar--fallback")), "failed favicon did not use fallback");
    await assertNoOverflow(page, `${viewport.width}px links`);
    await page.screenshot({ path: path.join(evidenceDir, `links-p21-${viewport.width}x${viewport.height}.png`), fullPage: true });
    assert(initialErrors.length === 0, `${viewport.width}px browser errors before fallback fixture: ${initialErrors.join(" | ")}`);
    await context.close();
  }

  console.log("P21 about/links QA passed: related content removed, favicon success/fallback covered, desktop/mobile layouts have no overflow.");
} finally {
  await browser.close();
}
