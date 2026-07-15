const avatars = document.querySelectorAll<HTMLElement>("[data-link-avatar]");

avatars.forEach((avatar) => {
    const image = avatar.querySelector<HTMLImageElement>("img");
    const useFallback = () => {
        avatar.classList.remove("links-card__avatar--loaded");
        avatar.classList.add("links-card__avatar--fallback");
        image?.remove();
    };

    if (!image) {
        useFallback();
        return;
    }

    const handleError = () => {
        const fallbackSource = image.dataset.fallbackSrc;
        if (fallbackSource && image.src !== fallbackSource) {
            image.src = fallbackSource;
            return;
        }
        useFallback();
    };

    image.addEventListener("load", () => {
        if (image.naturalWidth > 0) avatar.classList.add("links-card__avatar--loaded");
        else handleError();
    }, { once: true });
    image.addEventListener("error", handleError);

    if (image.complete) {
        if (image.naturalWidth > 0) avatar.classList.add("links-card__avatar--loaded");
        else handleError();
    }
});

const directory = document.querySelector<HTMLElement>("[data-links-directory]");

if (directory) {
    const cards = Array.from(directory.querySelectorAll<HTMLElement>("[data-link-card]"));
    const input = directory.querySelector<HTMLInputElement>("[data-links-search]");
    const filters = Array.from(directory.querySelectorAll<HTMLButtonElement>("[data-links-filter]"));
    const randomButton = directory.querySelector<HTMLButtonElement>("[data-links-random]");
    const status = directory.querySelector<HTMLElement>("[data-links-status]");
    const empty = directory.querySelector<HTMLElement>("[data-links-empty]");
    let activeCategory = "all";

    const normalize = (value: string): string => value.trim().toLocaleLowerCase("zh-CN");

    const visibleCards = (): HTMLElement[] => cards.filter((card) => !card.hidden);

    const update = (): void => {
        const query = normalize(input?.value || "");
        let count = 0;

        cards.forEach((card) => {
            const categoryMatches = activeCategory === "all" || card.dataset.linkCategory === activeCategory;
            const searchMatches = !query || normalize(card.dataset.linkSearch || "").includes(query);
            card.hidden = !(categoryMatches && searchMatches);
            if (!card.hidden) count += 1;
        });

        if (status) status.textContent = count === cards.length ? `共 ${count} 个站点` : `找到 ${count} 个站点`;
        if (empty) empty.hidden = count > 0;
        if (randomButton) randomButton.disabled = count === 0;
    };

    filters.forEach((button) => {
        button.addEventListener("click", () => {
            activeCategory = button.dataset.linksFilter || "all";
            filters.forEach((candidate) => {
                const active = candidate === button;
                candidate.classList.toggle("is-active", active);
                candidate.setAttribute("aria-pressed", String(active));
            });
            update();
        });
    });

    input?.addEventListener("input", update);
    randomButton?.addEventListener("click", () => {
        const candidates = visibleCards();
        if (candidates.length === 0) return;
        const card = candidates[Math.floor(Math.random() * candidates.length)];
        const link = card.querySelector<HTMLAnchorElement>("a[href]");
        if (!link) return;
        const opened = window.open(link.href, "_blank", "noopener,noreferrer");
        if (opened) opened.opener = null;
    });

    update();
}
