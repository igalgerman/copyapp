# CopyApp website

The tracked product site for CopyApp 1.0.1. The canonical public URL is https://copyapp.igalg.chatgpt.site/; GitHub Pages at https://igalgerman.github.io/copyapp/ is the tracked mirror.

It is plain HTML, CSS and JavaScript with no build step. Fonts, styles and scripts are local; there are no analytics or trackers. Downloads and videos use the versioned GitHub release assets. Local asset links are relative, so the same files work at `/copyapp/`, at a domain root, and from disk (`file://`). The site remembers language/theme in `localStorage`.

## Files

| File | What it is |
| --- | --- |
| `index.html` | The page. The Hebrew text (the default language, RTL) lives here. |
| `app.js` | English and Russian strings, the language and theme switchers, the hero animation, scroll reveals, video tabs and the FAQ accordion. |
| `styles.css` | All styles. Light and dark themes, logical (RTL-safe) properties, `prefers-reduced-motion` support. |
| `404.html` | Self-contained "not found" page (GitHub Pages serves it for unknown paths). |
| `og.html` | Source for the social preview image `assets/og.png` (1200×630). |
| `assets/icon.svg` | Logo and favicon. |
| `assets/bricolage.woff2` | Bricolage Grotesque display font (SIL Open Font License). |
| `assets/poster-install.svg`, `assets/poster-use.svg` | Video posters, also shown if a video file is missing. |
| GitHub release `v1.0.1`: `install.mp4`, `use.mp4` | The two how-to videos, served directly from the same release as the executable. No local video copies are required. |
| `assets/og.png` | Social preview image. |

## Deploy

Deployment is automatic. `.github/workflows/pages.yml` publishes the `site/` folder to GitHub Pages whenever a push to `master` changes `site/**`. You can also run it by hand from the Actions tab (**Deploy website to GitHub Pages → Run workflow**).

The first time only, turn on Pages in the repository: **Settings → Pages → Build and deployment → Source: GitHub Actions**.

All download/video links pin `https://github.com/igalgerman/copyapp/releases/download/v1.0.1/{copyapp.exe,install.mp4,use.mp4}`. Publish those verified assets before deploying this version of the page. The checksum link points to the same version's release page. GitHub release URLs are version-pinned; the release workflow never overwrites assets of an existing release.

The ignored `landing/` directory is a separate hosted deployment workspace and is not uploaded by Pages. For subsequent edits, use `site/` as the reviewed source and copy/package those files for the hosted deployment; do not assume changes to `landing/` reach GitHub Pages. Both deployments must retain the same versioned release URLs. The visible installation notes include the optional Android Share-menu setup and current limitations in Hebrew, English and Russian.

## Preview locally

Open `index.html` in a browser, or serve the folder:

```
npx http-server site -p 8080
```

To test the GitHub Pages sub-path, serve the parent of a folder named `copyapp` that contains these files, then open http://localhost:8080/copyapp/.

## Editing text

- Hebrew: edit `index.html` (each translatable element has a `data-i18n` key).
- English and Russian: edit the `DICT.en` / `DICT.ru` objects in `app.js`, using the same keys.
- The strings for the animated demo in the hero are in `STAGE` in `app.js`.

The chosen language and theme are remembered in `localStorage` (`copyapp.lang`, `copyapp.theme`); nothing else is stored.

## Re-render the social image

Open `og.html` at a 1200×630 viewport and take a screenshot, for example with Playwright:

```js
await page.setViewportSize({ width: 1200, height: 630 });
await page.goto('http://localhost:8080/og.html');
await page.screenshot({ path: 'site/assets/og.png' });
```
