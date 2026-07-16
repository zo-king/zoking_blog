import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.P37_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.P37_EVIDENCE_DIR || "docs/process/evidence");
const articleURL = `${siteBase}/p/gin-production-hardening/`;

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

const assertCallouts = async (page) => {
    const types = await page.locator(".article-content blockquote.alert").evaluateAll((elements) =>
        elements.map((element) => [...element.classList].find((name) => name.startsWith("alert-"))),
    );
    assert(JSON.stringify(types) === JSON.stringify(["alert-note", "alert-warning"]), `unexpected callout types: ${JSON.stringify(types)}`);
    assert((await page.locator(".article-content blockquote.alert .alert-title").count()) === 2, "callout titles are missing");
    assert((await page.getByText("本文的超时数值是工程起点，不是通用标准。", { exact: false }).count()) === 1, "note content is missing");
    assert((await page.getByText("不要把 recover 当作输入校验或业务错误处理。", { exact: false }).count()) === 1, "warning content is missing");
    assert((await page.locator(".article-content blockquote.alert .alert-icon").count()) === 2, "callout icons are missing");
    assert((await page.locator(".article-content").innerText()).includes("[!NOTE]") === false, "Markdown alert marker leaked into output");
};

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
    const desktop = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", reducedMotion: "reduce" });
    const desktopPage = await desktop.newPage();
    const desktopResponse = await desktopPage.goto(articleURL, { waitUntil: "networkidle" });
    assert(desktopResponse?.status() === 200, "article route did not return 200");
    await assertCallouts(desktopPage);
    await assertNoOverflow(desktopPage, "desktop article");
    const desktopStyle = await desktopPage.locator(".article-content blockquote.alert").first().evaluate((element) => ({
        borderRadius: getComputedStyle(element).borderRadius,
        boxShadow: getComputedStyle(element).boxShadow,
        marginBlock: getComputedStyle(element).marginBlock,
    }));
    assert(desktopStyle.borderRadius !== "0px", `callout radius missing: ${JSON.stringify(desktopStyle)}`);
    assert(desktopStyle.boxShadow !== "none", `callout shadow missing: ${JSON.stringify(desktopStyle)}`);
    await desktopPage.screenshot({ path: path.join(evidenceDir, "site-p37-article-callouts-desktop-1280x900.png"), fullPage: false });
    await desktop.close();

    const dark = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "zh-CN", colorScheme: "dark" });
    await dark.addInitScript(() => localStorage.setItem("StackColorScheme", "dark"));
    const darkPage = await dark.newPage();
    await darkPage.goto(articleURL, { waitUntil: "networkidle" });
    const darkStyle = await darkPage.locator(".article-content blockquote.alert").first().evaluate((element) => ({
        background: getComputedStyle(element).backgroundColor,
        title: getComputedStyle(element.querySelector(".alert-title")).color,
    }));
    assert(darkStyle.background !== "rgb(255, 255, 255)", `dark callout background did not apply: ${JSON.stringify(darkStyle)}`);
    assert(darkStyle.title !== "rgb(0, 0, 0)", `dark callout title contrast is suspicious: ${JSON.stringify(darkStyle)}`);
    await dark.close();

    for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 568 }]) {
        const mobile = await browser.newContext({ viewport, locale: "zh-CN", reducedMotion: "reduce" });
        const mobilePage = await mobile.newPage();
        await mobilePage.goto(articleURL, { waitUntil: "networkidle" });
        await assertCallouts(mobilePage);
        await assertNoOverflow(mobilePage, `${viewport.width}px article`);
        const calloutWidth = await mobilePage.locator(".article-content blockquote.alert").first().evaluate((element) => element.getBoundingClientRect().width);
        assert(calloutWidth <= viewport.width, `${viewport.width}px callout exceeds viewport: ${calloutWidth}`);
        if (viewport.width === 390) {
            await mobilePage.screenshot({ path: path.join(evidenceDir, "site-p37-article-callouts-mobile-390x844.png"), fullPage: false });
        }
        await mobile.close();
    }

    const noScript = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", javaScriptEnabled: false });
    const noScriptPage = await noScript.newPage();
    const noScriptResponse = await noScriptPage.goto(articleURL, { waitUntil: "load" });
    assert(noScriptResponse?.status() === 200, "no-JS article route did not return 200");
    await assertCallouts(noScriptPage);
    await assertNoOverflow(noScriptPage, "no-JS article");
    await noScript.close();

    console.log("P37 article callout QA passed: semantic types, Chinese content, style, dark mode, no-JS, and 1280/390/320 responsive layouts.");
} finally {
    await browser.close();
}
