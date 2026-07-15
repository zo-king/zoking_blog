const palette = document.querySelector<HTMLElement>("[data-command-palette]");

if (palette) {
    const dialog = palette.querySelector<HTMLElement>("[role='dialog']");
    const input = palette.querySelector<HTMLInputElement>("[data-command-input]");
    const items = Array.from(palette.querySelectorAll<HTMLElement>("[data-command-item]"));
    const empty = palette.querySelector<HTMLElement>("[data-command-empty]");
    const openButtons = Array.from(document.querySelectorAll<HTMLButtonElement>("[data-command-open]"));
    const closeButtons = Array.from(palette.querySelectorAll<HTMLElement>("[data-command-close]"));
    let activeIndex = 0;
    let restoreFocus: HTMLElement | null = null;
    let closeTimer = 0;

    const visibleItems = (): HTMLElement[] => items.filter((item) => !item.hidden);

    const setActive = (index: number): void => {
        const visible = visibleItems();
        if (visible.length === 0) {
            input?.removeAttribute("aria-activedescendant");
            return;
        }
        activeIndex = (index + visible.length) % visible.length;
        visible.forEach((item, itemIndex) => {
            const active = itemIndex === activeIndex;
            item.classList.toggle("is-active", active);
            item.setAttribute("aria-selected", String(active));
        });
        input?.setAttribute("aria-activedescendant", visible[activeIndex].id);
        visible[activeIndex].scrollIntoView({ block: "nearest" });
    };

    const filterItems = (): void => {
        const query = (input?.value || "").trim().toLocaleLowerCase("zh-CN");
        let count = 0;
        items.forEach((item) => {
            const matches = !query || (item.dataset.commandSearch || item.textContent || "").toLocaleLowerCase("zh-CN").includes(query);
            item.hidden = !matches;
            if (matches) count += 1;
        });
        if (empty) empty.hidden = count > 0;
        setActive(0);
    };

    const openPalette = (trigger?: HTMLElement): void => {
        window.clearTimeout(closeTimer);
        restoreFocus = trigger || (document.activeElement instanceof HTMLElement ? document.activeElement : null);
        palette.hidden = false;
        document.body.classList.add("command-palette-open");
        input?.setAttribute("aria-expanded", "true");
        if (input) input.value = "";
        filterItems();
        window.requestAnimationFrame(() => {
            palette.classList.add("is-open");
            input?.focus();
        });
    };

    const closePalette = (): void => {
        palette.classList.remove("is-open");
        document.body.classList.remove("command-palette-open");
        input?.setAttribute("aria-expanded", "false");
        closeTimer = window.setTimeout(() => {
            palette.hidden = true;
            restoreFocus?.focus();
        }, 160);
    };

    openButtons.forEach((button) => button.addEventListener("click", () => openPalette(button)));
    closeButtons.forEach((button) => button.addEventListener("click", closePalette));
    input?.addEventListener("input", filterItems);

    palette.querySelector<HTMLElement>("[data-command-theme]")?.addEventListener("click", () => {
        document.querySelector<HTMLButtonElement>("#dark-mode-toggle")?.click();
        closePalette();
    });

    palette.querySelector<HTMLElement>("[data-command-random]")?.addEventListener("click", (event) => {
        const target = event.currentTarget as HTMLElement;
        try {
            const posts = JSON.parse(target.dataset.commandPosts || "[]") as string[];
            if (posts.length > 0) window.location.href = posts[Math.floor(Math.random() * posts.length)];
        } catch {
            closePalette();
        }
    });

    dialog?.addEventListener("keydown", (event) => {
        const visible = visibleItems();
        if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault();
            setActive(activeIndex + (event.key === "ArrowDown" ? 1 : -1));
        } else if (event.key === "Enter" && visible.length > 0 && document.activeElement === input) {
            event.preventDefault();
            visible[activeIndex].click();
        } else if (event.key === "Escape") {
            event.preventDefault();
            closePalette();
        } else if (event.key === "Tab") {
            const focusable = [
                input,
                ...Array.from(dialog.querySelectorAll<HTMLElement>("button:not([hidden]):not([tabindex='-1']), a[href]:not([hidden]):not([tabindex='-1'])")),
            ].filter((item): item is HTMLElement => Boolean(item));
            if (focusable.length === 0) return;
            const first = focusable[0];
            const last = focusable[focusable.length - 1];
            if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        }
    });

    document.addEventListener("keydown", (event) => {
        if ((event.ctrlKey || event.metaKey) && event.key.toLocaleLowerCase() === "k") {
            event.preventDefault();
            if (palette.hidden) openPalette();
            else closePalette();
        }
    });
}

const setupRevealAnimations = (): void => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const elements = Array.from(document.querySelectorAll<HTMLElement>(".home-page .article-list > article, [data-reveal]"));
    if (elements.length === 0) return;

    const reveal = (element: HTMLElement, index: number): void => {
        const animation = element.animate([
            { opacity: 0, transform: "translateY(8px)" },
            { opacity: 1, transform: "translateY(0)" },
        ], {
            duration: 220,
            delay: Math.min(index % 5, 4) * 28,
            easing: "cubic-bezier(0.2, 0.7, 0.2, 1)",
            fill: "both",
        });
        const cleanup = (): void => {
            element.style.opacity = "";
            element.style.transform = "";
        };
        animation.finished.then(cleanup, cleanup);
    };

    if (!("IntersectionObserver" in window)) {
        elements.forEach(reveal);
        return;
    }

    const observer = new IntersectionObserver((entries) => {
        entries.forEach((entry) => {
            if (!entry.isIntersecting) return;
            const element = entry.target as HTMLElement;
            observer.unobserve(element);
            reveal(element, elements.indexOf(element));
        });
    }, { rootMargin: "0px 0px 80px", threshold: 0.06 });

    elements.forEach((element) => observer.observe(element));
};

const splash = document.querySelector("#site-splash");
if (splash) {
    const splashObserver = new MutationObserver(() => {
        if (document.contains(splash)) return;
        splashObserver.disconnect();
        setupRevealAnimations();
    });
    splashObserver.observe(document.body, { childList: true });
} else {
    setupRevealAnimations();
}
