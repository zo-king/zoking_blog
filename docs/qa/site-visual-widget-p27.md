# SITE-VISUAL-WIDGET-P27-001 QA

## Scope

- Replace the original splash artwork with a reusable, dependency-free MIT CSS pattern.
- Preserve session frequency control, skip/Escape, non-blocking fallback, and reduced motion.
- Add the Pixiv daily ranking component to the homepage right sidebar without injecting third-party scripts into the parent page.
- Verify light/dark desktop, the 1024px breakpoint, and hidden mobile sidebar behavior.

## Sources

- SpinKit: `https://github.com/tobiasahlin/SpinKit`, MIT, `circle-fade` pattern adapted locally.
- Pixiv widget guide: `https://mok.moe/p/pixiv-daily-ranking`.
- Pixiv widget repository: `https://github.com/mokeyjay/Pixiv-daily-ranking-widget`, MIT.

## Results

| Check | Result |
|---|---|
| Hugo production/minify build | Pass, 61 pages |
| Splash markup | Pass, title plus 8 animated dots |
| Splash automatic removal | Pass, 1.4s display plus 420ms curtain exit |
| Skip/Escape/session/reduced motion | Pass |
| Pixiv endpoint | Pass, HTTP 200 |
| Pixiv frame | Pass, 10 images and loaded first viewport images |
| Privacy/security attributes | Pass, lazy, no-referrer, restricted sandbox |
| 1280 light and dark | Pass, no overflow or runtime errors |
| 1024 breakpoint | Pass, sidebar remains usable |
| 390 mobile | Pass, sidebar hidden and no overflow |

Third-party ranking content and uptime remain outside this repository's control. The widget includes links to the original guide and source repository.
