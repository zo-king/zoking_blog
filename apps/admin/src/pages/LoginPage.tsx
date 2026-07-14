import { Alert, Button, Form, Input } from "@arco-design/web-react";
import { IconEmail, IconLock, IconSafe } from "@arco-design/web-react/icon";
import { useState } from "react";

type LoginValues = {
  email: string;
  password: string;
};

type Props = {
  busy: boolean;
  onLogin: (values: LoginValues) => Promise<void>;
};

export function LoginPage({ busy, onLogin }: Props) {
  const [error, setError] = useState("");

  async function handleLogin(values: LoginValues) {
    setError("");
    try {
      await onLogin(values);
    } catch {
      setError("登录失败，请检查邮箱和密码后重试。");
    }
  }

  return (
    <main className="login-shell">
      <section className="login-context" aria-label="Zoking 内容管理后台">
        <div className="login-brand">
          <img src="/favicon.png" alt="" className="login-brand-mark" />
          <div>
            <strong>Zoking Admin</strong>
            <span>内容管理系统</span>
          </div>
        </div>

        <div className="login-context-copy">
          <p className="login-kicker">管理控制台</p>
          <h1>Zoking 内容管理后台</h1>
          <p>集中管理站点内容、发布流程与系统配置。</p>
        </div>

        <div className="login-access-note">
          <IconSafe aria-hidden="true" />
          <span>仅限授权人员访问</span>
        </div>
      </section>

      <section className="login-form-pane">
        <div className="login-form-container">
          <div className="login-form-heading">
            <p>安全登录</p>
            <h2>管理员登录</h2>
            <span>请输入管理账号凭据以继续。</span>
          </div>

          {error && (
            <Alert
              className="login-alert"
              type="error"
              showIcon
              title="登录失败"
              content={error}
              closable
              onClose={() => setError("")}
            />
          )}

          <Form<LoginValues>
            className="login-form"
            layout="vertical"
            requiredSymbol={false}
            initialValues={{ email: "admin@zoking.local", password: "ChangeMe123!" }}
            onSubmit={(values) => void handleLogin(values)}
          >
            <Form.Item
              field="email"
              label="邮箱"
              rules={[
                { required: true, message: "请输入邮箱" },
                { type: "email", message: "请输入有效的邮箱地址" }
              ]}
            >
              <Input
                prefix={<IconEmail aria-hidden="true" />}
                placeholder="admin@example.com"
                autoComplete="username"
                inputMode="email"
                size="large"
              />
            </Form.Item>

            <Form.Item
              field="password"
              label="密码"
              rules={[{ required: true, message: "请输入密码" }]}
            >
              <Input.Password
                prefix={<IconLock aria-hidden="true" />}
                placeholder="请输入密码"
                autoComplete="current-password"
                size="large"
              />
            </Form.Item>

            <Button
              className="login-submit"
              type="primary"
              htmlType="submit"
              size="large"
              loading={busy}
              icon={<IconSafe />}
              long
            >
              登录后台
            </Button>
          </Form>
        </div>
      </section>
    </main>
  );
}
