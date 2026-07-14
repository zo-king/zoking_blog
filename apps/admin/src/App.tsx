import { Alert, Form, Input, Modal, Spin } from "@arco-design/web-react";
import { IconBook, IconDashboard, IconFile, IconImage, IconMessage, IconSafe, IconSend, IconSettings, IconStar, IconStorage, IconTag } from "@arco-design/web-react/icon";
import { useEffect, useMemo, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { AdminLayout, type AdminNavItem } from "./layout/AdminLayout";
import { ContentQualityPanel } from "./components/ContentQualityPanel";
import { useAdminSession } from "./hooks/useAdminSession";
import { useAdminData } from "./hooks/useAdminData";
import { useAchievementAdmin } from "./hooks/useAchievementAdmin";
import { useContentAdminCommands } from "./hooks/useContentAdminCommands";
import { useIdentityCommands } from "./hooks/useIdentityCommands";
import { usePublishingCommands } from "./hooks/usePublishingCommands";
import { useEditorialCommands } from "./hooks/useEditorialCommands";
import { DashboardPage } from "./pages/DashboardPage";
import { AuditPage } from "./pages/AuditPage";
import { AchievementsPage } from "./pages/AchievementsPage";
import { CommentsPage } from "./pages/CommentsPage";
import { MediaPage } from "./pages/MediaPage";
import { PagesPage } from "./pages/PagesPage";
import { PostsPage } from "./pages/PostsPage";
import { PublishingPage } from "./pages/PublishingPage";
import { SettingsPage } from "./pages/SettingsPage";
import { TaxonomyPage } from "./pages/TaxonomyPage";
import { UserManagementPage } from "./pages/UserManagementPage";
import { LoginPage } from "./pages/LoginPage";
import type { AchievementFormValues, AdminSection, CategoryFormValues, PageFormValues, PostFormValues, SeriesFormValues, SiteSettings, TagFormValues } from "./types/admin";

export function App({ section, editorID }: { section: AdminSection; editorID?: string }) {
  const session = useAdminSession();

  if (session.initializing) {
    return <main style={{ display: "grid", minHeight: "100vh", placeItems: "center" }}><Spin size={32} /></main>;
  }

  if (!session.token) {
    return <LoginPage busy={session.loginBusy} onLogin={session.login} />;
  }

  return <AuthenticatedAdmin section={section} editorID={editorID} session={session} />;
}

function AuthenticatedAdmin({ section, editorID, session }: { section: AdminSection; editorID?: string; session: ReturnType<typeof useAdminSession> }) {
  const navigate = useNavigate();
  const loadedEditorRef = useRef("");
  const { token, currentUser, setCurrentUser, logout: logoutSession, expireSession } = session;
  const [form] = Form.useForm<PostFormValues>();
  const [pageForm] = Form.useForm<PageFormValues>();
  const [settingsForm] = Form.useForm<SiteSettings>();
  const [userForm] = Form.useForm<{ email: string; username: string; display_name: string; password: string; role_codes: string[] }>();
  const [roleForm] = Form.useForm<{ code: string; name: string; description: string; permission_codes: string[] }>();
  const [roleEditForm] = Form.useForm<{ name: string; description: string }>();
  const [passwordForm] = Form.useForm<{ password: string }>();
  const [categoryForm] = Form.useForm<CategoryFormValues>();
  const [tagForm] = Form.useForm<TagFormValues>();
  const [seriesForm] = Form.useForm<SeriesFormValues>();
  const [achievementForm] = Form.useForm<AchievementFormValues>();
  const { health, ready, posts, pages, categories, tags, series, media, setMedia, comments, setComments, publishPreviews, publishJobs, publishReleases, auditLogs, adminUsers, systemRoles, permissions, siteSettings, setSiteSettings, siteSettingsHash, setSiteSettingsHash, postPagination, pagePagination, mediaPagination, commentPagination, previewPagination, jobPagination, releasePagination, userPagination, auditPagination, error, setError, refresh } = useAdminData(token, setCurrentUser, settingsForm, expireSession, section, editorID, section === "settings");
  const can = (permission: string) => Boolean(currentUser?.permissions.includes(permission));
  const achievementAdmin = useAchievementAdmin({
    token,
    editorID: section === "achievements" ? editorID : undefined,
    read: section === "achievements" && can("achievement:read") && (!editorID || (editorID === "new" ? can("achievement:create") : can("achievement:update"))),
    create: can("achievement:create"),
    update: can("achievement:update"),
    delete: can("achievement:delete"),
    publish: can("achievement:publish"),
    mediaRead: can("media:read"),
    onUnauthorized: expireSession,
  });
  const { taxonomyBusy, mediaBusy, mediaCleanupBusy, commentBusy, createCategory, deleteCategory, createTag, deleteTag, saveSeries, deleteSeries, uploadMediaFile, deleteMedia, runMediaCleanup, mediaURL, copyMediaURL, insertMediaMarkdown, moderateComment, deleteComment } = useContentAdminCommands({ token, canManageTaxonomy: can("taxonomy:manage"), canUploadMedia: can("media:upload"), canDeleteMedia: can("media:delete"), canInsertMediaMarkdown: can("post:update"), canModerateComments: can("comment:moderate"), refresh, setError, setMedia, setComments, categoryForm, tagForm, seriesForm, postForm: form });
  const { userBusy, roleBusy, passwordResetUser, setPasswordResetUser, closePasswordReset, editingRole, openRoleEditor, closeRoleEditor, createUser, updateUserStatus, updateUserRoles, createRole, updateRolePermissions, updateRole, deleteRole, resetUserPassword } = useIdentityCommands({ token, canManageUsers: can("user:manage"), canManageRoles: can("role:manage"), refresh, setError, userForm, roleForm, roleEditForm, passwordForm });
  const { releaseBusy, releaseCleanupBusy, previewCleanupBusy, promoteRelease, runReleaseCleanup, runPreviewCleanup, retryPublishJob, cancelPublishJob } = usePublishingCommands({ token, refresh, setError });
  const { busy, deletingPostID, pageBusy, settingsBusy, previewBusy, qualityBusy, qualityReport, qualityVisible, qualityTarget, selectPost, newPost, saveDraft, publish, previewPostDraft, deletePost, selectPage, newPage, savePage, publishPage, previewPageDraft, deletePage, checkPostQuality, checkPageQuality, retryQualityCheck, invalidateQualityReport, closeQualityPanel, saveSettings, publishSettings, previewSettings, openPreviewURL, resetEditors } = useEditorialCommands({ token, canUpdateSettings: can("setting:update"), refresh, setError, postForm: form, pageForm, settingsForm, setSiteSettings, setSiteSettingsHash });

  useEffect(() => {
    if (!editorID || (section !== "posts" && section !== "pages")) {
      loadedEditorRef.current = "";
      resetEditors();
      return;
    }

    const editorKey = `${section}:${editorID}`;
    if (loadedEditorRef.current === editorKey) return;

    if (section === "posts") {
      if (editorID === "new") {
        newPost();
        loadedEditorRef.current = editorKey;
        return;
      }
      const post = posts.find((item) => item.id === editorID);
      if (post) {
        selectPost(post);
        loadedEditorRef.current = editorKey;
      }
      return;
    }

    if (editorID === "new") {
      newPage();
      loadedEditorRef.current = editorKey;
      return;
    }
    const page = pages.find((item) => item.id === editorID);
    if (page) {
      selectPage(page);
      loadedEditorRef.current = editorKey;
    }
  }, [editorID, pages, posts, section]);

  function logout() {
    void logoutSession();
  }

  const navItems = useMemo(() => {
    const definitions: Array<AdminNavItem & { permission: string }> = [
      { key: "/dashboard", icon: <IconDashboard />, label: "工作台", group: "工作区", permission: "" },
      { key: "/posts", icon: <IconFile />, label: "文章", group: "工作区", permission: "post:read" },
      { key: "/pages", icon: <IconBook />, label: "页面", group: "工作区", permission: "page:read" },
      { key: "/achievements", icon: <IconStar />, label: "成果", group: "工作区", permission: "achievement:read" },
      { key: "/media", icon: <IconImage />, label: "媒体", group: "工作区", permission: "media:read" },
      { key: "/taxonomy", icon: <IconTag />, label: "内容组织", group: "管理", permission: "taxonomy:read" },
      { key: "/comments", icon: <IconMessage />, label: "评论", group: "管理", permission: "comment:read" },
      { key: "/publishing", icon: <IconSend />, label: "发布", group: "管理", permission: "publish:read" },
      { key: "/users", icon: <IconSafe />, label: "账号权限", group: "管理", permission: "user:read" },
      { key: "/settings", icon: <IconSettings />, label: "设置", group: "管理", permission: "setting:read" },
      { key: "/audit", icon: <IconStorage />, label: "审计", group: "管理", permission: "audit:read" }
    ];
    return definitions.filter((item) => !item.permission || Boolean(currentUser?.permissions.includes(item.permission)));
  }, [currentUser]);

  const sectionPermission: Partial<Record<AdminSection, string>> = {
    posts: "post:read", pages: "page:read", achievements: "achievement:read", taxonomy: "taxonomy:read", media: "media:read", comments: "comment:read",
    publishing: "publish:read", users: "user:read", settings: "setting:read", audit: "audit:read"
  };
  const requiredSectionPermission = sectionPermission[section];
  const identityReady = Boolean(currentUser);
  const sectionAllowed = identityReady && (!requiredSectionPermission || can(requiredSectionPermission));
  const editorRouteAllowed = !editorID || (
    section === "posts"
      ? editorID === "new" ? can("post:create") : can("post:update")
      : section === "pages"
        ? editorID === "new" ? can("page:create") : can("page:update")
        : section === "achievements"
          ? editorID === "new" ? can("achievement:create") : can("achievement:update")
          : true
  );
  const routeAllowed = sectionAllowed && editorRouteAllowed;
  const postEditorCanSave = editorID === "new" ? can("post:create") : can("post:update");
  const pageEditorCanSave = editorID === "new" ? can("page:create") : can("page:update");

  const categoryOptions = useMemo(
    () => categories.map((category) => ({ label: category.name, value: category.id })),
    [categories]
  );

  const tagOptions = useMemo(
    () => tags.map((tag) => ({ label: tag.name, value: tag.id })),
    [tags]
  );
  const activePost = section === "posts" && editorID && editorID !== "new" ? posts.find((post) => post.id === editorID) : undefined;
  const seriesOptions = useMemo(
    () => {
      const currentSeries = activePost?.series;
      const availableSeries = currentSeries && !series.some((item) => item.id === currentSeries.id) ? [...series, currentSeries] : series;
      return availableSeries.map((item) => ({ label: item.enabled ? item.name : `${item.name}（已停用）`, value: item.id, disabled: !item.enabled }));
    },
    [activePost?.series, series]
  );
  const activePage = section === "pages" && editorID && editorID !== "new" ? pages.find((page) => page.id === editorID) : undefined;

  return (
    <AdminLayout section={section} navItems={navItems} currentUser={currentUser} loggedIn={Boolean(token)} onLogout={logout} onRefresh={() => void refresh()}>
          {error && <Alert type="error" showIcon title="请求失败" content={error} closable onClose={() => setError("")} />}
          {!identityReady && <div style={{ display: "grid", minHeight: 240, placeItems: "center" }}><Spin size={32} /></div>}
          {identityReady && !routeAllowed && <Alert type="warning" showIcon title="无权访问" content="当前账号没有访问此模块所需的权限。" />}

          {identityReady && section === "dashboard" && <DashboardPage health={health} ready={ready} postCount={postPagination.total} pageCount={pagePagination.total} />}

          {routeAllowed && section === "posts" && <PostsPage
            posts={posts}
            pagination={postPagination}
            media={media}
            mediaURL={mediaURL}
             form={form}
             enabled={Boolean(token)}
             canCreate={can("post:create")}
             canUpdate={can("post:update")}
             canDelete={can("post:delete")}
             canSave={postEditorCanSave}
             canPreview={can("post:update")}
             canPublish={can("post:publish") && postEditorCanSave}
             writeBlocked={activePost?.status === "published" && !can("post:publish")}
            busy={busy}
            deletingPostID={deletingPostID}
            previewBusy={previewBusy === "post"}
            qualityBusy={qualityBusy && qualityTarget === "post"}
            categoryOptions={categoryOptions}
            tagOptions={tagOptions}
            seriesOptions={seriesOptions}
            mode={editorID ? "editor" : "list"}
            onNew={() => navigate("/posts/new")}
            onSelect={(post) => navigate(`/posts/${post.id}/edit`)}
            onBack={() => navigate("/posts")}
            onDelete={deletePost}
            onSave={(values) => void saveDraft(values).then((saved) => {
              if (saved && editorID === "new") navigate(`/posts/${saved.id}/edit`, { replace: true });
            })}
            onPreview={() => void previewPostDraft().then((saved) => {
              if (saved && editorID === "new") navigate(`/posts/${saved.id}/edit`, { replace: true });
            })}
            onPublish={() => void publish().then((saved) => {
              if (saved && editorID === "new") navigate(`/posts/${saved.id}/edit`, { replace: true });
            })}
            onQualityCheck={() => void checkPostQuality()}
            onFormChange={invalidateQualityReport}
          />}
          {routeAllowed && section === "pages" && <PagesPage
            pages={pages}
            pagination={pagePagination}
             form={pageForm}
             enabled={Boolean(token)}
             canCreate={can("page:create")}
             canUpdate={can("page:update")}
             canDelete={can("page:delete")}
             canSave={pageEditorCanSave}
             canPreview={can("page:update")}
             canPublish={can("page:publish") && pageEditorCanSave}
             writeBlocked={activePage?.status === "published" && !can("page:publish")}
            busy={pageBusy}
            previewBusy={previewBusy === "page"}
            qualityBusy={qualityBusy && qualityTarget === "page"}
            mode={editorID ? "editor" : "list"}
            onNew={() => navigate("/pages/new")}
            onSelect={(page) => navigate(`/pages/${page.id}/edit`)}
            onBack={() => navigate("/pages")}
            onDelete={(id) => void deletePage(id)}
            onSave={(values) => void savePage(values).then((saved) => {
              if (saved && editorID === "new") navigate(`/pages/${saved.id}/edit`, { replace: true });
            })}
            onPreview={() => void previewPageDraft().then((saved) => {
              if (saved && editorID === "new") navigate(`/pages/${saved.id}/edit`, { replace: true });
            })}
            onPublish={() => void publishPage().then((saved) => {
              if (saved && editorID === "new") navigate(`/pages/${saved.id}/edit`, { replace: true });
            })}
            onQualityCheck={() => void checkPageQuality()}
            onFormChange={invalidateQualityReport}
          />}
          {routeAllowed && section === "achievements" && <AchievementsPage
            achievements={achievementAdmin.achievements}
            achievement={achievementAdmin.achievement}
            pagination={achievementAdmin.pagination}
            form={achievementForm}
            enabled={Boolean(token)}
            canRead={can("achievement:read")}
            canCreate={can("achievement:create")}
            canUpdate={can("achievement:update")}
            canDelete={can("achievement:delete")}
            canPublish={can("achievement:publish")}
            canReadMedia={can("media:read")}
            mode={editorID ? "editor" : "list"}
            editorID={editorID}
            loading={achievementAdmin.loading}
            busy={achievementAdmin.busy}
            deletingID={achievementAdmin.deletingID}
            publishing={achievementAdmin.publishing}
            query={achievementAdmin.query}
            mediaURL={achievementAdmin.mediaURL}
            searchMedia={achievementAdmin.searchMedia}
            onQueryChange={achievementAdmin.updateQuery}
            onNew={() => navigate("/achievements/new")}
            onSelect={(item) => navigate(`/achievements/${item.id}/edit`)}
            onBack={() => navigate("/achievements")}
            onDelete={(id) => void achievementAdmin.remove(id)}
            onSave={(values) => void achievementAdmin.save(values, editorID === "new" ? undefined : editorID).then((saved) => {
              if (saved && editorID === "new") navigate(`/achievements/${saved.id}/edit`, { replace: true });
            })}
            onStatusChange={(id, status) => void achievementAdmin.updateStatus(id, status)}
            onPublish={() => void achievementAdmin.publish()}
          />}
          {sectionAllowed && section === "taxonomy" && <TaxonomyPage
            categories={categories}
            tags={tags}
            series={series}
            media={media}
            mediaURL={mediaURL}
            categoryForm={categoryForm}
            tagForm={tagForm}
            seriesForm={seriesForm}
            busy={taxonomyBusy}
            canManage={can("taxonomy:manage")}
            onCreateCategory={(values) => void createCategory(values)}
            onDeleteCategory={(id) => void deleteCategory(id)}
            onCreateTag={(values) => void createTag(values)}
            onDeleteTag={(id) => void deleteTag(id)}
            onSaveSeries={(values, id) => saveSeries(values, id)}
            onDeleteSeries={(id) => void deleteSeries(id)}
          />}
          {sectionAllowed && section === "media" && <MediaPage
            media={media}
            pagination={mediaPagination}
            canUpload={can("media:upload")}
            canDelete={can("media:delete")}
            canInsertMarkdown={can("post:update")}
            busy={mediaBusy}
            cleanupBusy={mediaCleanupBusy}
            mediaURL={mediaURL}
            onUpload={(options) => void uploadMediaFile(options)}
            onCleanup={(dryRun) => void runMediaCleanup(dryRun)}
            onCopyURL={(asset) => void copyMediaURL(asset)}
            onInsertMarkdown={insertMediaMarkdown}
            onDelete={(id) => void deleteMedia(id)}
          />}
          {sectionAllowed && section === "comments" && <CommentsPage
            comments={comments}
            pagination={commentPagination}
            canModerate={can("comment:moderate")}
            busy={commentBusy}
            onModerate={(id, status) => void moderateComment(id, status)}
            onDelete={(id) => void deleteComment(id)}
          />}
          {sectionAllowed && section === "settings" && <SettingsPage
            form={settingsForm}
            settings={siteSettings}
            settingsHash={siteSettingsHash}
            canUpdate={can("setting:update")}
            busy={settingsBusy}
            previewBusy={previewBusy === "site"}
            onSave={(values) => void saveSettings(values)}
            onPreview={() => void previewSettings()}
            onPublish={() => void publishSettings()}
          />}
          {sectionAllowed && section === "publishing" && <PublishingPage
            jobs={publishJobs}
            releases={publishReleases}
            previews={publishPreviews}
            jobPagination={jobPagination}
            releasePagination={releasePagination}
            previewPagination={previewPagination}
            canManageJobs={can("publish:create")}
            canPromote={can("publish:rollback")}
            canCleanup={can("publish:cleanup")}
            releaseBusy={releaseBusy}
            releaseCleanupBusy={releaseCleanupBusy}
            previewCleanupBusy={previewCleanupBusy}
            onRetryJob={(id) => void retryPublishJob(id)}
            onCancelJob={(id) => void cancelPublishJob(id)}
            onPromoteRelease={(id) => void promoteRelease(id)}
            onReleaseCleanup={(dryRun) => void runReleaseCleanup(dryRun)}
            onPreviewCleanup={(dryRun) => void runPreviewCleanup(dryRun)}
            onOpenPreview={openPreviewURL}
          />}
          {section === "users" && can("user:read") && <UserManagementPage
            users={adminUsers}
            userPagination={userPagination}
            roles={systemRoles}
            permissions={permissions}
            userForm={userForm}
            roleForm={roleForm}
            userBusy={userBusy}
            roleBusy={roleBusy}
            canManageUsers={can("user:manage")}
            canReadRoles={can("role:read")}
            canManageRoles={can("role:manage")}
            onCreateUser={(values) => void createUser(values)}
            onUpdateUserStatus={(userID, status) => void updateUserStatus(userID, status)}
            onUpdateUserRoles={(userID, roleCodes) => void updateUserRoles(userID, roleCodes)}
            onResetPassword={setPasswordResetUser}
            onCreateRole={(values) => void createRole(values)}
            onEditRole={openRoleEditor}
            onUpdateRolePermissions={(roleID, permissionCodes) => void updateRolePermissions(roleID, permissionCodes)}
            onDeleteRole={(roleID) => void deleteRole(roleID)}
          />}


          <Modal title={`重置密码：${passwordResetUser?.display_name || passwordResetUser?.username || ""}`} visible={Boolean(passwordResetUser)} onCancel={closePasswordReset} onOk={() => passwordForm.submit()} confirmLoading={userBusy} okText="确认重置" cancelText="取消" unmountOnExit>
            <Form form={passwordForm} layout="vertical" onSubmit={(values) => void resetUserPassword(values)}>
              <Form.Item field="password" label="新密码" extra="至少 10 个字符；重置后会撤销该用户现有刷新令牌。" rules={[{ required: true, minLength: 10, maxLength: 72 }]}>
                <Input.Password autoComplete="new-password" />
              </Form.Item>
            </Form>
          </Modal>

          <Modal title={`编辑角色：${editingRole?.code || ""}`} visible={Boolean(editingRole)} onCancel={closeRoleEditor} onOk={() => roleEditForm.submit()} confirmLoading={roleBusy} okText="保存" cancelText="取消" unmountOnExit>
            <Form form={roleEditForm} layout="vertical" onSubmit={(values) => void updateRole(values)}>
              <Form.Item field="name" label="角色名称" rules={[{ required: true, maxLength: 120 }]}><Input /></Form.Item>
              <Form.Item field="description" label="说明" rules={[{ maxLength: 500 }]}><Input.TextArea rows={4} /></Form.Item>
            </Form>
          </Modal>

          {section === "audit" && can("audit:read") && <AuditPage logs={auditLogs} pagination={auditPagination} />}
          <ContentQualityPanel
            visible={qualityVisible}
            loading={qualityBusy}
            report={qualityReport}
            targetLabel={String(qualityTarget === "post" ? form.getFieldValue("title") || "当前文章" : pageForm.getFieldValue("title") || "当前页面")}
            onClose={closeQualityPanel}
            onRetry={() => void retryQualityCheck()}
          />
    </AdminLayout>
  );
}
