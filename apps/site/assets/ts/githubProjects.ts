type GitHubRepository = {
    name: string;
    description: string | null;
    language: string | null;
    stargazers_count: number;
    forks_count: number;
    updated_at: string;
    pushed_at: string | null;
    html_url: string;
    fork: boolean;
    archived: boolean;
};

type CachedRepositories = {
    savedAt: number;
    repositories: GitHubRepository[];
};

const root = document.querySelector<HTMLElement>('[data-github-projects]');

if (root) {
    const list = root.querySelector<HTMLElement>('[data-projects-list]');
    const status = root.querySelector<HTMLElement>('[data-projects-status]');
    const configuredUsername = root.dataset.username?.trim() || 'zo-king';
    const username = /^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$/i.test(configuredUsername)
        ? configuredUsername
        : 'zo-king';
    const configuredMax = Number.parseInt(root.dataset.maxRepos || '6', 10);
    const maxRepos = Number.isFinite(configuredMax) ? Math.min(Math.max(configuredMax, 1), 24) : 6;
    const profileUrl = `https://github.com/${encodeURIComponent(username)}`;
    const cacheKey = `zo-king:github-projects:v1:${username}`;
    const cacheMaxAgeMs = 5 * 60 * 1000;
    const timeoutMs = 8000;

    const createElement = <K extends keyof HTMLElementTagNameMap>(
        tagName: K,
        className?: string,
        text?: string,
    ): HTMLElementTagNameMap[K] => {
        const element = document.createElement(tagName);
        if (className) element.className = className;
        if (text !== undefined) element.textContent = text;
        return element;
    };

    const safeRepositoryUrl = (value: string): string => {
        try {
            const url = new URL(value);
            return url.protocol === 'https:' && url.hostname === 'github.com' ? url.toString() : profileUrl;
        } catch {
            return profileUrl;
        }
    };

    const validDate = (value: unknown): value is string =>
        typeof value === 'string' && !Number.isNaN(Date.parse(value));

    const toRepository = (value: unknown): GitHubRepository | null => {
        if (!value || typeof value !== 'object') return null;
        const repository = value as Partial<GitHubRepository>;
        const valid = typeof repository.name === 'string'
            && (repository.description === null || typeof repository.description === 'string')
            && (repository.language === null || typeof repository.language === 'string')
            && typeof repository.stargazers_count === 'number'
            && typeof repository.forks_count === 'number'
            && validDate(repository.updated_at)
            && (repository.pushed_at === null || validDate(repository.pushed_at))
            && typeof repository.html_url === 'string'
            && typeof repository.fork === 'boolean'
            && typeof repository.archived === 'boolean';
        if (!valid) return null;
        return {
            name: repository.name as string,
            description: repository.description as string | null,
            language: repository.language as string | null,
            stargazers_count: repository.stargazers_count as number,
            forks_count: repository.forks_count as number,
            updated_at: repository.updated_at as string,
            pushed_at: repository.pushed_at as string | null,
            html_url: repository.html_url as string,
            fork: repository.fork as boolean,
            archived: repository.archived as boolean,
        };
    };

    const projectRepository = (repository: GitHubRepository): GitHubRepository => ({
        name: repository.name,
        description: repository.description,
        language: repository.language,
        stargazers_count: repository.stargazers_count,
        forks_count: repository.forks_count,
        updated_at: repository.updated_at,
        pushed_at: repository.pushed_at,
        html_url: repository.html_url,
        fork: repository.fork,
        archived: repository.archived,
    });

    const activityTime = (repository: GitHubRepository): number =>
        Date.parse(repository.pushed_at || repository.updated_at);

    const selectRepositories = (repositories: GitHubRepository[]): GitHubRepository[] =>
        repositories
            .filter((repository) => !repository.fork && !repository.archived)
            .sort((left, right) => {
                const pushedDifference = activityTime(right) - activityTime(left);
                const updatedDifference = Date.parse(right.updated_at) - Date.parse(left.updated_at);
                return pushedDifference || updatedDifference || left.name.localeCompare(right.name, 'en');
            })
            .slice(0, maxRepos);

    const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    });

    const formatDate = (value: string): string => dateFormatter.format(new Date(value));

    const languageColors: Record<string, string> = {
        C: '#555555',
        'C++': '#f34b7d',
        CSS: '#563d7c',
        Go: '#00add8',
        HTML: '#e34c26',
        Java: '#b07219',
        JavaScript: '#c7a500',
        Kotlin: '#a97bff',
        Python: '#3572a5',
        Rust: '#8b5a2b',
        Shell: '#3f8f3f',
        TypeScript: '#3178c6',
        Vue: '#2c8c6c',
    };

    const buildMetric = (label: string, value: number, symbol: string): HTMLElement => {
        const metric = createElement('span', 'github-project__metric');
        metric.setAttribute('aria-label', `${label} ${value}`);
        const icon = createElement('span', 'github-project__metric-icon', symbol);
        icon.setAttribute('aria-hidden', 'true');
        metric.append(icon, document.createTextNode(String(value)));
        return metric;
    };

    const buildCard = (repository: GitHubRepository): HTMLElement => {
        const item = createElement('li', 'github-projects__item');
        const article = createElement('article', 'github-project');
        const heading = createElement('h2', 'github-project__title');
        const link = createElement('a', undefined, repository.name);
        link.href = safeRepositoryUrl(repository.html_url);
        link.target = '_blank';
        link.rel = 'noopener noreferrer';
        link.setAttribute('aria-label', `${repository.name}（在新窗口打开 GitHub 仓库）`);
        heading.append(link);

        const description = createElement(
            'p',
            'github-project__description',
            repository.description?.trim() || '暂无项目简介，可前往 GitHub 查看 README 和最新进展。',
        );

        const metadata = createElement('div', 'github-project__metadata');
        if (repository.language) {
            const language = createElement('span', 'github-project__language');
            const swatch = createElement('span', 'github-project__language-swatch');
            swatch.setAttribute('aria-hidden', 'true');
            swatch.style.setProperty('--language-color', languageColors[repository.language] || '#64748b');
            language.append(swatch, document.createTextNode(repository.language));
            metadata.append(language);
        }
        metadata.append(
            buildMetric('Star', Math.max(0, repository.stargazers_count), '★'),
            buildMetric('Fork', Math.max(0, repository.forks_count), '⑂'),
        );

        const activityDate = repository.pushed_at || repository.updated_at;
        const updated = createElement(
            'time',
            'github-project__updated',
            `推送于 ${formatDate(activityDate)}`,
        );
        updated.dateTime = activityDate;
        metadata.append(updated);
        article.append(heading, description, metadata);
        item.append(article);
        return item;
    };

    const setStatus = (message: string): void => {
        if (status) status.textContent = message;
    };

    const renderRepositories = (repositories: GitHubRepository[]): void => {
        if (!list) return;
        list.replaceChildren(...repositories.map(buildCard));
        list.setAttribute('aria-busy', 'false');
    };

    const buildProfileLink = (): HTMLAnchorElement => {
        const link = createElement('a', undefined, `前往 ${username} 的 GitHub 主页`);
        link.href = profileUrl;
        link.target = '_blank';
        link.rel = 'noopener noreferrer';
        link.setAttribute('aria-label', `${username} 的 GitHub 主页（新窗口）`);
        return link;
    };

    const renderEmpty = (): void => {
        if (!list) return;
        const empty = createElement('li', 'github-projects__fallback');
        empty.append(
            createElement('strong', undefined, '暂时没有可展示的公开仓库'),
            createElement('p', undefined, 'Fork 和已归档仓库不会出现在这里。'),
            buildProfileLink(),
        );
        list.replaceChildren(empty);
        list.setAttribute('aria-busy', 'false');
        setStatus('没有找到符合展示条件的公开仓库。');
    };

    const readCache = (): CachedRepositories | null => {
        try {
            const serialized = window.sessionStorage.getItem(cacheKey);
            if (!serialized) return null;
            const parsed = JSON.parse(serialized) as Partial<CachedRepositories> | null;
            if (!parsed
                || typeof parsed.savedAt !== 'number'
                || Date.now() - parsed.savedAt > cacheMaxAgeMs
                || parsed.savedAt > Date.now()
                || !Array.isArray(parsed.repositories)) {
                window.sessionStorage.removeItem(cacheKey);
                return null;
            }
            const repositories = parsed.repositories.map(toRepository);
            if (repositories.some((repository) => repository === null)) {
                window.sessionStorage.removeItem(cacheKey);
                return null;
            }
            return { savedAt: parsed.savedAt, repositories: repositories as GitHubRepository[] };
        } catch {
            try {
                window.sessionStorage.removeItem(cacheKey);
            } catch {
                // Session storage can be unavailable in privacy-restricted contexts.
            }
            return null;
        }
    };

    const writeCache = (repositories: GitHubRepository[]): void => {
        try {
            const cached: CachedRepositories = {
                savedAt: Date.now(),
                repositories: repositories.map(projectRepository),
            };
            window.sessionStorage.setItem(cacheKey, JSON.stringify(cached));
        } catch {
            // The live repository list still works when browser storage is unavailable.
        }
    };

    const timeoutSignal = (milliseconds: number): { signal: AbortSignal; clear: () => void } => {
        if (typeof AbortSignal.timeout === 'function') {
            return { signal: AbortSignal.timeout(milliseconds), clear: () => undefined };
        }
        const controller = new AbortController();
        const timer = window.setTimeout(() => controller.abort(), milliseconds);
        return { signal: controller.signal, clear: () => window.clearTimeout(timer) };
    };

    const renderFailure = (message: string, retry: () => void): void => {
        const cached = readCache();
        if (cached) {
            const repositories = selectRepositories(cached.repositories);
            if (repositories.length) {
                renderRepositories(repositories);
                setStatus(`${message} 当前显示 ${formatDate(new Date(cached.savedAt).toISOString())} 保存的仓库数据。`);
                return;
            }
        }

        if (!list) return;
        const fallback = createElement('li', 'github-projects__fallback');
        const retryButton = createElement('button', 'github-projects__retry', '重新加载');
        retryButton.type = 'button';
        retryButton.addEventListener('click', retry, { once: true });
        fallback.append(
            createElement('strong', undefined, '暂时无法读取 GitHub 仓库'),
            createElement('p', undefined, message),
            retryButton,
            buildProfileLink(),
        );
        list.replaceChildren(fallback);
        list.setAttribute('aria-busy', 'false');
        setStatus(message);
    };

    const loadRepositories = async (): Promise<void> => {
        if (!list) return;
        const cached = readCache();
        if (cached) {
            const repositories = selectRepositories(cached.repositories);
            if (repositories.length) {
                renderRepositories(repositories);
                setStatus(`已从本次会话缓存加载 ${repositories.length} 个公开仓库。`);
            } else {
                renderEmpty();
            }
            return;
        }
        list.setAttribute('aria-busy', 'true');
        setStatus('正在读取 GitHub 仓库...');
        const timeout = timeoutSignal(timeoutMs);

        try {
            const endpoint = new URL(`https://api.github.com/users/${encodeURIComponent(username)}/repos`);
            endpoint.searchParams.set('type', 'owner');
            endpoint.searchParams.set('sort', 'pushed');
            endpoint.searchParams.set('direction', 'desc');
            endpoint.searchParams.set('per_page', '100');

            const response = await fetch(endpoint, {
                headers: {
                    Accept: 'application/vnd.github+json',
                    'X-GitHub-Api-Version': '2022-11-28',
                },
                credentials: 'omit',
                referrerPolicy: 'no-referrer',
                signal: timeout.signal,
            });

            if (response.status === 403 || response.status === 429) {
                throw new Error('RATE_LIMITED');
            }
            if (!response.ok) {
                throw new Error(`HTTP_${response.status}`);
            }

            const payload = await response.json() as unknown;
            if (!Array.isArray(payload)) throw new Error('INVALID_RESPONSE');
            const repositories = selectRepositories(
                payload.map(toRepository).filter((repository): repository is GitHubRepository => repository !== null),
            );
            writeCache(repositories);
            if (!repositories.length) {
                renderEmpty();
                return;
            }
            renderRepositories(repositories);
            setStatus(`已加载 ${repositories.length} 个最近更新的公开仓库。`);
        } catch (error) {
            const isRateLimited = error instanceof Error && error.message === 'RATE_LIMITED';
            const isTimeout = error instanceof DOMException
                && (error.name === 'TimeoutError' || error.name === 'AbortError');
            const message = isRateLimited
                ? 'GitHub API 请求次数已达上限，请稍后重试或直接访问个人主页。'
                : isTimeout
                    ? '连接 GitHub 超时，请检查网络后重试或直接访问个人主页。'
                    : 'GitHub 暂时不可访问，请检查网络后重试或直接访问个人主页。';
            renderFailure(message, () => void loadRepositories());
        } finally {
            timeout.clear();
        }
    };

    void loadRepositories();
}
