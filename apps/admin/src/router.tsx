import { Navigate, Route, Routes, useParams } from "react-router-dom";
import { App } from "./App";
import type { AdminSection } from "./types/admin";

const sections: Array<{ path: string; section: AdminSection }> = [
  { path: "/dashboard", section: "dashboard" },
  { path: "/posts", section: "posts" },
  { path: "/pages", section: "pages" },
  { path: "/achievements", section: "achievements" },
  { path: "/taxonomy", section: "taxonomy" },
  { path: "/media", section: "media" },
  { path: "/comments", section: "comments" },
  { path: "/publishing", section: "publishing" },
  { path: "/users", section: "users" },
  { path: "/settings", section: "settings" },
  { path: "/audit", section: "audit" }
];

export function AdminRouter() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
      <Route path="/posts/new" element={<App section="posts" editorID="new" />} />
      <Route path="/posts/:postID/edit" element={<PostEditorRoute />} />
      <Route path="/pages/new" element={<App section="pages" editorID="new" />} />
      <Route path="/pages/:pageID/edit" element={<PageEditorRoute />} />
      <Route path="/achievements/new" element={<App section="achievements" editorID="new" />} />
      <Route path="/achievements/:achievementID/edit" element={<AchievementEditorRoute />} />
      {sections.map((route) => (
        <Route key={route.path} path={route.path} element={<App section={route.section} />} />
      ))}
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  );
}

function PostEditorRoute() {
  const { postID = "" } = useParams();
  return <App section="posts" editorID={postID} />;
}

function PageEditorRoute() {
  const { pageID = "" } = useParams();
  return <App section="pages" editorID={pageID} />;
}

function AchievementEditorRoute() {
  const { achievementID = "" } = useParams();
  return <App section="achievements" editorID={achievementID} />;
}
