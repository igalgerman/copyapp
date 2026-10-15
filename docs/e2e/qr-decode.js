// Decodes the QR PNG at argv[2] and prints its text. Needs jsqr and pngjs on NODE_PATH.
const fs = require('fs');
const { PNG } = require('pngjs');
const jsQR = require('jsqr');
const png = PNG.sync.read(fs.readFileSync(process.argv[2]));
const r = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
if (!r) { console.error('no QR found'); process.exit(1); }
process.stdout.write(r.data);
