const slideToggle = (target: HTMLElement, open: boolean) => {
    target.classList.toggle('show', open);
};

export default function () {
    const toggleMenu = document.getElementById('toggle-menu') as HTMLButtonElement | null;
    const mainMenu = document.getElementById('main-menu');
    if (!toggleMenu || !mainMenu) return;

    const setMenuOpen = (open: boolean, restoreFocus = false) => {
        document.body.classList.toggle('show-menu', open);
        toggleMenu.classList.toggle('is-active', open);
        toggleMenu.setAttribute('aria-expanded', String(open));
        slideToggle(mainMenu, open);
        if (open) mainMenu.querySelector<HTMLElement>('a, button, select')?.focus();
        if (!open && restoreFocus) toggleMenu.focus();
    };

    toggleMenu.addEventListener('click', () => {
        setMenuOpen(toggleMenu.getAttribute('aria-expanded') !== 'true');
    });

    document.addEventListener('keydown', event => {
        if (event.key === 'Escape' && toggleMenu.getAttribute('aria-expanded') === 'true') {
            setMenuOpen(false, true);
        }
    });

    document.addEventListener('pointerdown', event => {
        if (toggleMenu.getAttribute('aria-expanded') !== 'true') return;
        const target = event.target;
        if (target instanceof Node && !mainMenu.contains(target) && !toggleMenu.contains(target)) {
            setMenuOpen(false);
        }
    });

    mainMenu.addEventListener('click', event => {
        const target = event.target;
        if (target instanceof Element && target.closest('a')) setMenuOpen(false);
    });
}
