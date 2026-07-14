import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P22_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P22_EVIDENCE_DIR || "docs/process/evidence");
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

const assertTimeline = async (page) => {
    const currentYear = new Date().getFullYear();
    const years = Array.from({ length: currentYear - 2024 + 1 }, (_, index) => String(currentYear - index));
    const renderedYears = await page.locator("[data-achievement-year]").evaluateAll((elements) =>
        elements.map((element) => element.id.replace("achievement-year-", "")),
    );
    assert(JSON.stringify(renderedYears) === JSON.stringify(years), `year tracks are incorrect: ${JSON.stringify(renderedYears)}`);

    const navigation = await page.locator(".archives-year-nav__link").evaluateAll((links) =>
        links.map((link) => ({ href: link.getAttribute("href"), text: link.textContent.trim() })),
    );
    assert(navigation.length === years.length, "year navigation does not cover every track");
    assert(navigation.every(({ href, text }) => href?.endsWith(`#achievement-year-${text}`)), "year navigation anchors are invalid");
    assert((await page.locator("main h1").count()) === 1, "archive must have one H1");
    assert((await page.locator("main h1").textContent())?.trim() === "成果时间线", "archive title is incorrect");
    assert((await page.locator(".article-list, .article-entry, [data-article]").count()) === 0, "article content leaked into achievement timeline");
    assert((await page.locator("[data-achievement-timeline]").count()) === 1, "achievement timeline is missing");

    if ((await page.locator(".achievement-item").count()) === 0) {
        assert((await page.getByText("暂无已发布成果。", { exact: true }).count()) === 1, "empty published-achievement state is missing");
    }
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
    const desktop = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN", reducedMotion: "reduce" });
    const desktopPage = await desktop.newPage();
    const desktopErrors = collectErrors(desktopPage);
    const response = await desktopPage.goto(`${siteBase}/archives/`, { waitUntil: "networkidle" });
    assert(response?.status() === 200, "archive route did not return 200");
    await assertTimeline(desktopPage);
    await assertNoOverflow(desktopPage, "desktop archive");
    await desktopPage.locator("[data-achievement-year]").first().scrollIntoViewIfNeeded();
    await desktopPage.waitForFunction(() => document.querySelectorAll(".archives-year-nav__link.active").length === 1, null, { timeout: 5000 });
    const reducedMotionState = await desktopPage.locator("html").evaluate((element) => ({
        scrollBehavior: getComputedStyle(element).scrollBehavior,
        animatedElements: [...document.querySelectorAll(".archives-year-nav, .archives-timeline")].filter((node) =>
            getComputedStyle(node).animationName !== "none" || getComputedStyle(node).transitionDuration !== "0s",
        ).length,
    }));
    assert(reducedMotionState.scrollBehavior === "auto", `reduced motion keeps smooth scrolling: ${JSON.stringify(reducedMotionState)}`);
    assert(reducedMotionState.animatedElements === 0, `reduced motion keeps animation or transition: ${JSON.stringify(reducedMotionState)}`);
    const avatar = desktopPage.locator(".site-avatar img");
    assert((await avatar.count()) === 1, "sidebar avatar is missing");
    assert(await avatar.evaluate((image) => image.complete && image.naturalWidth > 0), "Hugo avatar resource did not load");
    await desktopPage.screenshot({ path: path.join(evidenceDir, "archive-p22-desktop-1280x800.png"), fullPage: true });
    assert(desktopErrors.length === 0, `desktop browser errors: ${desktopErrors.join(" | ")}`);
    await desktop.close();

    const noScript = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN", javaScriptEnabled: false });
    const noScriptPage = await noScript.newPage();
    const noScriptResponse = await noScriptPage.goto(`${siteBase}/archives/`, { waitUntil: "networkidle" });
    assert(noScriptResponse?.status() === 200, "archive route failed without JavaScript");
    await assertTimeline(noScriptPage);
    await assertNoOverflow(noScriptPage, "no-script archive");
    assert((await noScriptPage.locator(".archives-year-nav__link.active").count()) === 0, "active state was baked into no-script HTML");
    await noScript.close();

    for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 568 }]) {
        const mobile = await browser.newContext({ viewport, locale: "zh-CN", reducedMotion: "reduce" });
        const page = await mobile.newPage();
        const errors = collectErrors(page);
        await page.goto(`${siteBase}/archives/`, { waitUntil: "networkidle" });
        await assertTimeline(page);
        await assertNoOverflow(page, `${viewport.width}px archive`);
        const columnCounts = await page.locator(".archives-year").evaluateAll((elements) =>
            elements.map((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length),
        );
        assert(columnCounts.every((count) => count === 1), `${viewport.width}px year tracks are not single-column: ${columnCounts}`);
        await page.screenshot({ path: path.join(evidenceDir, `archive-p22-mobile-${viewport.width}x${viewport.height}.png`), fullPage: true });
        assert(errors.length === 0, `${viewport.width}px browser errors: ${errors.join(" | ")}`);
        await mobile.close();
    }

    console.log("P22 achievement timeline QA passed: empty-data fallback, 2024-now tracks, anchors, article exclusion, avatar, reduced motion, and 1280/390/320 responsive layouts.");
} finally {
    await browser.close();
}
