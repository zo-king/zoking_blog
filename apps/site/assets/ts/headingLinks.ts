import { copyPlainText } from './clipboard';

const setupStatusRegion = (): HTMLElement => {
    const existing = document.querySelector<HTMLElement>('[data-heading-link-status]');
    if (existing) return existing;

    const status = document.createElement('span');
    status.className = 'article-utility-status';
    status.dataset.headingLinkStatus = '';
    status.setAttribute('role', 'status');
    status.setAttribute('aria-live', 'polite');
    status.setAttribute('aria-atomic', 'true');
    document.body.appendChild(status);
    return status;
};

export function setupHeadingLinks(): void {
    const anchors = document.querySelectorAll<HTMLAnchorElement>('.article-content a.header-anchor[data-heading-link]');
    if (anchors.length === 0) return;

    const status = setupStatusRegion();
    anchors.forEach((anchor) => {
        if (anchor.dataset.headingLinkReady === 'true') return;
        anchor.dataset.headingLinkReady = 'true';
        const title = anchor.dataset.headingTitle || '当前章节';
        const defaultLabel = `定位并复制“${title}”章节链接`;
        let resetTimer = 0;

        anchor.addEventListener('click', async (event) => {
            if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;

            window.clearTimeout(resetTimer);
            const url = new URL(anchor.getAttribute('href') || '#', window.location.href).href;
            try {
                await copyPlainText(url);
                anchor.classList.add('is-copied');
                anchor.setAttribute('aria-label', `“${title}”章节链接已复制`);
                anchor.title = '章节链接已复制';
                status.textContent = `“${title}”章节链接已复制`;
            } catch {
                anchor.setAttribute('aria-label', defaultLabel);
                status.textContent = '未能自动复制，已定位到该章节';
            }

            resetTimer = window.setTimeout(() => {
                anchor.classList.remove('is-copied');
                anchor.setAttribute('aria-label', defaultLabel);
                anchor.title = '复制章节链接';
            }, 1400);
        });
    });
}
