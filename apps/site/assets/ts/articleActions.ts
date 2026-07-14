const shareButton = document.querySelector<HTMLButtonElement>('[data-article-share]');
const shareStatus = document.querySelector<HTMLElement>('[data-article-share-status]');
const readingProgress = document.querySelector<HTMLElement>('[data-reading-progress]');
const readingProgressBar = document.querySelector<HTMLElement>('[data-reading-progress-bar]');
const resumeButton = document.querySelector<HTMLButtonElement>('[data-reading-resume]');
const articleContent = document.querySelector<HTMLElement>('.article-content');

type ReadingState = {
    progress: number;
    updatedAt: number;
};

const storageKey = `zoking:reading-progress:v1:${window.location.pathname}`;
const stateMaxAge = 30 * 24 * 60 * 60 * 1000;

const readState = (): ReadingState | null => {
    try {
        const value = window.localStorage.getItem(storageKey);
        if (!value) return null;
        const parsed = JSON.parse(value) as Partial<ReadingState>;
        if (typeof parsed.progress !== 'number' || parsed.progress < 0.05 || parsed.progress >= 0.98) return null;
        if (typeof parsed.updatedAt !== 'number' || Date.now() - parsed.updatedAt > stateMaxAge) {
            window.localStorage.removeItem(storageKey);
            return null;
        }
        return { progress: Math.min(parsed.progress, 1), updatedAt: Number(parsed.updatedAt) || 0 };
    } catch {
        return null;
    }
};

const writeState = (progress: number): void => {
    try {
        if (progress >= 0.98) {
            window.localStorage.removeItem(storageKey);
            return;
        }
        if (progress >= 0.05) {
            window.localStorage.setItem(storageKey, JSON.stringify({ progress, updatedAt: Date.now() }));
        }
    } catch {
        // Reading progress remains fully optional when browser storage is unavailable.
    }
};

const getReadingMetrics = () => {
    if (!articleContent) return { progress: 0, startY: 0, distance: 1 };
    const startY = articleContent.getBoundingClientRect().top + window.scrollY;
    const distance = Math.max(articleContent.scrollHeight - window.innerHeight * 0.65, 1);
    const progress = Math.min(Math.max((window.scrollY - startY) / distance, 0), 1);
    return { progress, startY, distance };
};

if (articleContent && readingProgress && readingProgressBar) {
    let frame = 0;
    let lastSavedAt = 0;
    const updateProgress = (persist = true) => {
        frame = 0;
        const { progress } = getReadingMetrics();
        const percentage = Math.round(progress * 100);
        readingProgressBar.style.transform = `scaleX(${progress})`;
        readingProgress.setAttribute('aria-valuenow', String(percentage));
        if (persist && (progress >= 0.98 || (progress >= 0.05 && Date.now() - lastSavedAt >= 500))) {
            writeState(progress);
            lastSavedAt = Date.now();
        }
    };

    const scheduleUpdate = () => {
        if (!frame) frame = window.requestAnimationFrame(updateProgress);
    };

    const savedState = readState();
    if (savedState && resumeButton) {
        resumeButton.hidden = false;
        const resumeLabel = resumeButton.querySelector<HTMLElement>('[data-reading-resume-label]');
        if (resumeLabel) resumeLabel.textContent = `继续阅读（${Math.round(savedState.progress * 100)}%）`;
        resumeButton.addEventListener('click', () => {
            const { startY, distance } = getReadingMetrics();
            window.scrollTo({
                top: Math.max(startY + distance * savedState.progress, 0),
                behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
            });
            resumeButton.hidden = true;
        });
    }

    updateProgress();
    window.addEventListener('scroll', scheduleUpdate, { passive: true });
    window.addEventListener('resize', scheduleUpdate);
    window.addEventListener('pagehide', () => {
        writeState(getReadingMetrics().progress);
    });
}

shareButton?.addEventListener('click', async () => {
    const title = shareButton.dataset.title || document.title;
    const url = shareButton.dataset.url || window.location.href;

    try {
        if (navigator.share) {
            await navigator.share({ title, url });
            if (shareStatus) shareStatus.textContent = '分享面板已打开';
            return;
        }
        await navigator.clipboard.writeText(url);
        if (shareStatus) shareStatus.textContent = '文章链接已复制';
    } catch (error) {
        if (error instanceof DOMException && error.name === 'AbortError') return;
        if (shareStatus) shareStatus.textContent = '暂时无法分享，请复制浏览器地址';
    }
});
