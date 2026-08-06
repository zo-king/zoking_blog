/*
 * Service Worker:离线可安装支持。
 * 策略:HTML 导航请求走网络优先(失败回缓存,再回离线页);
 * 同源静态资源走 stale-while-revalidate(资源已 fingerprint,内容不会串版本)。
 * API 请求(/api/)与跨域请求一律不拦截,避免影响评论与统计的实时性。
 */
const CACHE_PREFIX = "zoking-pwa";
const CACHE_NAME = `${CACHE_PREFIX}-v1`;
const OFFLINE_URL = "/offline.html";
const PRECACHE = [OFFLINE_URL, "/manifest.webmanifest", "/img/pwa/icon-192.png"];

const sw = self as unknown as {
    addEventListener(type: string, listener: (event: any) => void): void;
    skipWaiting(): Promise<void>;
    clients: { claim(): Promise<void> };
    location: Location;
};

sw.addEventListener("install", (event: any) => {
    event.waitUntil(
        caches
            .open(CACHE_NAME)
            .then((cache) => cache.addAll(PRECACHE))
            .then(() => sw.skipWaiting()),
    );
});

sw.addEventListener("activate", (event: any) => {
    event.waitUntil(
        caches
            .keys()
            .then((keys) =>
                Promise.all(
                    keys
                        .filter((key) => key.startsWith(CACHE_PREFIX) && key !== CACHE_NAME)
                        .map((key) => caches.delete(key)),
                ),
            )
            .then(() => sw.clients.claim()),
    );
});

sw.addEventListener("fetch", (event: any) => {
    const request: Request = event.request;
    if (request.method !== "GET") return;

    const url = new URL(request.url);
    if (url.origin !== sw.location.origin) return;
    if (url.pathname.startsWith("/api/")) return;

    if (request.mode === "navigate") {
        event.respondWith(
            fetch(request)
                .then((response) => {
                    const copy = response.clone();
                    caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
                    return response;
                })
                .catch(() =>
                    caches
                        .match(request)
                        .then((hit) => hit || caches.match(OFFLINE_URL))
                        .then((hit) => hit as Response),
                ),
        );
        return;
    }

    event.respondWith(
        caches.match(request).then((hit) => {
            const refresh = fetch(request)
                .then((response) => {
                    if (response.ok) {
                        const copy = response.clone();
                        caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
                    }
                    return response;
                })
                .catch(() => undefined);
            return hit || refresh.then((r) => r || Promise.reject(new Error("offline")));
        }),
    );
});
