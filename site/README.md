# CopyApp website

The product site for CopyApp, live at https://igalgerman.github.io/copyapp/.

It is plain HTML, CSS and JavaScript with no build step and no external requests (no CDNs, fonts, analytics or cookies). Every link is relative, so the same files work at `/copyapp/`, at a domain root, and when opened straight from disk (`file://`).

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
| `assets/install.mp4`, `assets/use.mp4` | The two how-to videos. Copy the final recordings here under exactly these names. |
| `assets/og.png` | Social preview image. |

## Deploy

Deployment is automatic. `.github/workflows/pages.yml` publishes the `site/` folder to GitHub Pages whenever a push to `master` changes `site/**`. You can also run it by hand from the Actions tab (**Deploy website to GitHub Pages → Run workflow**).

The first time only, turn on Pages in the repository: **Settings → Pages → Build and deployment → Source: GitHub Actions**.

The videos are served from this folder, so `assets/install.mp4` and `assets/use.mp4` have to be committed for the live site to show them. Until they are, the page shows the poster and the "open the video" links return 404.

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
