const splash = document.querySelector<HTMLElement>("#site-splash");
const skipButton = document.querySelector<HTMLButtonElement>("#site-splash-skip");

if (splash) {
    const storageKey = "zoking-blog:splash-seen";
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    let dismissed = false;

    const hasSeenSplash = (): boolean => {
        try {
            return window.sessionStorage.getItem(storageKey) === "1";
        } catch {
            return false;
        }
    };

    const rememberSplash = (): void => {
        try {
            window.sessionStorage.setItem(storageKey, "1");
        } catch {
            // Private browsing can disable sessionStorage; the animation still works.
        }
    };

    const dismissSplash = (): void => {
        if (dismissed) {
            return;
        }

        dismissed = true;
        rememberSplash();
        splash.classList.remove("is-active");
        splash.classList.add("is-leaving");
        window.setTimeout(() => splash.remove(), 460);
    };

    skipButton?.addEventListener("click", dismissSplash);
    window.addEventListener("keydown", (event) => {
        if (event.key === "Escape") {
            dismissSplash();
        }
    });

    if (reducedMotion || hasSeenSplash()) {
        splash.remove();
    } else {
        splash.classList.add("is-active");
        window.setTimeout(dismissSplash, 1400);
    }
}
