import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const requiredEnvironment = [
  "PLAYWRIGHT_PACKAGE_PATH",
  "SEC_P15_UI_ADMIN_BASE",
  "SEC_P15_UI_ADMIN_EMAIL",
  "SEC_P15_UI_ADMIN_PASSWORD",
  "SEC_P15_UI_AUTHOR_EMAIL",
  "SEC_P15_UI_AUTHOR_PASSWORD",
  "SEC_P15_UI_VIEWER_EMAIL",
  "SEC_P15_UI_VIEWER_PASSWORD",
  "SEC_P15_UI_AUTHOR_POST_TITLE",
  "SEC_P15_UI_ADMIN_POST_TITLE",
  "SEC_P15_UI_ADMIN_POST_ID",
  "SEC_P15_UI_ADMIN_PAGE_TITLE",
  "SEC_P15_UI_ADMIN_PAGE_ID",
  "SEC_P15_UI_EVIDENCE_DIR"
];

for (const name of requiredEnvironment) {
  if (!process.env[name]?.trim()) {
    throw new Error(`${name} is required`);
  }
}

const playwrightEntry = path.join(process.env.PLAYWRIGHT_PACKAGE_PATH, "index.js");
const playwrightModule = await import(pathToFileURL(playwrightEntry).href);
const playwright = playwrightModule.default ?? playwrightModule;
const { chromium } = playwright;

const adminBase = process.env.SEC_P15_UI_ADMIN_BASE.replace(/\/$/, "");
const evidenceDir = path.resolve(process.env.SEC_P15_UI_EVIDENCE_DIR);
const authorPostTitle = process.env.SEC_P15_UI_AUTHOR_POST_TITLE;
const adminPostTitle = process.env.SEC_P15_UI_ADMIN_POST_TITLE;
const adminPostID = process.env.SEC_P15_UI_ADMIN_POST_ID;
const adminPageTitle = process.env.SEC_P15_UI_ADMIN_PAGE_TITLE;
const adminPageID = process.env.SEC_P15_UI_ADMIN_PAGE_ID;
const results = [];
const evidence = [];

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function pass(name, detail) {
  results.push({ name, ok: true, detail });
  process.stdout.write(`[ui-blackbox] PASS ${name} - ${detail}\n`);
}

async function expectRendered(locator, message) {
  await locator.first().waitFor({ state: "visible", timeout: 15_000 });
  assert((await locator.count()) > 0, message);
}

async function expectAbsent(locator, message) {
  assert((await locator.count()) === 0, message);
}

async function expectEnabled(locator, label) {
  await expectRendered(locator, `${label} missing`);
  assert(!(await locator.first().isDisabled()), `${label} is disabled`);
}

async function expectAllDisabled(locator, label) {
  const count = await locator.count();
  assert(count > 0, `${label} has no rendered controls`);
  for (let index = 0; index < count; index += 1) {
    assert(await locator.nth(index).isDisabled(), `${label} control ${index + 1} is enabled`);
  }
}

async function openAdminSection(page, target, heading, message) {
  await page.goto(`${adminBase}${target}`, { waitUntil: "domcontentloaded" });
  await expectRendered(page.getByRole("heading", { name: heading, exact: true }), message);
  await page.waitForLoadState("networkidle");
}

async function assertNoViewportOverflow(page, label) {
  const dimensions = await page.evaluate(() => ({
    viewportWidth: window.innerWidth,
    documentWidth: document.documentElement.scrollWidth,
    bodyWidth: document.body.scrollWidth
  }));
  assert(
    dimensions.documentWidth <= dimensions.viewportWidth + 1 && dimensions.bodyWidth <= dimensions.viewportWidth + 1,
    `${label} has horizontal page overflow: ${JSON.stringify(dimensions)}`
  );
}

async function login(page, email, password, target = "/posts") {
  await page.goto(`${adminBase}${target}`, { waitUntil: "domcontentloaded" });
  await page.locator('input[placeholder="admin@example.com"]').fill(email);
  await page.locator('input[placeholder="请输入密码"]').fill(password);
  await page.getByRole("button", { name: "登录后台" }).click();
  await expectRendered(page.getByRole("heading", { name: "文章", exact: true }), `login did not open the content workspace for ${email}`);
}

async function openScenario(browser, name, credentials, viewport, target = "/posts") {
  const context = await browser.newContext({ viewport, locale: "zh-CN" });
  const page = await context.newPage();
  const runtimeErrors = [];
  page.on("pageerror", (error) => runtimeErrors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") runtimeErrors.push(`console: ${message.text()}`);
  });
  await login(page, credentials.email, credentials.password, target);
  return { name, context, page, runtimeErrors };
}

async function finishScenario(scenario) {
  assert(scenario.runtimeErrors.length === 0, `${scenario.name} runtime errors: ${scenario.runtimeErrors.join(" | ")}`);
  await scenario.context.close();
}

async function capture(page, filename) {
  const target = path.join(evidenceDir, filename);
  await page.screenshot({ path: target, fullPage: false });
  evidence.push(target);
}

await fs.mkdir(evidenceDir, { recursive: true });

const browser = await chromium.launch({
  channel: process.env.PLAYWRIGHT_BROWSER_CHANNEL || "msedge",
  headless: true
});

try {
  const admin = await openScenario(
    browser,
    "super-admin-desktop",
    { email: process.env.SEC_P15_UI_ADMIN_EMAIL, password: process.env.SEC_P15_UI_ADMIN_PASSWORD },
    { width: 1280, height: 720 }
  );
  await expectRendered(admin.page.getByRole("heading", { name: "文章", exact: true }), "super admin posts heading missing");
  await expectRendered(admin.page.getByText(authorPostTitle, { exact: true }), "super admin cannot see author post");
  await expectRendered(admin.page.getByText(adminPostTitle, { exact: true }), "super admin cannot see admin post");
  await expectRendered(admin.page.getByRole("button", { name: "新建文章" }), "super admin create-post action missing");
  assert((await admin.page.getByRole("button", { name: "编辑文章" }).count()) > 0, "super admin edit-post actions missing");
  assert((await admin.page.getByRole("button", { name: "归档文章" }).count()) > 0, "super admin archive-post actions missing");
  const adminNavigation = admin.page.getByRole("navigation", { name: "后台主导航" }).first();
  await expectRendered(adminNavigation.getByText("发布", { exact: true }), "super admin publishing navigation missing");
  await assertNoViewportOverflow(admin.page, "super admin posts desktop");
  await capture(admin.page, "sec-p15-super-admin-posts-desktop-1280x720.png");

  await admin.page.goto(`${adminBase}/pages`, { waitUntil: "domcontentloaded" });
  await expectRendered(admin.page.getByRole("heading", { name: "独立页面", exact: true }), "super admin pages heading missing");
  await expectRendered(admin.page.getByText(adminPageTitle, { exact: true }), "super admin cannot see admin page");
  await expectRendered(admin.page.getByRole("button", { name: "新建页面" }), "super admin create-page action missing");
  assert((await admin.page.getByText("编辑", { exact: true }).count()) > 0, "super admin edit-page actions missing");
  assert((await admin.page.getByText("归档", { exact: true }).count()) > 0, "super admin archive-page actions missing");
  await assertNoViewportOverflow(admin.page, "super admin pages desktop");

  await openAdminSection(admin.page, "/taxonomy", "分类与标签", "super admin taxonomy heading missing");
  await expectEnabled(admin.page.getByRole("button", { name: "创建分类", exact: true }), "super admin create-category action");
  await expectRendered(admin.page.getByRole("columnheader", { name: "操作", exact: true }), "super admin taxonomy operation column missing");
  await expectRendered(admin.page.getByRole("button", { name: /^删除分类 .+/ }), "super admin delete-category action missing");
  await admin.page.getByRole("tab", { name: "标签", exact: true }).click();
  await expectEnabled(admin.page.getByRole("button", { name: "创建标签", exact: true }), "super admin create-tag action");
  await expectRendered(admin.page.getByRole("button", { name: /^删除标签 .+/ }), "super admin delete-tag action missing");

  await openAdminSection(admin.page, "/media", "媒体资产", "super admin media heading missing");
  await expectEnabled(admin.page.getByRole("button", { name: "上传图片", exact: true }), "super admin media upload action");
  await expectEnabled(admin.page.getByRole("button", { name: "维护", exact: true }), "super admin media maintenance action");
  await expectRendered(admin.page.getByRole("button", { name: /^复制 .+ 的媒体地址$/ }), "super admin copy-media-URL action missing");
  await expectRendered(admin.page.getByRole("button", { name: /^将 .+ 插入 Markdown$/ }), "super admin insert-media-Markdown action missing");
  await expectRendered(admin.page.getByRole("button", { name: /^删除 .+/ }), "super admin delete-media action missing");

  await openAdminSection(admin.page, "/comments", "评论审核", "super admin comments heading missing");
  await expectRendered(admin.page.getByRole("columnheader", { name: "操作", exact: true }), "super admin comment operation column missing");

  await openAdminSection(admin.page, "/settings", "站点设置", "super admin settings heading missing");
  await expectEnabled(admin.page.getByRole("button", { name: "保存", exact: true }), "super admin settings save action");
  await expectEnabled(admin.page.getByRole("button", { name: "预览", exact: true }), "super admin settings preview action");
  await expectEnabled(admin.page.getByRole("button", { name: "发布站点", exact: true }), "super admin settings publish action");

  await adminNavigation.getByText("发布", { exact: true }).click();
  await admin.page.waitForURL(/\/publishing(?:\?|$)/);
  await expectAbsent(admin.page.getByText("无权访问", { exact: true }), "super admin was denied publishing center");
  pass("super-admin-capabilities", "global content, object write actions, settings, and publishing center remain available");
  await finishScenario(admin);

  const author = await openScenario(
    browser,
    "author-desktop",
    { email: process.env.SEC_P15_UI_AUTHOR_EMAIL, password: process.env.SEC_P15_UI_AUTHOR_PASSWORD },
    { width: 1280, height: 720 }
  );
  await expectRendered(author.page.getByRole("heading", { name: "文章", exact: true }), "author posts heading missing");
  await expectRendered(author.page.getByText(authorPostTitle, { exact: true }), "author cannot see own draft");
  await expectRendered(author.page.getByText("共 1 篇文章", { exact: true }), "author list total is not owner-scoped");
  await expectAbsent(author.page.getByText(adminPostTitle, { exact: true }), "author can see another user's post");
  await expectRendered(author.page.getByRole("button", { name: "新建文章" }), "author create-post action missing");
  assert((await author.page.getByRole("button", { name: "编辑文章" }).count()) > 0, "author cannot edit own draft");
  await expectAbsent(author.page.getByRole("button", { name: "归档文章" }), "author received archive-post action");
  const authorNavigation = author.page.getByRole("navigation", { name: "后台主导航" }).first();
  await expectAbsent(authorNavigation.getByText("发布", { exact: true }), "author received publishing navigation");

  await author.page.getByRole("button", { name: "编辑文章" }).first().click();
  await author.page.waitForURL(/\/posts\/[0-9a-f-]+\/edit$/);
  await expectRendered(author.page.getByRole("heading", { name: "文章编辑", exact: true }), "author editor did not open");
  await expectAbsent(author.page.getByRole("button", { name: "发布", exact: true }), "author received publish action");
  assert(!(await author.page.getByRole("button", { name: "保存", exact: true }).isDisabled()), "author save action is disabled");
  assert(!(await author.page.getByRole("button", { name: "预览", exact: true }).isDisabled()), "author preview action is disabled");
  const authorStatusSelectClass = await author.page.locator(".editor-sidebar .arco-select").first().getAttribute("class");
  assert(authorStatusSelectClass?.includes("arco-select-disabled"), "author can change published status in the editor");
  await assertNoViewportOverflow(author.page, "author editor desktop");
  await capture(author.page, "sec-p15-author-editor-desktop-1280x720.png");

  await author.page.goto(`${adminBase}/pages`, { waitUntil: "domcontentloaded" });
  await expectRendered(author.page.getByRole("heading", { name: "独立页面", exact: true }), "author pages heading missing");
  await expectRendered(author.page.getByText("共 0 个页面", { exact: true }), "author page list total is not owner-scoped");
  await expectAbsent(author.page.getByText(adminPageTitle, { exact: true }), "author can see another user's page");
  await expectAbsent(author.page.getByRole("button", { name: "新建页面" }), "author received create-page action");
  await expectAbsent(author.page.getByText("编辑", { exact: true }), "author received edit-page action");
  await expectAbsent(author.page.getByText("归档", { exact: true }), "author received archive-page action");

  await author.page.goto(`${adminBase}/pages/new`, { waitUntil: "domcontentloaded" });
  await expectRendered(author.page.getByText("无权访问", { exact: true }), "author page-create deep link was not denied");
  await expectAbsent(author.page.getByRole("heading", { name: "页面编辑器", exact: true }), "author page-create deep link rendered an editor");
  await author.page.goto(`${adminBase}/pages/${adminPageID}/edit`, { waitUntil: "domcontentloaded" });
  await expectRendered(author.page.getByText("无权访问", { exact: true }), "author page-edit deep link was not denied");
  await expectAbsent(author.page.getByRole("heading", { name: "页面编辑器", exact: true }), "author page-edit deep link rendered an editor");

  await openAdminSection(author.page, "/taxonomy", "分类与标签", "author taxonomy heading missing");
  await expectAbsent(author.page.getByRole("button", { name: "创建分类", exact: true }), "author received create-category action");
  await expectAbsent(author.page.getByRole("button", { name: /^删除分类 .+/ }), "author received delete-category action");
  await expectAbsent(author.page.getByRole("columnheader", { name: "操作", exact: true }), "author received taxonomy operation column");
  await author.page.getByRole("tab", { name: "标签", exact: true }).click();
  await expectAbsent(author.page.getByRole("button", { name: "创建标签", exact: true }), "author received create-tag action");
  await expectAbsent(author.page.getByRole("button", { name: /^删除标签 .+/ }), "author received delete-tag action");
  await expectAbsent(author.page.getByRole("columnheader", { name: "操作", exact: true }), "author received tag operation column");

  await openAdminSection(author.page, "/media", "媒体资产", "author media heading missing");
  await expectEnabled(author.page.getByRole("button", { name: "上传图片", exact: true }), "author media upload action");
  await expectRendered(author.page.getByRole("button", { name: /^复制 .+ 的媒体地址$/ }), "author copy-media-URL action missing");
  await expectRendered(author.page.getByRole("button", { name: /^将 .+ 插入 Markdown$/ }), "author insert-media-Markdown action missing");
  await expectAbsent(author.page.getByRole("button", { name: "维护", exact: true }), "author received media maintenance action");
  await expectAbsent(author.page.getByRole("button", { name: /^删除 .+/ }), "author received delete-media action");

  await openAdminSection(author.page, "/comments", "评论审核", "author comments heading missing");
  await expectAbsent(author.page.getByRole("columnheader", { name: "操作", exact: true }), "author received comment operation column");

  await openAdminSection(author.page, "/settings", "站点设置", "author settings heading missing");
  await expectAbsent(author.page.getByRole("button", { name: "保存", exact: true }), "author received settings save action");
  await expectAbsent(author.page.getByRole("button", { name: "预览", exact: true }), "author received settings preview action");
  await expectAbsent(author.page.getByRole("button", { name: "发布站点", exact: true }), "author received settings publish action");
  await expectAllDisabled(
    author.page.locator(".settings-workbench form").locator("input, textarea, button"),
    "author settings form"
  );

  await author.page.goto(`${adminBase}/publishing`, { waitUntil: "domcontentloaded" });
  await expectRendered(author.page.getByText("无权访问", { exact: true }), "author direct publishing route was not denied");
  pass("author-owner-scope", "only own draft and allowed media writes remain; privileged object and settings actions are absent");
  await finishScenario(author);

  const authorMobile = await openScenario(
    browser,
    "author-mobile",
    { email: process.env.SEC_P15_UI_AUTHOR_EMAIL, password: process.env.SEC_P15_UI_AUTHOR_PASSWORD },
    { width: 390, height: 844 }
  );
  await expectRendered(authorMobile.page.getByText(authorPostTitle, { exact: true }), "author mobile list did not load own draft");
  await authorMobile.page.getByRole("button", { name: "编辑文章" }).first().click();
  await authorMobile.page.waitForURL(/\/posts\/[0-9a-f-]+\/edit$/);
  await expectAbsent(authorMobile.page.getByRole("button", { name: "发布", exact: true }), "author mobile editor received publish action");
  await authorMobile.page.getByRole("button", { name: "打开导航" }).click();
  const mobileDrawer = authorMobile.page.locator(".mobile-nav-drawer");
  await mobileDrawer.waitFor({ state: "visible" });
  await expectAbsent(mobileDrawer.getByText("发布", { exact: true }), "author mobile navigation received publishing entry");
  await authorMobile.page.locator(".arco-drawer-mask").last().click({ position: { x: 360, y: 400 } });
  await mobileDrawer.waitFor({ state: "hidden" });
  await assertNoViewportOverflow(authorMobile.page, "author editor mobile");
  await capture(authorMobile.page, "sec-p15-author-editor-mobile-390x844.png");
  pass("author-mobile-layout", "permission-aware editor fits 390x844 without page overflow");
  await finishScenario(authorMobile);

  const viewer = await openScenario(
    browser,
    "viewer-desktop",
    { email: process.env.SEC_P15_UI_VIEWER_EMAIL, password: process.env.SEC_P15_UI_VIEWER_PASSWORD },
    { width: 1280, height: 720 }
  );
  await expectRendered(viewer.page.getByRole("heading", { name: "文章", exact: true }), "viewer posts heading missing");
  await expectRendered(viewer.page.getByText(authorPostTitle, { exact: true }), "viewer cannot globally read author post");
  await expectRendered(viewer.page.getByText(adminPostTitle, { exact: true }), "viewer cannot globally read admin post");
  await expectAbsent(viewer.page.getByRole("button", { name: "新建文章" }), "viewer received create-post action");
  await expectAbsent(viewer.page.getByRole("button", { name: "编辑文章" }), "viewer received edit-post action");
  await expectAbsent(viewer.page.getByRole("button", { name: "归档文章" }), "viewer received archive-post action");
  const viewerNavigation = viewer.page.getByRole("navigation", { name: "后台主导航" }).first();
  await expectRendered(viewerNavigation.getByText("发布", { exact: true }), "viewer publishing read navigation missing");
  await assertNoViewportOverflow(viewer.page, "viewer posts desktop");
  await capture(viewer.page, "sec-p15-viewer-posts-desktop-1280x720.png");

  await viewer.page.goto(`${adminBase}/posts/${adminPostID}/edit`, { waitUntil: "domcontentloaded" });
  await expectRendered(viewer.page.getByText("无权访问", { exact: true }), "viewer post-edit deep link was not denied");
  await expectAbsent(viewer.page.getByRole("heading", { name: "文章编辑", exact: true }), "viewer post-edit deep link rendered an editor");

  await viewer.page.goto(`${adminBase}/pages`, { waitUntil: "domcontentloaded" });
  await expectRendered(viewer.page.getByRole("heading", { name: "独立页面", exact: true }), "viewer pages heading missing");
  await expectRendered(viewer.page.getByText(adminPageTitle, { exact: true }), "viewer cannot globally read admin page");
  await expectAbsent(viewer.page.getByRole("button", { name: "新建页面" }), "viewer received create-page action");
  await expectAbsent(viewer.page.getByText("编辑", { exact: true }), "viewer received edit-page action");
  await expectAbsent(viewer.page.getByText("归档", { exact: true }), "viewer received archive-page action");

  await viewer.page.goto(`${adminBase}/pages/${adminPageID}/edit`, { waitUntil: "domcontentloaded" });
  await expectRendered(viewer.page.getByText("无权访问", { exact: true }), "viewer page-edit deep link was not denied");
  await expectAbsent(viewer.page.getByRole("heading", { name: "页面编辑器", exact: true }), "viewer page-edit deep link rendered an editor");

  await openAdminSection(viewer.page, "/taxonomy", "分类与标签", "viewer taxonomy heading missing");
  await expectAbsent(viewer.page.getByRole("button", { name: "创建分类", exact: true }), "viewer received create-category action");
  await expectAbsent(viewer.page.getByRole("button", { name: /^删除分类 .+/ }), "viewer received delete-category action");
  await expectAbsent(viewer.page.getByRole("columnheader", { name: "操作", exact: true }), "viewer received taxonomy operation column");
  await viewer.page.getByRole("tab", { name: "标签", exact: true }).click();
  await expectAbsent(viewer.page.getByRole("button", { name: "创建标签", exact: true }), "viewer received create-tag action");
  await expectAbsent(viewer.page.getByRole("button", { name: /^删除标签 .+/ }), "viewer received delete-tag action");
  await expectAbsent(viewer.page.getByRole("columnheader", { name: "操作", exact: true }), "viewer received tag operation column");

  await openAdminSection(viewer.page, "/media", "媒体资产", "viewer media heading missing");
  await expectRendered(viewer.page.getByRole("button", { name: /^复制 .+ 的媒体地址$/ }), "viewer copy-media-URL action missing");
  await expectAbsent(viewer.page.getByRole("button", { name: "上传图片", exact: true }), "viewer received media upload action");
  await expectAbsent(viewer.page.getByRole("button", { name: "维护", exact: true }), "viewer received media maintenance action");
  await expectAbsent(viewer.page.getByRole("button", { name: /^将 .+ 插入 Markdown$/ }), "viewer received insert-media-Markdown action");
  await expectAbsent(viewer.page.getByRole("button", { name: /^删除 .+/ }), "viewer received delete-media action");

  await openAdminSection(viewer.page, "/comments", "评论审核", "viewer comments heading missing");
  await expectAbsent(viewer.page.getByRole("columnheader", { name: "操作", exact: true }), "viewer received comment operation column");

  await openAdminSection(viewer.page, "/settings", "站点设置", "viewer settings heading missing");
  await expectAbsent(viewer.page.getByRole("button", { name: "保存", exact: true }), "viewer received settings save action");
  await expectAbsent(viewer.page.getByRole("button", { name: "预览", exact: true }), "viewer received settings preview action");
  await expectAbsent(viewer.page.getByRole("button", { name: "发布站点", exact: true }), "viewer received settings publish action");
  await expectAllDisabled(
    viewer.page.locator(".settings-workbench form").locator("input, textarea, button"),
    "viewer settings form"
  );

  await viewerNavigation.getByText("发布", { exact: true }).click();
  await viewer.page.waitForURL(/\/publishing(?:\?|$)/);
  await expectAbsent(viewer.page.getByText("无权访问", { exact: true }), "viewer was denied read-only publishing center");
  await expectAbsent(viewer.page.getByRole("button", { name: "重试", exact: true }), "viewer received publish-job retry action");
  await expectAbsent(viewer.page.getByRole("button", { name: "取消", exact: true }), "viewer received publish-job cancel action");
  await viewer.page.getByRole("tab", { name: /正式版本/ }).click();
  await expectAbsent(viewer.page.getByRole("button", { name: "切换", exact: true }), "viewer received release promote action");
  await expectAbsent(viewer.page.getByRole("button", { name: "清理预演", exact: true }), "viewer received release cleanup preview action");
  await expectAbsent(viewer.page.getByRole("button", { name: "清理旧版本", exact: true }), "viewer received release cleanup action");
  await viewer.page.getByRole("tab", { name: /预览构建/ }).click();
  await expectAbsent(viewer.page.getByRole("button", { name: "清理预演", exact: true }), "viewer received preview cleanup preview action");
  await expectAbsent(viewer.page.getByRole("button", { name: "执行清理", exact: true }), "viewer received preview cleanup action");
  pass("viewer-read-only", "global content remains visible while taxonomy, media, comment, and settings writes are absent");
  await finishScenario(viewer);
} finally {
  await browser.close();
}

process.stdout.write(`${JSON.stringify({ ok: true, passed: results.length, results, evidence }, null, 2)}\n`);
