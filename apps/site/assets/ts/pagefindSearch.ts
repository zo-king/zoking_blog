/*
 * Pagefind 搜索页脚本:替换主题的 JSON 全文子串匹配。
 * 索引由构建流水线在 Hugo 构建后用 pagefind CLI 生成(/pagefind/ 目录);
 * 开发服务器下索引不存在,会展示降级提示而不是报错。
 * UX 与旧版保持一致:?keyword= 查询串、输入即搜、相同的结果卡片标记。
 */

interface PagefindResultData {
    url: string;
    excerpt: string;
    meta: { title?: string; image?: string };
}

interface PagefindResult {
    id: string;
    score: number;
    data(): Promise<PagefindResultData>;
}

interface PagefindModule {
    init(): void;
    search(query: string, options?: object): Promise<{ results: PagefindResult[] }>;
    debouncedSearch(query: string, options?: object, debounceMs?: number): Promise<{ results: PagefindResult[] } | null>;
}

declare global {
    interface Window {
        searchResultTitleTemplate: string;
    }
}

const PAGEFIND_PATH = "/pagefind/pagefind.js";
const MAX_RESULTS = 30;

class PagefindSearch {
    private form: HTMLFormElement;
    private input: HTMLInputElement;
    private list: HTMLDivElement;
    private resultTitle: HTMLHeadingElement;
    private container: HTMLDivElement;
    private pagefind: PagefindModule | null = null;
    private loadFailed = false;
    private searchSeq = 0;

    constructor(form: HTMLFormElement, input: HTMLInputElement, list: HTMLDivElement, resultTitle: HTMLHeadingElement) {
        this.form = form;
        this.input = input;
        this.list = list;
        this.resultTitle = resultTitle;
        this.container = list.parentElement as HTMLDivElement;

        this.handleQueryString();
        window.addEventListener("popstate", () => this.handleQueryString());
        this.bindSearchForm();
    }

    private async getPagefind(): Promise<PagefindModule | null> {
        if (this.pagefind || this.loadFailed) return this.pagefind;
        try {
            // import() 的路径保持运行时解析(js.Build 已将其标记为 external),索引由构建流水线产出。
            this.pagefind = (await import(PAGEFIND_PATH)) as PagefindModule;
            this.pagefind.init();
        } catch {
            this.loadFailed = true;
        }
        return this.pagefind;
    }

    private bindSearchForm() {
        let lastSearch = "";
        const handler = (e: Event) => {
            e.preventDefault();
            const keyword = this.input.value.trim();
            this.updateQueryString(keyword);
            if (keyword === "") {
                lastSearch = "";
                this.clear();
                return;
            }
            if (lastSearch === keyword) return;
            lastSearch = keyword;
            void this.doSearch(keyword);
        };
        this.form.addEventListener("submit", handler);
        this.input.addEventListener("input", handler);
        this.input.addEventListener("compositionend", handler);
    }

    private handleQueryString() {
        const keyword = new URL(window.location.toString()).searchParams.get("keyword") || "";
        this.input.value = keyword;
        if (keyword) {
            void this.doSearch(keyword);
        } else {
            this.clear();
        }
    }

    private updateQueryString(keyword: string) {
        const pageURL = new URL(window.location.toString());
        if (keyword === "") {
            pageURL.searchParams.delete("keyword");
        } else {
            pageURL.searchParams.set("keyword", keyword);
        }
        window.history.replaceState("", "", pageURL.toString());
    }

    private async doSearch(keyword: string) {
        const startTime = performance.now();
        const seq = ++this.searchSeq;
        this.form.setAttribute("aria-busy", "true");
        try {
            const pagefind = await this.getPagefind();
            if (!pagefind) {
                this.showUnavailable();
                return;
            }
            const search = await pagefind.debouncedSearch(keyword, {}, 300);
            if (search === null) return; // 已被更新的输入取代

            let items: PagefindResultData[];
            if (search.results.length > 0) {
                items = await Promise.all(search.results.slice(0, MAX_RESULTS).map((r) => r.data()));
            } else {
                items = await PagefindSearch.cjkFallbackSearch(pagefind, keyword);
            }
            // 回退路径可能比后续查询慢,渲染前确认自己仍是最新一次搜索
            if (seq !== this.searchSeq) return;
            this.clear();
            for (const item of items) {
                this.list.append(PagefindSearch.render(item));
            }
            const seconds = ((performance.now() - startTime) / 1000).toPrecision(1);
            this.resultTitle.innerText = window.searchResultTitleTemplate
                .replace("#PAGES_COUNT", items.length.toString())
                .replace("#TIME_SECONDS", seconds);
            this.container.classList.remove("hidden");
        } finally {
            this.form.removeAttribute("aria-busy");
        }
    }

    /*
     * 中文回退检索:Pagefind 查询侧按单字 AND 匹配、索引侧按词典整词分词,
     * 两侧错位时整词查询(如「哈希表」)会漏掉包含该词的文章。
     * 主查询无结果且含 CJK 时,按单字逐个检索取并集,再用完整查询词
     * 是否出现在标题/摘要里重排,保证正主排最前。
     */
    private static async cjkFallbackSearch(pagefind: PagefindModule, keyword: string): Promise<PagefindResultData[]> {
        if (!/[一-鿿぀-ヿ]/.test(keyword)) return [];
        const chars = [...new Set([...keyword])].filter((c) => c.trim() !== "").slice(0, 8);

        const merged = new Map<string, { result: PagefindResult; hits: number }>();
        for (const char of chars) {
            const { results } = await pagefind.search(char);
            for (const result of results) {
                const entry = merged.get(result.id);
                if (entry) {
                    entry.hits += 1;
                } else {
                    merged.set(result.id, { result, hits: 1 });
                }
            }
        }

        const candidates = [...merged.values()]
            .sort((a, b) => b.hits - a.hits || b.result.score - a.result.score)
            .slice(0, MAX_RESULTS);
        const withData = await Promise.all(
            candidates.map(async (entry) => ({ ...entry, data: await entry.result.data() })),
        );
        const containsKeyword = (item: PagefindResultData) => {
            const excerptText = item.excerpt.replace(/<[^>]+>/g, "");
            return (item.meta.title || "").includes(keyword) || excerptText.includes(keyword) ? 1 : 0;
        };
        return withData
            .sort((a, b) =>
                containsKeyword(b.data) - containsKeyword(a.data) ||
                b.hits - a.hits ||
                b.result.score - a.result.score,
            )
            .map((entry) => entry.data);
    }

    private showUnavailable() {
        this.clear();
        this.resultTitle.innerText = "搜索暂时不可用";
        const error = document.createElement("div");
        error.className = "search-result__error";
        const message = document.createElement("p");
        message.textContent = "搜索索引尚未生成。开发模式下请先执行构建与 pagefind 索引后再试。";
        error.append(message);
        this.list.append(error);
        this.container.classList.remove("hidden");
    }

    private clear() {
        this.list.innerHTML = "";
        this.resultTitle.innerText = "";
        this.container.classList.add("hidden");
    }

    private static render(item: PagefindResultData): HTMLElement {
        const article = document.createElement("article");
        const link = document.createElement("a");
        link.href = item.url;

        const details = document.createElement("div");
        details.className = "article-details";

        const title = document.createElement("h3");
        title.className = "article-title";
        title.textContent = item.meta.title || item.url;

        const preview = document.createElement("section");
        preview.className = "article-preview";
        // excerpt 由 Pagefind 生成,仅含 <mark> 高亮标签
        preview.innerHTML = item.excerpt;

        details.append(title, preview);
        link.append(details);

        if (item.meta.image) {
            const imageWrapper = document.createElement("div");
            imageWrapper.className = "article-image";
            const img = document.createElement("img");
            img.src = item.meta.image;
            img.alt = item.meta.title || "文章封面";
            img.loading = "lazy";
            imageWrapper.append(img);
            link.append(imageWrapper);
        }

        article.append(link);
        return article;
    }
}

window.addEventListener("load", () => {
    const form = document.querySelector(".search-form") as HTMLFormElement;
    const input = form?.querySelector("input") as HTMLInputElement;
    const list = document.querySelector(".search-result--list") as HTMLDivElement;
    const resultTitle = document.querySelector(".search-result--title") as HTMLHeadingElement;
    if (!form || !input || !list || !resultTitle) return;
    new PagefindSearch(form, input, list, resultTitle);
});

export {};
