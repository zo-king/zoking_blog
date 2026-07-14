type ColorScheme = 'light' | 'dark' | 'auto';

class StackColorScheme {
    private readonly localStorageKey = 'StackColorScheme';
    private currentScheme: ColorScheme;
    private systemPreferScheme: ColorScheme;
    private readonly toggleEl: HTMLButtonElement | null;

    constructor(toggleEl: HTMLElement | null) {
        this.toggleEl = toggleEl instanceof HTMLButtonElement ? toggleEl : null;
        this.systemPreferScheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        this.currentScheme = this.getSavedScheme();
        this.bindMatchMedia();
        this.updateControl();
        this.toggleEl?.addEventListener('click', () => this.toggle());
        if (document.body.style.transition === '') document.body.style.setProperty('transition', 'background-color .3s ease');
    }

    private toggle() {
        this.currentScheme = this.isDark() ? 'light' : 'dark';
        this.setBodyClass();
        if (this.currentScheme === this.systemPreferScheme) this.currentScheme = 'auto';
        localStorage.setItem(this.localStorageKey, this.currentScheme);
        this.updateControl();
    }

    private isDark() {
        return this.currentScheme === 'dark' || (this.currentScheme === 'auto' && this.systemPreferScheme === 'dark');
    }

    private setBodyClass() {
        document.documentElement.dataset.scheme = this.isDark() ? 'dark' : 'light';
        window.dispatchEvent(new CustomEvent('onColorSchemeChange', { detail: document.documentElement.dataset.scheme }));
    }

    private updateControl() {
        this.toggleEl?.setAttribute('aria-pressed', String(this.isDark()));
    }

    private getSavedScheme(): ColorScheme {
        const saved = localStorage.getItem(this.localStorageKey);
        return saved === 'light' || saved === 'dark' || saved === 'auto' ? saved : 'auto';
    }

    private bindMatchMedia() {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', event => {
            this.systemPreferScheme = event.matches ? 'dark' : 'light';
            this.setBodyClass();
            this.updateControl();
        });
    }
}

export default StackColorScheme;
