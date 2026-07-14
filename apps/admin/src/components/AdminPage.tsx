import { Typography } from "@arco-design/web-react";
import type { ReactNode } from "react";

const { Title, Text } = Typography;

type PageHeaderProps = {
  title: string;
  description: string;
  eyebrow?: string;
  actions?: ReactNode;
};

export function PageHeader({ title, description, eyebrow, actions }: PageHeaderProps) {
  return (
    <header className="page-heading">
      <div className="page-heading-copy">
        {eyebrow ? <span className="page-eyebrow">{eyebrow}</span> : null}
        <Title heading={3}>{title}</Title>
        <Text type="secondary">{description}</Text>
      </div>
      {actions ? <div className="page-heading-actions">{actions}</div> : null}
    </header>
  );
}

type ContentPanelProps = {
  title?: string;
  description?: string;
  actions?: ReactNode;
  className?: string;
  children: ReactNode;
};

export function ContentPanel({ title, description, actions, className = "", children }: ContentPanelProps) {
  return (
    <section className={`content-panel ${className}`.trim()}>
      {title || description || actions ? (
        <header className="content-panel-header">
          <div>
            {title ? <Title heading={6}>{title}</Title> : null}
            {description ? <Text type="secondary">{description}</Text> : null}
          </div>
          {actions ? <div className="content-panel-actions">{actions}</div> : null}
        </header>
      ) : null}
      <div className="content-panel-body">{children}</div>
    </section>
  );
}
