/* 导出 Logo 为独立 SVG（固定品牌色）。运行：node export.js
   @author ixxxxoooo */
const fs = require('fs'), path = require('path');
const L = require('../scripts/logo.js');
const BRAND = { a: '#5b5df0', b: '#8b5cf6', fg: '#ffffff' };
const out = (n, svg) => fs.writeFileSync(path.join(__dirname, n), svg.replace(/<svg /, '<svg shape-rendering="geometricPrecision" ') + '\n');
for (const k of ['peak', 'stack', 'cube']) {
  out(`${k}-tile.svg`, L.glyph(k, 512, { ...BRAND, tile: true }));
  out(`${k}-glyph.svg`, L.glyph(k, 512, k === 'peak' ? { ...BRAND, tile: false } : BRAND));
  out(`${k}-mono-black.svg`, L.glyph(k, 512, { a: '#111', b: '#111', fg: '#fff', tile: k === 'peak' ? true : undefined }));
  out(`${k}-mono-white.svg`, L.glyph(k, 512, { a: '#fff', b: '#fff', fg: '#111', tile: k === 'peak' ? true : undefined }));
}
console.log(fs.readdirSync(__dirname).filter(f => f.endsWith('.svg')).join('\n'));
