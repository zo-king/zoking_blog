import * as params from '@params';
import { copyPlainText } from './clipboard';

const languageNames: Record<string, string> = {
    bash: 'Shell',
    c: 'C',
    cpp: 'C++',
    css: 'CSS',
    dockerfile: 'Dockerfile',
    go: 'Go',
    html: 'HTML',
    javascript: 'JavaScript',
    js: 'JavaScript',
    json: 'JSON',
    jsx: 'JSX',
    plaintext: '文本',
    powershell: 'PowerShell',
    ps1: 'PowerShell',
    scss: 'SCSS',
    sh: 'Shell',
    shell: 'Shell',
    sql: 'SQL',
    text: '文本',
    toml: 'TOML',
    ts: 'TypeScript',
    tsx: 'TSX',
    typescript: 'TypeScript',
    yaml: 'YAML',
    yml: 'YAML',
    zsh: 'Shell',
};

const displayLanguage = (language: string): string => {
    const normalized = language.trim().toLocaleLowerCase('en-US');
    return languageNames[normalized] || normalized.toLocaleUpperCase('en-US');
};

export function setupCodeCopy() {
    const highlights = document.querySelectorAll<HTMLElement>('.article-content div.highlight');
    const copyText = params.codeblock.copy;
    const copiedText = params.codeblock.copied;

    highlights.forEach((highlight) => {
        if (highlight.dataset.codeUtilitiesReady === 'true') return;

        const codeBlock = highlight.querySelector<HTMLElement>('code[data-lang]');
        if (!codeBlock) return;

        const language = displayLanguage(codeBlock.dataset.lang || 'text');
        const toolbar = document.createElement('div');
        toolbar.className = 'code-block-toolbar';

        const languageLabel = document.createElement('span');
        languageLabel.className = 'code-block-language';
        languageLabel.textContent = language;
        languageLabel.title = `代码语言：${language}`;

        const copyButton = document.createElement('button');
        copyButton.type = 'button';
        copyButton.className = 'copyCodeButton';
        copyButton.textContent = copyText;
        copyButton.setAttribute('aria-label', `复制 ${language} 代码`);

        const status = document.createElement('span');
        status.className = 'article-utility-status';
        status.setAttribute('role', 'status');
        status.setAttribute('aria-live', 'polite');
        status.setAttribute('aria-atomic', 'true');

        toolbar.append(languageLabel, copyButton, status);
        highlight.prepend(toolbar);
        highlight.dataset.codeUtilitiesReady = 'true';

        let resetTimer = 0;
        copyButton.addEventListener('click', async () => {
            window.clearTimeout(resetTimer);
            try {
                await copyPlainText(codeBlock.textContent || '');
                copyButton.textContent = copiedText;
                copyButton.setAttribute('aria-label', `${language} 代码已复制`);
                copyButton.classList.add('is-copied');
                status.textContent = `${language} 代码已复制`;
            } catch {
                copyButton.textContent = '复制失败';
                copyButton.setAttribute('aria-label', `${language} 代码复制失败`);
                status.textContent = '暂时无法复制，请手动选择代码';
            }

            resetTimer = window.setTimeout(() => {
                copyButton.textContent = copyText;
                copyButton.setAttribute('aria-label', `复制 ${language} 代码`);
                copyButton.classList.remove('is-copied');
            }, 1400);
        });
    });
}
