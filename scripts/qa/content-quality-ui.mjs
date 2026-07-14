import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const playwrightPath = process.env.PLAYWRIGHT_PACKAGE_PATH?.trim();
if (!playwrightPath) throw new Error("PLAYWRIGHT_PACKAGE_PATH is required");
const playwrightModule = await import(pathToFileURL(path.join(playwrightPath, "index.js")).href);
const { chromium } = playwrightModule.default ?? playwrightModule;

const adminBase = (process.env.CONTENT_QUALITY_ADMIN_BASE || "http://localhost:5173").replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.CONTENT_QUALITY_EVIDENCE_DIR || "docs/process/evidence");
const requests = [];
const csrfRejections = [];
const csrfToken = "qa-content-quality-csrf-7a8d00d527c04f439e0b2a4fb6a9e89f";
const now = "2026-07-13T12:00:00Z";

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function envelope(data, pagination) {
  return { data, request_id: "quality-ui", ...(pagination ? { pagination } : {}) };
}

function report(payload, kind) {
  const blocked = String(payload.content_md || "").toLowerCase().includes("javascript:");
  return {
    status: blocked ? "blocked" : "warning",
    ready: !blocked,
    score: blocked ? 58 : 88,
    error_count: blocked ? 1 : 0,
    warning_count: 2,
    content_hash: blocked ? "b".repeat(64) : "a".repeat(64),
    policy_version: "2026-07-13.1",
    issues: [
      ...(blocked ? [{ code: "UNSAFE_URL", severity: "error", field: "content_md", message: "正文包含不安全或不受支持的链接协议" }] : []),
      kind === "post"
        ? { code: "COVER_MISSING", severity: "warning", field: "cover_media_id", message: "建议为文章选择封面图" }
        : { code: "SEO_DESCRIPTION_LENGTH", severity: "warning", field: "seo_description", message: "搜索摘要建议保持在 40 到 160 个字符" },
      { code: "CONTENT_SHORT", severity: "warning", field: "content_md", message: "正文较短，请确认内容已经完整" },
    ],
  };
}

function postFrom(payload, status = "draft") {
  return {
    id: "91000000-0000-0000-0000-000000000001",
    title: payload.title,
    slug: payload.slug,
    summary: payload.summary || "",
    content_md: payload.content_md || "",
    status,
    visibility: payload.visibility || "public",
    allow_comment: payload.allow_comment ?? true,
    published_at: status === "published" ? now : null,
    author_id: "90000000-0000-0000-0000-000000000001",
    cover_media_id: null,
    cover_media: null,
    seo_title: payload.seo_title || payload.title,
    seo_description: payload.seo_description || payload.summary || "",
    categories: [],
    tags: [],
    created_at: now,
    updated_at: now,
  };
}

async function installMockAPI(context) {
  await context.addInitScript((token) => {
    sessionStorage.setItem("zoking_admin_session", "1");
    sessionStorage.setItem("zoking_admin_csrf", token);
  }, csrfToken);
  await context.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (!url.pathname.startsWith("/api/v1/") && url.pathname !== "/healthz" && url.pathname !== "/readyz") {
      await route.continue();
      return;
    }

    const method = request.method().toUpperCase();
    const body = request.postData() ? JSON.parse(request.postData()) : {};
    const key = `${method} ${url.pathname}`;
    const fulfill = (data, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(data) });
    if (url.pathname.startsWith("/api/v1/admin/") && !["GET", "HEAD", "OPTIONS"].includes(method)) {
      const receivedToken = request.headers()["x-csrf-token"] || "";
      if (receivedToken !== csrfToken) {
        csrfRejections.push(key);
        await fulfill({ error: { code: "CSRF_FAILED", message: "invalid CSRF token" }, request_id: "quality-ui" }, 403);
        return;
      }
      assert(receivedToken === csrfToken, `${key} did not carry the expected CSRF token`);
      requests.push(key);
    }

    if (url.pathname === "/api/v1/admin/auth/me") {
      await fulfill(envelope({
        id: "90000000-0000-0000-0000-000000000001",
        email: "quality-ui@zoking.local",
        roles: ["editor"],
        permissions: ["post:read", "post:create", "post:update", "post:publish", "page:read", "page:create", "page:update", "page:publish", "taxonomy:read"],
      }));
      return;
    }
    if (url.pathname.endsWith("/quality-check")) {
      await fulfill(envelope(report(body, url.pathname.includes("/pages/") ? "page" : "post")));
      return;
    }
    if (url.pathname === "/api/v1/admin/categories" || url.pathname === "/api/v1/admin/tags") {
      await fulfill(envelope([]));
      return;
    }
    if (url.pathname === "/api/v1/admin/posts" && request.method() === "POST") {
      await fulfill(envelope(postFrom(body)), 201);
      return;
    }
    if (url.pathname === "/api/v1/admin/posts/91000000-0000-0000-0000-000000000001/publish") {
      await fulfill(envelope({
        post: postFrom(body.title ? body : { title: "新文章", slug: "quality-ui-post", content_md: "安全正文", visibility: "public" }, "published"),
        job: { id: "92000000-0000-0000-0000-000000000001", status: "requested", job_type: "post" },
      }), 202);
      return;
    }
    if (url.pathname === "/api/v1/admin/posts/91000000-0000-0000-0000-000000000001" && request.method() === "GET") {
      await fulfill(envelope(postFrom({ title: "新文章", slug: "quality-ui-post", content_md: "安全正文", visibility: "public" }, "published")));
      return;
    }
    if (url.pathname === "/healthz" || url.pathname === "/readyz") {
      await fulfill(envelope({ status: "ok" }));
      return;
    }
    await fulfill({ error: { code: "MOCK_ROUTE_MISSING", message: key }, request_id: "quality-ui" }, 404);
  });
}

async function assertNoOverflow(page, label) {
  const widths = await page.evaluate(() => ({ viewport: innerWidth, html: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  assert(widths.html <= widths.viewport + 1 && widths.body <= widths.viewport + 1, `${label} overflow: ${JSON.stringify(widths)}`);
}

await fs.mkdir(evidenceDir, { recursive: true });
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge", headless: true });
try {
  const desktop = await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "zh-CN" });
  await installMockAPI(desktop);
  const page = await desktop.newPage();
  await page.goto(`${adminBase}/posts/new`, { waitUntil: "networkidle" });
  await page.getByRole("heading", { name: "文章编辑", exact: true }).waitFor();
  const textarea = page.locator("textarea").first();
  const missingCSRF = await page.evaluate(async () => {
    const response = await fetch("/api/v1/admin/posts/quality-check", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title: "CSRF negative probe", content_md: "probe" }),
    });
    return { status: response.status, body: await response.json() };
  });
  assert(missingCSRF.status === 403, `missing CSRF mock request returned ${missingCSRF.status}`);
  assert(missingCSRF.body?.error?.code === "CSRF_FAILED", "missing CSRF mock request did not return CSRF_FAILED");
  assert(csrfRejections.includes("POST /api/v1/admin/posts/quality-check"), "missing CSRF request was not rejected by the mock");

  await page.getByRole("button", { name: "发布检查", exact: true }).click();
  await page.getByText("可以发布", { exact: true }).waitFor();
  await page.waitForTimeout(500);
  let drawer = page.locator(".content-quality-drawer");
  let box = await drawer.boundingBox();
  assert(box && box.width >= 370 && box.width <= 390, `desktop drawer width is ${box?.width}`);
  assert(box && Math.abs(box.x + box.width - 1280) <= 1, `desktop drawer is not aligned to viewport: ${JSON.stringify(box)}`);
  await page.screenshot({ path: path.join(evidenceDir, "content-quality-p17-desktop-1280x720.png"), fullPage: false });

  await textarea.fill("[危险链接](javascript:alert(1))");
  await drawer.waitFor({ state: "hidden" });
  await page.getByRole("button", { name: "发布检查", exact: true }).click();
  await page.getByText("暂不可发布", { exact: true }).waitFor();
  await page.getByText("正文包含不安全或不受支持的链接协议", { exact: true }).waitFor();

  await textarea.fill("这是修复后的安全正文。".repeat(30));
  await drawer.waitFor({ state: "hidden" });
  requests.length = 0;
  await page.getByRole("button", { name: "发布", exact: true }).click();
  await page.getByText("文章发布任务已进入队列", { exact: true }).waitFor();
  const writes = requests.filter((value) => value.includes("/posts"));
  assert(writes.length >= 3, `publish request sequence is incomplete: ${JSON.stringify(writes)}`);
  assert(writes[0] === "POST /api/v1/admin/posts/quality-check", `quality was not first: ${JSON.stringify(writes)}`);
  assert(writes[1] === "POST /api/v1/admin/posts", `save was not second: ${JSON.stringify(writes)}`);
  assert(writes[2].endsWith("/publish"), `publish was not third: ${JSON.stringify(writes)}`);
  await assertNoOverflow(page, "desktop quality workflow");
  await desktop.close();

  const mobile = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN" });
  await installMockAPI(mobile);
  const mobilePage = await mobile.newPage();
  await mobilePage.goto(`${adminBase}/pages/new`, { waitUntil: "networkidle" });
  await mobilePage.getByRole("heading", { name: "页面编辑器", exact: true }).waitFor();
  await mobilePage.getByRole("button", { name: "发布检查", exact: true }).click();
  await mobilePage.getByText("可以发布", { exact: true }).waitFor();
  await mobilePage.waitForTimeout(500);
  drawer = mobilePage.locator(".content-quality-drawer");
  box = await drawer.boundingBox();
  assert(box && box.width >= 388 && box.width <= 391, `mobile drawer width is ${box?.width}`);
  assert(box && Math.abs(box.x) <= 1, `mobile drawer is not aligned to viewport: ${JSON.stringify(box)}`);
  await assertNoOverflow(mobilePage, "mobile quality drawer");
  await mobilePage.screenshot({ path: path.join(evidenceDir, "content-quality-p17-mobile-390x844.png"), fullPage: false });
  await mobile.close();

  process.stdout.write("[content-quality-ui] PASS CSRF enforcement, desktop drawer, invalidation, blocking report, publish order, and mobile layout\n");
} finally {
  await browser.close();
}
