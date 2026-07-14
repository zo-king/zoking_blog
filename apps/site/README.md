# Zoking Blog Site

This is the reader-facing Hugo site. It starts from the upstream Stack demo content so the first viewport closely matches `https://demo.stack.cai.im/`.

Run locally:

```powershell
hugo server --source apps/site
```

Build:

```powershell
hugo --source apps/site --destination ..\..\dist\site --minify
```

The theme is resolved through the local repository root with a Hugo module replace directive.
