type HitokotoPayload = {
    hitokoto: string;
    from: string;
};

const root = document.querySelector<HTMLElement>("[data-hitokoto]");

if (root) {
    const text = root.querySelector<HTMLElement>("[data-hitokoto-text]");
    const source = root.querySelector<HTMLElement>("[data-hitokoto-source]");
    const refresh = root.querySelector<HTMLButtonElement>("[data-hitokoto-refresh]");
    const endpoint = root.dataset.endpoint || "https://v1.hitokoto.cn/?encode=json&charset=utf-8&max_length=48";
    const cacheKey = "zoking-blog:hitokoto:v1";
    const cacheMaxAge = 30 * 60 * 1000;
    let loading = false;

    const parsePayload = (value: unknown): HitokotoPayload | null => {
        if (!value || typeof value !== "object") return null;
        const candidate = value as Partial<HitokotoPayload>;
        const hitokoto = candidate.hitokoto?.trim();
        const from = candidate.from?.trim();
        if (!hitokoto || hitokoto.length > 80 || !from || from.length > 40) return null;
        return { hitokoto, from };
    };

    const render = (payload: HitokotoPayload): void => {
        if (text) text.textContent = payload.hitokoto;
        if (source) source.textContent = `《${payload.from}》`;
    };

    const readCache = (): HitokotoPayload | null => {
        try {
            const cached = JSON.parse(sessionStorage.getItem(cacheKey) || "null") as {
                savedAt?: number;
                payload?: unknown;
            } | null;
            if (!cached?.savedAt || Date.now() - cached.savedAt > cacheMaxAge) return null;
            return parsePayload(cached.payload);
        } catch {
            return null;
        }
    };

    const writeCache = (payload: HitokotoPayload): void => {
        try {
            sessionStorage.setItem(cacheKey, JSON.stringify({ savedAt: Date.now(), payload }));
        } catch {
            // The local fallback remains available when session storage is restricted.
        }
    };

    const load = async (force = false): Promise<void> => {
        if (loading) return;
        if (!force) {
            const cached = readCache();
            if (cached) {
                render(cached);
                return;
            }
        }

        loading = true;
        refresh?.setAttribute("aria-busy", "true");
        const controller = new AbortController();
        const timeout = window.setTimeout(() => controller.abort(), 5000);

        try {
            const response = await fetch(endpoint, {
                credentials: "omit",
                referrerPolicy: "no-referrer",
                signal: controller.signal,
            });
            if (!response.ok) return;
            const payload = parsePayload(await response.json());
            if (!payload) return;
            render(payload);
            writeCache(payload);
        } catch {
            // Keep the local fallback; a decorative widget must not affect reading.
        } finally {
            window.clearTimeout(timeout);
            loading = false;
            refresh?.removeAttribute("aria-busy");
        }
    };

    refresh?.addEventListener("click", () => void load(true));

    if ("IntersectionObserver" in window) {
        const observer = new IntersectionObserver((entries) => {
            if (!entries.some((entry) => entry.isIntersecting)) return;
            observer.disconnect();
            void load();
        }, { rootMargin: "120px" });
        observer.observe(root);
    } else {
        void load();
    }
}
