import { useCallback, useEffect, useState } from "react";
import { Message } from "@arco-design/web-react";
import { ApiError, apiFetch } from "../api/client";
import type { ApiEnvelope, CurrentUser, LoginResponse, SessionResumeResponse } from "../types/admin";

const sessionMarkerKey = "zoking_admin_session";
const csrfTokenKey = "zoking_admin_csrf";

export function useAdminSession() {
  const [token, setToken] = useState(() => sessionStorage.getItem(sessionMarkerKey) === "1" ? sessionStorage.getItem(csrfTokenKey) || "" : "");
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null);
  const [loginBusy, setLoginBusy] = useState(false);
  const [initializing, setInitializing] = useState(true);

  const persistSession = useCallback((csrfToken: string) => {
    setToken(csrfToken);
    sessionStorage.setItem(sessionMarkerKey, "1");
    sessionStorage.setItem(csrfTokenKey, csrfToken);
  }, []);

  const expireSession = useCallback(() => {
    setToken("");
    sessionStorage.removeItem(sessionMarkerKey);
    sessionStorage.removeItem(csrfTokenKey);
    setCurrentUser(null);
  }, []);

  useEffect(() => {
    const storedToken = sessionStorage.getItem(sessionMarkerKey) === "1" ? sessionStorage.getItem(csrfTokenKey) || "" : "";
    if (storedToken) {
      setInitializing(false);
      return;
    }
    void apiFetch<ApiEnvelope<SessionResumeResponse>>("/api/v1/admin/auth/session", { method: "POST" })
      .then((result) => persistSession(result.data.csrf_token))
      .catch(() => expireSession())
      .finally(() => setInitializing(false));
  }, [expireSession, persistSession]);

  const login = useCallback(async (values: { email: string; password: string }) => {
    setLoginBusy(true);
    try {
      const result = await apiFetch<ApiEnvelope<LoginResponse>>("/api/v1/admin/auth/login", { method: "POST", body: JSON.stringify(values) });
      persistSession(result.data.csrf_token);
      Message.success("登录成功");
    } finally {
      setLoginBusy(false);
    }
  }, [persistSession]);

  const logout = useCallback(async () => {
    try {
      await apiFetch("/api/v1/admin/auth/logout", { method: "POST" }, token);
      expireSession();
      Message.success("已退出登录");
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        expireSession();
        return;
      }
      Message.error("退出失败，请检查网络后重试");
    }
  }, [expireSession, token]);

  return { token, currentUser, setCurrentUser, initializing, loginBusy, login, logout, expireSession };
}
