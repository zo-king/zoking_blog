import { Avatar, Button, Drawer, Layout, Menu, Tooltip, Typography } from "@arco-design/web-react";
import { IconBook, IconLaunch, IconMenu, IconPoweroff, IconRefresh } from "@arco-design/web-react/icon";
import { useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { roleLabel } from "../labels";
import type { AdminSection, CurrentUser } from "../types/admin";

const { Header, Sider, Content } = Layout;
const { Text } = Typography;
const siteUrl = window.__ZOKING_ADMIN_CONFIG__?.siteBaseUrl?.trim()
  || import.meta.env.VITE_SITE_URL
  || (import.meta.env.DEV ? "http://localhost:1313" : "https://zoking.tech");

export type AdminNavItem = {
  key: string;
  icon: ReactNode;
  label: string;
  group: "工作区" | "管理";
};

type Props = {
  section: AdminSection;
  navItems: AdminNavItem[];
  currentUser: CurrentUser | null;
  loggedIn: boolean;
  children: ReactNode;
  onLogout: () => void;
  onRefresh: () => void;
};

export function AdminLayout(props: Props) {
  const navigate = useNavigate();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const selectedKey = `/${props.section}`;
  const navigateTo = (key: string) => { navigate(key); setMobileNavOpen(false); };
  const groups = ["工作区", "管理"] as const;
  const userInitial = props.currentUser?.email.trim().charAt(0).toUpperCase() || "Z";
  const roleText = props.currentUser?.roles.map((role) => roleLabel(role)).join("、") || "未分配角色";

  const navigation = (
    <nav className="admin-navigation" aria-label="后台主导航">
      {groups.map((group) => {
        const items = props.navItems.filter((item) => item.group === group);
        if (!items.length) return null;
        return (
          <div className="nav-section" key={group}>
            <div className="nav-section-label">{group}</div>
            <Menu selectedKeys={[selectedKey]} onClickMenuItem={navigateTo}>
              {items.map((item) => (
                <Menu.Item key={item.key}>
                  {item.icon}
                  <span>{item.label}</span>
                </Menu.Item>
              ))}
            </Menu>
          </div>
        );
      })}
    </nav>
  );

  return <Layout className="app-shell">
    <Sider width={216} className="app-sider">
      <div className="brand"><span className="brand-mark"><IconBook /></span><span><strong>Zoking</strong><small>内容管理平台</small></span></div>
      {navigation}
      <div className="sidebar-account">
        <Avatar size={34}>{userInitial}</Avatar>
        <div><strong>{props.currentUser?.email || "管理员"}</strong><span>{roleText}</span></div>
      </div>
    </Sider>
    <Drawer title="后台导航" placement="left" width={288} visible={mobileNavOpen} footer={null} onCancel={() => setMobileNavOpen(false)} className="mobile-nav-drawer">
      {navigation}
    </Drawer>
    <Layout>
      <Header className="app-header">
        <div className="header-title-group">
          <Button className="mobile-nav-trigger" type="text" icon={<IconMenu />} onClick={() => setMobileNavOpen(true)} aria-label="打开导航" />
          <Text className="header-workspace-name">个人博客</Text>
        </div>
        <div className="header-actions">
          <Tooltip content="查看博客"><Button type="text" icon={<IconLaunch />} aria-label="查看博客" onClick={() => window.open(siteUrl, "_blank", "noopener,noreferrer")} /></Tooltip>
          <Tooltip content="刷新数据"><Button type="text" icon={<IconRefresh />} aria-label="刷新数据" onClick={props.onRefresh} /></Tooltip>
          {props.loggedIn ? <Tooltip content="退出登录"><Button type="text" icon={<IconPoweroff />} aria-label="退出登录" onClick={props.onLogout} /></Tooltip> : null}
        </div>
      </Header>
      <Content className="app-content"><div className="content-frame">{props.children}</div></Content>
    </Layout>
  </Layout>;
}
