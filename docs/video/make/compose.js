// Builds the final mp4 for one scenario from raw/<scenario>/{pc,phone}.webm + meta.json.
//   node compose.js install|use      env: FFMPEG (path to ffmpeg.exe), RAW_DIR, OUT_DIR, WEB_DIR, NODE_PATH
const fs = require('fs'), path = require('path'), cp = require('child_process');
const { chromium } = require('playwright');
const name = process.argv[2];
const FF = process.env.FFMPEG || 'ffmpeg', RAW = path.join(process.env.RAW_DIR, name), OUT = process.env.OUT_DIR, WEB = process.env.WEB_DIR;
const meta = JSON.parse(fs.readFileSync(path.join(RAW, 'meta.json'), 'utf8'));
// Windows font. ffmpeg's filter syntax treats ':' in "C:/..." as an option separator, so the font is
// copied next to the work files as font.ttf and referenced by that relative name (no colon to escape).
const FONT = 'C:/Windows/Fonts/segoeuib.ttf';
const run = (args) => { const r = cp.spawnSync(FF, ['-y', '-hide_banner', '-loglevel', 'error', ...args], { cwd: RAW, stdio: 'inherit' }); if (r.status) throw new Error('ffmpeg failed: ' + args.join(' ')); };

const CARDS = {
  install: { title: 'How to install CopyApp', sub: 'About one minute. No technical skills needed.', out: 'You are connected!', outSub: 'Next: watch "How to use CopyApp"' },
  use: { title: 'How to use CopyApp', sub: 'Copy on one device, paste on the other.', out: 'Copy. Paste. Done.', outSub: 'Free. Works on your own Wi-Fi. Nothing goes to the cloud.' },
}[name];

async function cards() {
  const b = await chromium.launch(); const p = await b.newPage({ viewport: { width: 1280, height: 720 } });
  const font = 'file:///' + path.join(WEB, 'bricolage.woff2').replace(/\\/g, '/');
  const icon = 'file:///' + path.join(WEB, 'icon-512.png').replace(/\\/g, '/');
  const html = (h, s) => `<!doctype html><meta charset=utf-8><style>@font-face{font-family:B;src:url(${font})}
  body{margin:0;width:1280px;height:720px;background:radial-gradient(circle at 20% 15%,#4a5df0,#2E44E8 45%,#1d2eb8);display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;color:#fff;font-family:B,"Segoe UI",sans-serif;position:relative;overflow:hidden}
  .blob{position:absolute;right:-170px;bottom:-170px;width:340px;height:340px;border-radius:50%;background:#FFD447}
  img{width:150px;height:150px;border-radius:34px;box-shadow:0 20px 50px rgba(0,0,0,.3);position:relative}
  h1{font-size:88px;margin:36px 0 10px;line-height:1.05;position:relative}
  p{font:600 34px "Segoe UI",sans-serif;color:#FFD447;margin:0;position:relative}</style>
  <div class=blob></div><img src="${icon}"><h1>${h}</h1><p>${s}</p>`;
  const tmp = path.join(RAW, 'card.html');
  for (const [f, h, s] of [['intro', CARDS.title, CARDS.sub], ['outro', CARDS.out, CARDS.outSub]]) {
    fs.writeFileSync(tmp, html(h, s));
    await p.goto('file:///' + tmp.replace(/\\/g, '/')); await p.evaluate(() => document.fonts.ready);
    await p.waitForTimeout(400);
    await p.screenshot({ path: path.join(RAW, f + '.png') });
  }
  await b.close();
}

(async () => {
  await cards();
  fs.copyFileSync(FONT, path.join(RAW, 'font.ttf'));
  const marks = meta.marks; const end = marks.find((m) => m.text === 'END').t;
  const caps = marks.filter((m) => m.text !== 'END').map((m, i, a) => ({ ...m, t1: (a[i + 1] ? a[i + 1].t : end) }));
  caps.forEach((c, i) => fs.writeFileSync(path.join(RAW, `cap${i}.txt`), c.text));
  const dt = (file, extra) => `drawtext=fontfile=font.ttf:textfile=${file}:fontcolor=0x1E1A05:fontsize=36:x=(w-text_w)/2:y=650+(70-text_h)/2${extra || ''}`;
  const lab = (t, x, y) => `drawtext=fontfile=font.ttf:text='${t}':fontcolor=0xFFD447:fontsize=26:x=${x}:y=${y}`;
  const filters = [
    `color=c=0x1C2A9C:s=1280x720:r=25:d=${end.toFixed(2)}[bg]`,
    `[0:v]trim=0:${end.toFixed(2)},setpts=PTS-STARTPTS,fps=25,scale=880:495,pad=892:507:6:6:color=white[pc]`,
    `[1:v]setpts=PTS-STARTPTS,fps=25,scale=268:580:flags=lanczos,pad=288:600:10:10:color=0x0b0d17[ph]`,
    `[bg][pc]overlay=28:78[a]`, `[a][ph]overlay=964:46[b]`,
    `[b]drawbox=x=0:y=650:w=1280:h=70:color=0xFFD447:t=fill,${lab('On your PC', 28, 36)},${lab('On your phone', 964, 8)},` +
      caps.map((c, i) => dt(`cap${i}.txt`, `:enable='between(t,${c.t.toFixed(2)},${c.t1.toFixed(2)})'`)).join(',') + `[v]`,
  ].join(';');
  fs.writeFileSync(path.join(RAW, 'filter.txt'), filters);
  const common = ['-c:v', 'libx264', '-preset', 'medium', '-crf', '20', '-pix_fmt', 'yuv420p', '-r', '25', '-c:a', 'aac', '-b:a', '64k', '-ar', '44100', '-ac', '2'];
  const sil = ['-f', 'lavfi', '-i', 'anullsrc=r=44100:cl=stereo'];
  run(['-i', 'pc.webm', '-itsoffset', String(meta.phoneOffset), '-i', 'phone.webm', ...sil, '-/filter_complex', 'filter.txt', '-map', '[v]', '-map', '2:a', '-t', end.toFixed(2), ...common, 'main.mp4']);
  for (const f of ['intro', 'outro'])
    run(['-loop', '1', '-t', '4', '-i', f + '.png', ...sil, '-vf', 'fade=in:0:12,fade=out:st=3.5:d=0.5,format=yuv420p', '-t', '4', ...common, '-shortest', f + '.mp4']);
  fs.writeFileSync(path.join(RAW, 'list.txt'), "file 'intro.mp4'\nfile 'main.mp4'\nfile 'outro.mp4'\n");
  fs.mkdirSync(OUT, { recursive: true });
  run(['-f', 'concat', '-safe', '0', '-i', 'list.txt', '-c', 'copy', '-movflags', '+faststart', path.join(OUT, name + '.mp4')]);
  console.log('wrote', path.join(OUT, name + '.mp4'));
})().catch((e) => { console.error(e); process.exit(1); });
