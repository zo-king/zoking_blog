const anchorLinksQuery = 'a[href]';

function setupSmoothAnchors() {
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)');

    document.querySelectorAll<HTMLAnchorElement>(anchorLinksQuery).forEach((anchor) => {
        if (anchor.dataset.smoothAnchorReady === 'true') return;
        const href = anchor.getAttribute('href');
        if (!href?.startsWith('#')) return;

        anchor.dataset.smoothAnchorReady = 'true';
        anchor.addEventListener('click', (event) => {
            const rawTargetId = anchor.getAttribute('href')?.substring(1) || '';
            let targetId = rawTargetId;
            try {
                targetId = decodeURI(rawTargetId);
            } catch {
                // Keep the raw id so malformed third-party anchors can still degrade normally.
            }

            const target = document.getElementById(targetId);
            if (!target) return;

            event.preventDefault();
            const offset = target.getBoundingClientRect().top - document.documentElement.getBoundingClientRect().top;
            window.history.pushState({}, '', anchor.getAttribute('href'));
            window.scrollTo({
                top: offset,
                behavior: reduceMotion.matches ? 'auto' : 'smooth',
            });
        });
    });
}

export { setupSmoothAnchors };
