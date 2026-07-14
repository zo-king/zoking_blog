import { useCallback } from "react";
import { useSearchParams } from "react-router-dom";

type ListQueryPatch = Partial<{
  page: number;
  pageSize: number;
  q: string;
  status: string;
  sort: string;
}>;

function positiveInt(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

export function useListQuery(defaultPageSize = 20) {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = positiveInt(searchParams.get("page"), 1);
  const pageSize = positiveInt(searchParams.get("page_size"), defaultPageSize);
  const q = searchParams.get("q") || "";
  const status = searchParams.get("status") || "";
  const sort = searchParams.get("sort") || "";

  const update = useCallback((patch: ListQueryPatch, replace = false) => {
    const next = new URLSearchParams(searchParams);
    const changesFilter = "q" in patch || "status" in patch || "sort" in patch || "pageSize" in patch;
    const nextPage = patch.page ?? (changesFilter ? 1 : page);

    if (nextPage <= 1) next.delete("page");
    else next.set("page", String(nextPage));

    if (patch.pageSize !== undefined) {
      if (patch.pageSize === defaultPageSize) next.delete("page_size");
      else next.set("page_size", String(patch.pageSize));
    }

    for (const [key, value] of [["q", patch.q], ["status", patch.status], ["sort", patch.sort]] as const) {
      if (value === undefined) continue;
      if (value) next.set(key, value);
      else next.delete(key);
    }

    setSearchParams(next, { replace });
  }, [defaultPageSize, page, searchParams, setSearchParams]);

  return { page, pageSize, q, status, sort, update };
}
