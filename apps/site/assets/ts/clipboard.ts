export async function copyPlainText(text: string): Promise<void> {
    if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
        return;
    }

    const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.inset = '0 auto auto -9999px';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    textarea.setSelectionRange(0, textarea.value.length);

    const copied = document.execCommand('copy');
    textarea.remove();
    activeElement?.focus({ preventScroll: true });

    if (!copied) throw new Error('clipboard copy failed');
}
