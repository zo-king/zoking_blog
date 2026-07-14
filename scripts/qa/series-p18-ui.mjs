import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const packagePath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!packagePath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(packagePath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const siteBase = (process.env.SERIES_SITE_BASE || "http://localhost:1313").replace(/\/$/, "");
const adminBase = (process.env.SERIES_ADMIN_BASE || "http://localhost:5173").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.SERIES_EVIDENCE_DIR || "docs/process/evidence");
const csrfToken = "qa-series-p18-csrf-123ba37c9d6d4d6cb64db647e20ec328";
const now = "2026-07-13T12:00:00Z";

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function envelope(data, pagination) {
  return { data, request_id: "series-p18-ui", ...(pagination ? { pagination } : {}) };
}

const series = [
  {
    id: "71000000-0000-0000-0000-000000000001",
    name: "长期主义实践",
    slug: "long-term-practice",
    description: "从系统、空间与生活三个角度整理长期实践。",
    cover_media_id: "72000000-0000-0000-0000-000000000001",
    sort_order: 10,
    enabled: true,
    post_count: 1,
    created_at: now,
    updated_at: now,
  },
  {
    id: "71000000-0000-0000-0000-000000000002",
    name: "已停用系列",
    slug: "disabled-series",
    description: "仅用于验证已有关系可读。",
    cover_media_id: null,
    sort_order: 20,
    enabled: false,
    post_count: 0,
    created_at: now,
    updated_at: now,
  },
];

const media = [{
  id: "72000000-0000-0000-0000-000000000001",
  filename: "series-cover.jpg",
  original_name: "系列封面.jpg",
  mime_type: "image/jpeg",
  size_bytes: 1024,
  width: 1200,
  height: 630,
  storage_key: "series-cover.jpg",
  public_url: "/media-files/series-cover.jpg",
  status: "ready",
  created_at: now,
}];

const post = {
  id: "73000000-0000-0000-0000-000000000001",
  author_id: "70000000-0000-0000-0000-000000000001",
  title: "一个让人愿意长期工作的空间",
  slug: "thoughtful-workspace",
  summary: "稳定空间带来的长期收益。",
  content_md: "## 把复杂度藏到流程里\n\n这是用于浏览器验收的只读正文。",
  status: "draft",
  visibility: "public",
  allow_comment: true,
  seo_title: "一个让人愿意长期工作的空间",
  seo_description: "稳定空间带来的长期收益。",
  cover_media_id: null,
  categories: [],
  tags: [],
  series_id: series[0].id,
  series_order: 2,
  series: series[0],
  created_at: now,
  updated_at: now,
};

async function installAdminFixture(context) {
  await context.addInitScript((token) => {
    sessionStorage.setItem("zoking_admin_session", "1");
    sessionStorage.setItem("zoking_admin_csrf", token);
  }, csrfToken);
  await context.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (!url.pathname.startsWith("/api/v1/") && !["/healthz", "/readyz"].includes(url.pathname)) {
      await route.continue();
      return;
    }
    const fulfill = (data, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(data) });
    const method = request.method().toUpperCase();
    if (url.pathname.startsWith("/api/v1/admin/") && !["GET", "HEAD", "OPTIONS"].includes(method)) {
      const receivedToken = request.headers()["x-csrf-token"] || "";
      assert(receivedToken === csrfToken, `${method} ${url.pathname} did not carry the expected CSRF token`);
    }
    if (url.pathname === "/healthz" || url.pathname === "/readyz") return fulfill(envelope({ status: "ok" }));
    if (url.pathname === "/api/v1/admin/auth/me") {
      return fulfill(envelope({
        id: "70000000-0000-0000-0000-000000000001",
        email: "series-ui@zoking.local",
        roles: ["editor"],
        permissions: ["post:read", "post:create", "post:update", "post:publish", "taxonomy:read", "taxonomy:manage", "media:read"],
      }));
    }
    if (url.pathname === "/api/v1/admin/categories" || url.pathname === "/api/v1/admin/tags") return fulfill(envelope([]));
    if (url.pathname === "/api/v1/admin/series") return fulfill(envelope(series));
    if (url.pathname === "/api/v1/admin/media") return fulfill(envelope(media, { page: 1, page_size: 100, total: 1, total_pages: 1 }));
    if (url.pathname === `/api/v1/admin/posts/${post.id}`) return fulfill(envelope(post));
    if (url.pathname === "/api/v1/admin/posts") return fulfill(envelope([post], { page: 1, page_size: 20, total: 1, total_pages: 1 }));
    return fulfill({ error: { code: "NOT_MOCKED", message: `${request.method()} ${url.pathname}` }, request_id: "series-p18-ui" }, 404);
  });
}

async function installPublicFixture(context) {
  await context.route("**/api/v1/public/posts/*/comments", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(envelope([])),
  }));
}

function collectRuntimeErrors(page) {
  const errors = [];
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(`console: ${message.text()}`);
  });
  return errors;
}

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });

try {
  const siteContext = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installPublicFixture(siteContext);
  const sitePage = await siteContext.newPage();
  const siteErrors = collectRuntimeErrors(sitePage);
  await sitePage.goto(`${siteBase}/p/thoughtful-workspace/`, { waitUntil: "networkidle" });
  const seriesModule = sitePage.locator(".article-series");
  await seriesModule.waitFor({ state: "visible" });
  assert((await seriesModule.getByRole("heading", { name: "长期主义实践" }).count()) === 1, "series heading is missing");
  const orders = await seriesModule.locator(".article-series__order").allTextContents();
  const titles = await seriesModule.locator(".article-series__title").allTextContents();
  assert(orders.join(",") === "1,2,3", `series order is unstable: ${orders.join(",")}`);
  assert(titles.length === 3 && titles[1].includes("长期工作的空间"), `unexpected series titles: ${titles.join(" | ")}`);
  assert((await seriesModule.locator('[aria-current="page"]').count()) === 1, "current series item is not unique");
  assert((await seriesModule.getByText("系列上一篇", { exact: true }).count()) === 1, "middle article previous link missing");
  assert((await seriesModule.getByText("系列下一篇", { exact: true }).count()) === 1, "middle article next link missing");
  await seriesModule.scrollIntoViewIfNeeded();
  await sitePage.screenshot({ path: path.join(evidenceDir, "series-p18-site-desktop-1280x800.png"), fullPage: false });

  await sitePage.goto(`${siteBase}/p/system-design-boundaries/`, { waitUntil: "networkidle" });
  assert((await sitePage.getByText("系列上一篇", { exact: true }).count()) === 0, "first series article exposes previous link");
  assert((await sitePage.getByText("系列下一篇", { exact: true }).count()) === 1, "first series article next link missing");
  await sitePage.goto(`${siteBase}/p/city-walk/`, { waitUntil: "networkidle" });
  assert((await sitePage.getByText("系列上一篇", { exact: true }).count()) === 1, "last series article previous link missing");
  assert((await sitePage.getByText("系列下一篇", { exact: true }).count()) === 0, "last series article exposes next link");
  assert(siteErrors.length === 0, `site runtime errors: ${siteErrors.join("; ")}`);
  await siteContext.close();

  const mobileSiteContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installPublicFixture(mobileSiteContext);
  const mobileSite = await mobileSiteContext.newPage();
  await mobileSite.goto(`${siteBase}/p/thoughtful-workspace/`, { waitUntil: "networkidle" });
  await mobileSite.locator(".article-series").scrollIntoViewIfNeeded();
  const siteWidth = await mobileSite.evaluate(() => ({ document: document.documentElement.scrollWidth, viewport: window.innerWidth }));
  assert(siteWidth.document <= siteWidth.viewport, `mobile site overflows: ${JSON.stringify(siteWidth)}`);
  await mobileSite.screenshot({ path: path.join(evidenceDir, "series-p18-site-mobile-390x844.png"), fullPage: false });
  await mobileSiteContext.close();

  const adminContext = await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installAdminFixture(adminContext);
  const adminPage = await adminContext.newPage();
  const adminErrors = collectRuntimeErrors(adminPage);
  await adminPage.goto(`${adminBase}/taxonomy`, { waitUntil: "networkidle" });
  await adminPage.getByRole("tab", { name: "系列" }).click();
  await adminPage.getByText("长期主义实践", { exact: true }).waitFor({ state: "visible" });
  assert((await adminPage.getByRole("button", { name: "编辑系列 长期主义实践" }).count()) === 1, "series edit action missing");
  await adminPage.getByRole("button", { name: "编辑系列 长期主义实践" }).click();
  const seriesDialog = adminPage.getByRole("dialog", { name: "编辑系列" });
  await seriesDialog.waitFor({ state: "visible" });
  assert((await adminPage.locator('input[value="long-term-practice"]').count()) === 1, "series slug was not restored into modal");
  await adminPage.waitForTimeout(350);
  await adminPage.screenshot({ path: path.join(evidenceDir, "series-p18-admin-modal-desktop-1280x720.png"), fullPage: false });
  await adminPage.getByRole("button", { name: "取消" }).click();
  await seriesDialog.waitFor({ state: "hidden" });
  await adminPage.screenshot({ path: path.join(evidenceDir, "series-p18-admin-desktop-1280x720.png"), fullPage: false });

  await adminPage.goto(`${adminBase}/posts/${post.id}/edit`, { waitUntil: "networkidle" });
  await adminPage.getByText("系列序号", { exact: true }).waitFor({ state: "visible" });
  const orderInput = adminPage.locator('.arco-form-item').filter({ hasText: "系列序号" }).locator("input");
  assert((await orderInput.inputValue()) === "2", `series order was not restored: ${await orderInput.inputValue()}`);
  assert((await adminPage.getByText("长期主义实践", { exact: true }).count()) >= 1, "selected series label is not visible in editor");
  assert(adminErrors.length === 0, `admin runtime errors: ${adminErrors.join("; ")}`);
  await adminContext.close();

  const mobileAdminContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", reducedMotion: "reduce" });
  await installAdminFixture(mobileAdminContext);
  const mobileAdmin = await mobileAdminContext.newPage();
  await mobileAdmin.goto(`${adminBase}/taxonomy`, { waitUntil: "networkidle" });
  await mobileAdmin.getByRole("tab", { name: "系列" }).click();
  await mobileAdmin.getByText("长期主义实践", { exact: true }).waitFor({ state: "visible" });
  const adminWidth = await mobileAdmin.evaluate(() => ({ document: document.documentElement.scrollWidth, viewport: window.innerWidth }));
  assert(adminWidth.document <= adminWidth.viewport, `mobile admin overflows: ${JSON.stringify(adminWidth)}`);
  await mobileAdmin.screenshot({ path: path.join(evidenceDir, "series-p18-admin-mobile-390x844.png"), fullPage: false });
  await mobileAdminContext.close();

  process.stdout.write("[series-p18] PASS site order/navigation, Admin management/editor, desktop/mobile overflow, runtime errors\n");
} finally {
  await browser.close();
}
