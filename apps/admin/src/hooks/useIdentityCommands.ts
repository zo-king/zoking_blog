import { useState } from "react";
import { Message, type FormInstance } from "@arco-design/web-react";
import { apiFetch } from "../api/client";
import type { AdminUser, ApiEnvelope, SystemRole } from "../types/admin";

type UserValues = { email: string; username: string; display_name: string; password: string; role_codes: string[] };
type RoleValues = { code: string; name: string; description: string; permission_codes: string[] };

export function useIdentityCommands({ token, canManageUsers, canManageRoles, refresh, setError, userForm, roleForm, roleEditForm, passwordForm }: {
  token: string; canManageUsers: boolean; canManageRoles: boolean; refresh: () => Promise<void>; setError: (value: string) => void;
  userForm: FormInstance<UserValues>; roleForm: FormInstance<RoleValues>; roleEditForm: FormInstance<{ name: string; description: string }>; passwordForm: FormInstance<{ password: string }>;
}) {
  const [userBusy, setUserBusy] = useState(false);
  const [roleBusy, setRoleBusy] = useState(false);
  const [passwordResetUser, setPasswordResetUser] = useState<AdminUser | null>(null);
  const [editingRole, setEditingRole] = useState<SystemRole | null>(null);

  async function createUser(values: UserValues) {
    if (!token || !canManageUsers) return; setUserBusy(true);
    try { await apiFetch<ApiEnvelope<AdminUser>>("/api/v1/admin/users", { method: "POST", body: JSON.stringify(values) }, token); userForm.resetFields(); Message.success("用户已创建"); await refresh(); }
    catch (err) { setError(String(err)); } finally { setUserBusy(false); }
  }
  async function updateUserStatus(id: string, status: string) {
    if (!token || !canManageUsers) return; setUserBusy(true);
    try { await apiFetch(`/api/v1/admin/users/${id}/status`, { method: "PATCH", body: JSON.stringify({ status }) }, token); Message.success(status === "active" ? "用户已启用" : "用户已停用"); await refresh(); }
    catch (err) { setError(String(err)); } finally { setUserBusy(false); }
  }
  async function updateUserRoles(id: string, roleCodes: string[]) {
    if (!token || !canManageUsers) return; setUserBusy(true);
    try { await apiFetch(`/api/v1/admin/users/${id}/roles`, { method: "PATCH", body: JSON.stringify({ role_codes: roleCodes }) }, token); Message.success("用户角色已更新"); await refresh(); }
    catch (err) { setError(String(err)); await refresh(); } finally { setUserBusy(false); }
  }
  async function createRole(values: RoleValues) {
    if (!token || !canManageRoles) return; setRoleBusy(true);
    try { await apiFetch("/api/v1/admin/roles", { method: "POST", body: JSON.stringify(values) }, token); roleForm.resetFields(); Message.success("角色已创建"); await refresh(); }
    catch (err) { setError(String(err)); } finally { setRoleBusy(false); }
  }
  async function updateRolePermissions(id: string, codes: string[]) {
    if (!token || !canManageRoles) return; setRoleBusy(true);
    try { await apiFetch(`/api/v1/admin/roles/${id}/permissions`, { method: "PATCH", body: JSON.stringify({ permission_codes: codes }) }, token); Message.success("角色权限已更新"); await refresh(); }
    catch (err) { setError(String(err)); await refresh(); } finally { setRoleBusy(false); }
  }
  function openRoleEditor(role: SystemRole) { setEditingRole(role); roleEditForm.setFieldsValue({ name: role.name, description: role.description }); }
  function closeRoleEditor() { setEditingRole(null); roleEditForm.resetFields(); }
  async function updateRole(values: { name: string; description: string }) {
    if (!token || !editingRole || editingRole.is_system || !canManageRoles) return; setRoleBusy(true);
    try { await apiFetch(`/api/v1/admin/roles/${editingRole.id}`, { method: "PATCH", body: JSON.stringify(values) }, token); Message.success("角色信息已更新"); closeRoleEditor(); await refresh(); }
    catch (err) { setError(String(err)); } finally { setRoleBusy(false); }
  }
  async function deleteRole(id: string) {
    if (!token || !canManageRoles) return; setRoleBusy(true);
    try { await apiFetch(`/api/v1/admin/roles/${id}`, { method: "DELETE" }, token); Message.success("角色已删除"); await refresh(); }
    catch (err) { setError(String(err)); } finally { setRoleBusy(false); }
  }
  function closePasswordReset() { setPasswordResetUser(null); passwordForm.resetFields(); }
  async function resetUserPassword(values: { password: string }) {
    if (!token || !passwordResetUser || !canManageUsers) return; setUserBusy(true);
    try { await apiFetch(`/api/v1/admin/users/${passwordResetUser.id}/reset-password`, { method: "POST", body: JSON.stringify(values) }, token); Message.success("密码已重置，原有登录会话已撤销"); closePasswordReset(); }
    catch (err) { setError(String(err)); } finally { setUserBusy(false); }
  }

  return { userBusy, roleBusy, passwordResetUser, setPasswordResetUser, closePasswordReset, editingRole, openRoleEditor, closeRoleEditor, createUser, updateUserStatus, updateUserRoles, createRole, updateRolePermissions, updateRole, deleteRole, resetUserPassword };
}
