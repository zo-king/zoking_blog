type Metrics = { views?: number; likes?: number; liked?: boolean };
type Envelope<T> = { data?: T; error?: { message?: string } };

const unwrap = async <T>(response: Response): Promise<T> => {
    const payload = (await response.json().catch(() => ({}))) as Envelope<T> | T;
    if (!response.ok) {
        throw new Error('请求失败');
    }
    if (payload && typeof payload === 'object' && 'data' in payload) {
        return (payload as Envelope<T>).data as T;
    }
    return payload as T;
};

const endpoint = (apiBase: string, slug: string, suffix: string): string => {
    const base = apiBase.replace(/\/+$/, '');
    return `${base}/api/v1/public/posts/${encodeURIComponent(slug)}/${suffix}`;
};

const formatCount = (value: number): string => {
    if (value >= 10000) {
        return `${(value / 10000).toFixed(1).replace(/\.0$/, '')}w`;
    }
    if (value >= 1000) {
        return `${(value / 1000).toFixed(1).replace(/\.0$/, '')}k`;
    }
    return String(value);
};

const initPostEngagement = (root: HTMLElement): void => {
    const apiBase = root.dataset.apiBase;
    const slug = root.dataset.postSlug;
    const viewsEl = root.querySelector<HTMLElement>('[data-engagement-views-count]');
    const likesEl = root.querySelector<HTMLElement>('[data-engagement-likes-count]');
    const likeBtn = root.querySelector<HTMLButtonElement>('[data-engagement-like]');

    if (!apiBase || !slug || !likeBtn) {
        root.hidden = true;
        return;
    }

    const render = (metrics: Metrics): void => {
        if (viewsEl && typeof metrics.views === 'number') {
            viewsEl.textContent = formatCount(metrics.views);
        }
        if (likesEl && typeof metrics.likes === 'number') {
            likesEl.textContent = formatCount(metrics.likes);
        }
        const liked = Boolean(metrics.liked);
        likeBtn.setAttribute('aria-pressed', liked ? 'true' : 'false');
        likeBtn.classList.toggle('is-liked', liked);
    };

    // Record a view on load, then reflect counts. If the view POST is rate-limited,
    // fall back to a read-only metrics fetch so the counts still display.
    const load = async (): Promise<void> => {
        try {
            render(await fetch(endpoint(apiBase, slug, 'views'), {
                method: 'POST',
                headers: { Accept: 'application/json' },
            }).then(response => unwrap<Metrics>(response)));
        } catch {
            try {
                render(await fetch(endpoint(apiBase, slug, 'metrics'), {
                    headers: { Accept: 'application/json' },
                }).then(response => unwrap<Metrics>(response)));
            } catch {
                root.hidden = true;
            }
        }
    };

    likeBtn.addEventListener('click', async () => {
        likeBtn.disabled = true;
        try {
            render(await fetch(endpoint(apiBase, slug, 'likes'), {
                method: 'POST',
                headers: { Accept: 'application/json' },
            }).then(response => unwrap<Metrics>(response)));
        } catch {
            // Keep the current state on failure; the reader can retry.
        } finally {
            likeBtn.disabled = false;
        }
    });

    void load();
};

document.querySelectorAll<HTMLElement>('[data-post-engagement]').forEach(initPostEngagement);
