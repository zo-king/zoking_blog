import {
  Button,
  Empty,
  Image,
  Input,
  Modal,
  Pagination,
  Spin,
  Tag,
  Typography
} from "@arco-design/web-react";
import { IconClose, IconImage, IconSearch } from "@arco-design/web-react/icon";
import { useEffect, useState } from "react";
import type { MediaAsset, PaginationMeta } from "../types/admin";

const { Text } = Typography;

type Props = {
  value?: string;
  selected?: MediaAsset | null;
  disabled?: boolean;
  canRead?: boolean;
  mediaURL: (asset: MediaAsset) => string;
  search: (query: string, page: number, pageSize?: number) => Promise<{ data: MediaAsset[]; pagination: PaginationMeta }>;
  onChange: (value?: string, asset?: MediaAsset | null) => void;
};

function mediaName(asset: MediaAsset) { return asset.original_name || asset.filename || "未命名图片"; }

export function AchievementMediaPicker({ value, selected, disabled, canRead = false, mediaURL, search, onChange }: Props) {
  const [visible, setVisible] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [assets, setAssets] = useState<MediaAsset[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>({ page: 1, page_size: 12, total: 0, total_pages: 0 });
  const [loading, setLoading] = useState(false);
  const [currentAsset, setCurrentAsset] = useState<MediaAsset | null>(selected || null);

  async function load(page = 1, query = keyword) {
    setLoading(true);
    try {
      const result = await search(query, page, 12);
      setAssets(result.data);
      setPagination(result.pagination);
    } finally { setLoading(false); }
  }

  useEffect(() => {
    if (visible) void load();
  }, [visible]);

  useEffect(() => { setCurrentAsset(selected || null); }, [selected]);

  const clear = () => { setCurrentAsset(null); onChange(undefined, null); };

  return (
    <div className="achievement-media-picker">
      {currentAsset ? (
        <div className="achievement-media-current">
          <Image width={68} height={52} src={mediaURL(currentAsset)} alt={mediaName(currentAsset)} preview />
          <div className="achievement-media-current-copy">
            <Text ellipsis={{ showTooltip: true }}>{mediaName(currentAsset)}</Text>
            <Text type="secondary">已选择成果配图</Text>
          </div>
          <Button type="text" size="mini" icon={<IconClose />} aria-label="移除成果配图" onClick={clear} disabled={disabled} />
        </div>
      ) : (
        <div className="achievement-media-empty">
          <IconImage />
          <Text type="secondary">尚未选择配图</Text>
        </div>
      )}
      <Button
        long
        icon={<IconSearch />}
        disabled={disabled || !canRead}
        onClick={() => setVisible(true)}
      >
        {currentAsset ? "更换配图" : "从媒体库选择"}
      </Button>

      <Modal
        title="选择成果配图"
        visible={visible}
        unmountOnExit
        footer={null}
        onCancel={() => setVisible(false)}
        className="achievement-media-modal"
      >
        <div className="achievement-media-search">
          <Input.Search
            allowClear
            value={keyword}
            placeholder="搜索文件名"
            onChange={setKeyword}
            onSearch={(next) => void load(1, next)}
          />
        </div>
        <Spin loading={loading}>
          {assets.length ? (
            <div className="achievement-media-grid">
              {assets.map((asset) => (
                <button
                  type="button"
                  className={`achievement-media-option${asset.id === value ? " is-selected" : ""}`}
                  key={asset.id}
                  onClick={() => { setCurrentAsset(asset); onChange(asset.id, asset); setVisible(false); }}
                >
                  <Image width="100%" height={92} src={mediaURL(asset)} alt={mediaName(asset)} preview={false} />
                  <span>{mediaName(asset)}</span>
                  {asset.id === value ? <Tag color="green">当前</Tag> : null}
                </button>
              ))}
            </div>
          ) : <Empty description={keyword ? "没有找到匹配的图片" : "媒体库中暂无可用图片"} />}
        </Spin>
        {pagination.total > pagination.page_size ? (
          <Pagination
            className="achievement-media-pagination"
            size="small"
            current={pagination.page}
            pageSize={pagination.page_size}
            total={pagination.total}
            hideOnSinglePage
            onChange={(page) => void load(page)}
          />
        ) : null}
      </Modal>
    </div>
  );
}
