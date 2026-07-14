# SITE-HOME-SPLASH-P26-001 QA

## Scope

- Homepage pagination must render six article entries per page.
- The personal introduction article must be the first entry and load its local cover.
- The splash must be short, skippable, session-scoped, non-blocking, and consistent with the site palette.
- Reduced-motion and narrow mobile layouts must not show a blocking overlay or horizontal overflow.

## Manual/Build Checks

| Check | Result |
|---|---|
| Hugo Extended development build | Pass, 61 pages |
| Homepage article count | Pass, 6 |
| First article | Pass, `关于我：把技术做成可复用的东西` |
| First article image | Pass, `/img/showcase/architecture.jpg` |
| Splash CSS/JS fingerprinted assets | Pass |
| Production build without public comments setting | Expected config gate; `comments.public.apiBase` is required by existing production policy |

## Browser Checks

Executed against `http://localhost:1313/` with Playwright and Microsoft Edge headless:

- Fresh session: the splash becomes active, then removes itself after the short display window.
- `sessionStorage` revisit: the splash is not rendered again in the same browser context.
- Skip/Escape controls: implemented and keyboard-focusable; the container is not `aria-hidden` while containing the control.
- `prefers-reduced-motion: reduce`: splash is removed without animation.
- Desktop 1280px: six entries, first image, no runtime errors.
- Mobile 390px: no horizontal overflow and no splash overlay under reduced motion.

## Design Basis

The implementation follows common production splash patterns: brand-only presentation, short duration, explicit skip, session-level frequency control, and reduced-motion support. It deliberately avoids external media, heavy loaders, progress bars, and animations that delay access to the content.
