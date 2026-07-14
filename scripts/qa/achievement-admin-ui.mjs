import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const adminBase = (process.env.P22_ADMIN_BASE || "http://localhost:5173").replace(/\/$/, "");
const apiBase = (process.env.P22_API_BASE || "http://localhost:18080").replace(/\/$/, "");
const assert = (condition, message) => { if (!condition) throw new Error(message); };

const loginResponse = await fetch(`${apiBase}/api/v1/admin/auth/login`, {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ email: "admin@zoking.local", password: "ChangeMe123!" }),
});
assert(loginResponse.ok, `admin login failed: ${loginResponse.status}`);
const csrfToken = (await loginResponse.json()).data.csrf_token;
const setCookies = loginResponse.headers.getSetCookie?.() ?? [loginResponse.headers.get("set-cookie")].filter(Boolean);
const cookiePairs = setCookies.flatMap((value) => value.split(/,(?=\s*[A-Za-z0-9_]+=)/)).map((value) => value.split(";", 1)[0].trim());
assert(cookiePairs.some((value) => value.startsWith("zoking_admin_access=")), "login did not return the admin session cookie");
const authHeaders = { cookie: cookiePairs.join("; "), "x-csrf-token": csrfToken, "content-type": "application/json" };
const apiURL = new URL(apiBase);
const browserCookies = cookiePairs.map((pair) => {
  const separator = pair.indexOf("=");
  return { name: pair.slice(0, separator), value: pair.slice(separator + 1), domain: apiURL.hostname, path: "/api/v1/admin", httpOnly: true, secure: apiURL.protocol === "https:", sameSite: "Strict" };
});

async function prepareContext(context) {
  await context.addCookies(browserCookies);
  await context.addInitScript((csrf) => {
    sessionStorage.setItem("zoking_admin_session", "1");
    sessionStorage.setItem("zoking_admin_csrf", csrf);
  }, csrfToken);
}

const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });
let createdID = "";

try {
  for (const viewport of [{ width: 1280, height: 800 }, { width: 390, height: 844 }, { width: 320, height: 568 }]) {
    const context = await browser.newContext({ viewport, locale: "zh-CN" });
    await prepareContext(context);
    const page = await context.newPage();
    const browserErrors = [];
    page.on("pageerror", (error) => browserErrors.push(error.message));
    page.on("console", (message) => { if (message.type() === "error") browserErrors.push(message.text()); });
    const response = await page.goto(`${adminBase}/achievements`, { waitUntil: "networkidle" });
    assert(response?.ok(), `${viewport.width}px achievements route failed`);
    await page.getByRole("heading", { name: "成果", exact: true }).waitFor();
    const dimensions = await page.locator("html").evaluate((element) => ({ width: innerWidth, scrollWidth: element.scrollWidth }));
    assert(dimensions.scrollWidth <= dimensions.width + 1, `${viewport.width}px admin has horizontal overflow`);
    assert(browserErrors.length === 0, `${viewport.width}px browser errors: ${browserErrors.join(" | ")}`);
    await context.close();
  }

  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN" });
  await prepareContext(context);
  const page = await context.newPage();
  await page.goto(`${adminBase}/achievements/new`, { waitUntil: "networkidle" });
  await page.getByLabel("成果名称").fill("P22 后台界面验收");
  await page.getByLabel("组织 / 颁发方").fill("zoking.tech");
  await page.getByLabel("发生日期").fill("2026-07-13");
  const createResponse = page.waitForResponse((res) => res.url().endsWith("/api/v1/admin/achievements") && res.request().method() === "POST");
  await page.getByRole("button", { name: "保存" }).click();
  const createdResponse = await createResponse;
  assert(createdResponse.status() === 201, `achievement create returned ${createdResponse.status()}`);
  createdID = (await createdResponse.json()).data.id;
  await page.waitForURL(new RegExp(`/achievements/${createdID}/edit$`));
  await page.waitForFunction(() => document.querySelector('input[placeholder="例如：年度优秀开源项目"]')?.value === "P22 后台界面验收");
  assert((await page.getByLabel("成果名称").inputValue()) === "P22 后台界面验收", "saved achievement was not restored in editor");
  await context.close();
} finally {
  if (createdID) {
    const deleteResponse = await fetch(`${apiBase}/api/v1/admin/achievements/${createdID}`, { method: "DELETE", headers: authHeaders });
    assert(deleteResponse.ok, `cleanup achievement failed: ${deleteResponse.status}`);
  }
  await browser.close();
}

console.log("P22 achievement Admin QA passed: routing, responsive layouts, create/edit round trip, and cleanup.");
