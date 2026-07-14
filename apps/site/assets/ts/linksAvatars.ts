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
