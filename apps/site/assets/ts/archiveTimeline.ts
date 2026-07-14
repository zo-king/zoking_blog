const timeline = document.querySelector<HTMLElement>("[data-achievement-timeline]");
const links = Array.from(document.querySelectorAll<HTMLAnchorElement>(".archives-year-nav__link"));
const years = Array.from(document.querySelectorAll<HTMLElement>("[data-achievement-year]"));

if (timeline && links.length > 0 && years.length > 0 && "IntersectionObserver" in window) {
    const setActiveYear = (id: string) => {
        links.forEach((link) => {
            link.classList.toggle("active", link.hash === `#${id}`);
        });
    };

    const observer = new IntersectionObserver(
        (entries) => {
            const visible = entries
                .filter((entry) => entry.isIntersecting)
                .sort((first, second) => second.intersectionRatio - first.intersectionRatio)[0];

            if (visible) {
                setActiveYear(visible.target.id);
            }
        },
        { root: null, rootMargin: "0px 0px -45% 0px", threshold: [0, 0.25, 0.5, 1] },
    );

    years.forEach((year) => observer.observe(year));
}
