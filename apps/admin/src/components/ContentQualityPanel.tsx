import { Alert, Button, Drawer, Empty, Progress, Space, Tag, Typography } from "@arco-design/web-react";
import { IconCheckCircle, IconExclamationCircle, IconRefresh } from "@arco-design/web-react/icon";
import type { ContentQualityIssue, ContentQualityReport } from "../types/admin";

const { Text, Title } = Typography;

type Props = {
  visible: boolean;
  loading: boolean;
  report: ContentQualityReport | null;
  targetLabel: string;
  onClose: () => void;
  onRetry: () => void;
};

function IssueGroup({ title, issues, severity }: { title: string; issues: ContentQualityIssue[]; severity: "error" | "warning" }) {
  if (!issues.length) return null;
  return (
    <section className="quality-issue-group">
      <div className="quality-issue-heading">
        <Text bold>{title}</Text>
        <Tag color={severity === "error" ? "red" : "orange"}>{issues.length}</Tag>
      </div>
      <div className="quality-issue-list">
        {issues.map((issue, index) => (
          <div className={`quality-issue quality-issue-${severity}`} key={`${issue.code}-${issue.field}-${index}`}>
            <IconExclamationCircle />
            <div>
              <Text>{issue.message}</Text>
              <Space size={6} wrap>
                {issue.field ? <Tag>{issue.field}</Tag> : null}
                <Text type="secondary">{issue.code}</Text>
              </Space>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

export function ContentQualityPanel({ visible, loading, report, targetLabel, onClose, onRetry }: Props) {
  const errors = report?.issues.filter((issue) => issue.severity === "error") || [];
  const warnings = report?.issues.filter((issue) => issue.severity === "warning") || [];
  const status = report?.status || "passed";
  const statusMeta = status === "blocked"
    ? { label: "暂不可发布", color: "red", message: "请先修复阻断项，再重新检查。" }
    : status === "warning"
      ? { label: "可以发布", color: "orange", message: "检查已通过，建议发布前处理提示项。" }
      : { label: "检查通过", color: "green", message: "当前内容符合发布要求。" };

  return (
    <Drawer
      className="content-quality-drawer"
      title="发布检查"
      placement="right"
      width={380}
      visible={visible}
      unmountOnExit={false}
      footer={
        <div className="quality-drawer-footer">
          <Button onClick={onClose}>关闭</Button>
          <Button type="primary" icon={<IconRefresh />} loading={loading} onClick={onRetry}>重新检查</Button>
        </div>
      }
      onCancel={onClose}
    >
      {report ? (
        <div className="quality-panel-content">
          <Text type="secondary" ellipsis>{targetLabel || "当前内容"}</Text>
          <div className="quality-score-summary">
            <Progress
              type="circle"
              size="small"
              percent={Math.max(0, Math.min(100, report.score))}
              color={status === "blocked" ? "rgb(var(--red-6))" : status === "warning" ? "rgb(var(--orange-6))" : "rgb(var(--green-6))"}
              formatText={() => `${report.score}`}
            />
            <div>
              <Space size={8}>
                <Title heading={6}>{statusMeta.label}</Title>
                <Tag color={statusMeta.color}>{report.ready ? "已就绪" : "需修复"}</Tag>
              </Space>
              <Text type="secondary">{statusMeta.message}</Text>
            </div>
          </div>

          {report.status === "passed" ? (
            <Alert type="success" showIcon icon={<IconCheckCircle />} content="未发现阻断项或优化建议。" />
          ) : null}

          <div className="quality-counts" aria-label="检查结果统计">
            <div><strong>{report.error_count}</strong><span>阻断项</span></div>
            <div><strong>{report.warning_count}</strong><span>提示项</span></div>
            <div><strong>{report.issues.length}</strong><span>检查发现</span></div>
          </div>

          <IssueGroup title="必须修复" issues={errors} severity="error" />
          <IssueGroup title="优化建议" issues={warnings} severity="warning" />
          {!report.issues.length && report.status !== "passed" ? <Empty description="没有可展示的检查项" /> : null}

          <div className="quality-report-meta">
            <Text type="secondary">规则版本 {report.policy_version || "-"}</Text>
            <Text type="secondary" ellipsis>内容指纹 {report.content_hash || "-"}</Text>
          </div>
        </div>
      ) : (
        <div className="quality-panel-empty">
          <Empty description={loading ? "正在检查内容..." : "尚无检查结果"} />
        </div>
      )}
    </Drawer>
  );
}
