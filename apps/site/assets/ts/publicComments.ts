type PublicComment = {
    id?: string;
    parent_id?: string | null;
    author_name?: string;
    author_website?: string;
    content?: string;
    status?: string;
    created_at?: string;
};

type ApiEnvelope<T> = {
    data?: T;
    error?: {
        message?: string;
    };
};

const getApiData = async <T>(response: Response): Promise<T> => {
    const payload = (await response.json().catch(() => ({}))) as ApiEnvelope<T> | T;

    if (!response.ok) {
        throw new Error('请求失败，请稍后重试。');
    }

    if (payload && typeof payload === 'object' && 'data' in payload) {
        return (payload as ApiEnvelope<T>).data as T;
    }

    return payload as T;
};

const buildEndpoint = (apiBase: string, slug: string): string => {
    const normalizedBase = apiBase.replace(/\/+$/, '');
    return `${normalizedBase}/api/v1/public/posts/${encodeURIComponent(slug)}/comments`;
};

const formatCount = (count: number): string => {
    if (count === 0) {
        return '暂无已通过审核的评论';
    }

    if (count === 1) {
        return '1 条评论';
    }

    return `${count} 条评论`;
};

const formatDate = (value?: string): string => {
    if (!value) {
        return '';
    }

    const date = new Date(value);
    if (Number.isNaN(date.valueOf())) {
        return '';
    }

    return new Intl.DateTimeFormat('zh-CN', {
        dateStyle: 'medium',
        timeStyle: 'short',
    }).format(date);
};

const safeWebsite = (value?: string): string => {
    if (!value) {
        return '';
    }

    try {
        const url = new URL(value);
        if (url.protocol !== 'http:' && url.protocol !== 'https:') {
            return '';
        }
        return url.toString();
    } catch {
        return '';
    }
};

const createCommentElement = (comment: PublicComment): HTMLElement => {
    const item = document.createElement('article');
    item.className = 'public-comments__item';
    if (comment.parent_id) {
        item.classList.add('public-comments__item--reply');
    }

    const meta = document.createElement('div');
    meta.className = 'public-comments__meta';

    const authorName = comment.author_name?.trim() || '匿名读者';
    const authorUrl = safeWebsite(comment.author_website);
    const author = authorUrl ? document.createElement('a') : document.createElement('span');
    author.className = 'public-comments__author';
    author.textContent = authorName;
    if (author instanceof HTMLAnchorElement) {
        author.href = authorUrl;
        author.rel = 'nofollow noopener noreferrer';
        author.target = '_blank';
    }
    meta.appendChild(author);

    const formattedDate = formatDate(comment.created_at);
    if (formattedDate) {
        const time = document.createElement('time');
        time.className = 'public-comments__time';
        time.dateTime = comment.created_at ?? '';
        time.textContent = formattedDate;
        meta.appendChild(time);
    }

    const body = document.createElement('div');
    body.className = 'public-comments__body';
    body.textContent = comment.content?.trim() || '';

    item.append(meta, body);
    return item;
};

const setNotice = (notice: HTMLElement | null, message: string, tone?: 'success'): void => {
    if (!notice) {
        return;
    }

    notice.textContent = message;
    notice.hidden = false;
    if (tone) {
        notice.dataset.tone = tone;
    } else {
        delete notice.dataset.tone;
    }
};

const clearNotice = (notice: HTMLElement | null): void => {
    if (!notice) {
        return;
    }

    notice.textContent = '';
    notice.hidden = true;
    delete notice.dataset.tone;
};

const initPublicComments = (root: HTMLElement): void => {
    const apiBase = root.dataset.apiBase;
    const slug = root.dataset.postSlug;
    const list = root.querySelector<HTMLElement>('[data-comments-list]');
    const empty = root.querySelector<HTMLElement>('[data-comments-empty]');
    const count = root.querySelector<HTMLElement>('[data-comments-count]');
    const notice = root.querySelector<HTMLElement>('[data-comments-notice]');
    const form = root.querySelector<HTMLFormElement>('[data-comment-form]');
    const status = root.querySelector<HTMLElement>('[data-comments-form-status]');
    const submit = form?.querySelector<HTMLButtonElement>('button[type="submit"]') ?? null;

    if (!apiBase || !slug || !list || !form) {
        setNotice(notice, '评论功能暂时不可用。');
        return;
    }

    const endpoint = buildEndpoint(apiBase, slug);

    if (empty) {
        empty.hidden = false;
        empty.textContent = '正在加载评论...';
    }

    const renderComments = (comments: PublicComment[]): void => {
        list.querySelectorAll('.public-comments__item').forEach(element => element.remove());

        if (empty) {
            empty.hidden = comments.length > 0;
            empty.textContent = comments.length > 0 ? '' : '暂无已通过审核的评论';
        }

        if (count) {
            count.textContent = formatCount(comments.length);
        }

        const fragment = document.createDocumentFragment();
        comments.forEach(comment => {
            fragment.appendChild(createCommentElement(comment));
        });
        list.appendChild(fragment);
    };

    const loadComments = async (): Promise<void> => {
        try {
            const comments = await fetch(endpoint, {
                headers: { Accept: 'application/json' },
            }).then(response => getApiData<PublicComment[]>(response));

            renderComments(Array.isArray(comments) ? comments : []);
            clearNotice(notice);
        } catch {
            if (empty) {
                empty.hidden = true;
            }
            if (count) {
                count.textContent = '评论加载失败';
            }
            setNotice(notice, '评论功能暂时不可用，不影响继续阅读文章。');
        }
    };

    form.addEventListener('submit', async event => {
        event.preventDefault();
        clearNotice(notice);

        if (typeof form.reportValidity === 'function' && !form.reportValidity()) {
            return;
        }

        const formData = new FormData(form);
        const payload = {
            author_name: String(formData.get('author_name') ?? '').trim(),
            author_email: String(formData.get('author_email') ?? '').trim(),
            author_website: String(formData.get('author_website') ?? '').trim(),
            content: String(formData.get('content') ?? '').trim(),
        };

        if (!payload.author_name || !payload.content) {
            setNotice(notice, '请填写昵称和评论内容。');
            return;
        }

        if (submit) {
            submit.disabled = true;
        }
        if (status) {
            status.textContent = '正在提交...';
        }

        try {
            await fetch(endpoint, {
                method: 'POST',
                headers: {
                    Accept: 'application/json',
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(payload),
            }).then(response => getApiData<PublicComment>(response));

            form.reset();
            setNotice(notice, '评论已提交，审核通过后将公开显示。', 'success');
            if (status) {
                status.textContent = '评论将在审核通过后公开，邮箱不会展示。';
            }
        } catch (error) {
            const message = error instanceof Error && error.message !== 'Failed to fetch'
                ? error.message
                : '网络连接失败，请检查网络后重试。';
            setNotice(notice, message);
            if (status) {
                status.textContent = '';
            }
        } finally {
            if (submit) {
                submit.disabled = false;
            }
        }
    });

    void loadComments();
};

document.querySelectorAll<HTMLElement>('[data-public-comments]').forEach(initPublicComments);
