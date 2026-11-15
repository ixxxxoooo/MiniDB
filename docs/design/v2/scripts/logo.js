/* ============================================================
   MiniDB Logo · 三套概念（页面内用 CSS 变量着色，随主题和强调色变化；
   导出文件时传入固定颜色）。
   1) Stack  层叠：三枚数据圆盘，经典、稳重，小尺寸也认得出是「库」
   2) Peak   山峰 M：圆角方块里的 M，两道山峰 + 一道底弧，像字母也像数据的起伏
   3) Cube   方格立方：等距立方体，顶面是一张表（2×2 的格子），最有「表」的意味
   ============================================================ */
(function () {
  let uid = 0;
  const C = (o) => Object.assign({ a: 'var(--acc)', b: 'var(--acc-2)', fg: '#fff', ink: 'var(--fg)' }, o || {});

  // 返回 <svg> 字符串。kind: stack | peak | cube；o.tile=true 时画在渐变圆角方块（应用图标）里
  function glyph(kind, size, o) {
    const c = C(o), id = 'lg' + (++uid), s = size || 32;
    const defs = `<defs><linearGradient id="${id}" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${c.a}"/><stop offset="1" stop-color="${c.b}"/></linearGradient>
      <linearGradient id="${id}s" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity=".38"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></linearGradient></defs>`;
    let body = '';
    if (kind === 'stack') {
      const layer = (y, f, op) => `<path d="M12 ${y} v7 c0 4.4 8.95 8 20 8 s20-3.6 20-8 v-7z" fill="${f}" opacity="${op}"/><ellipse cx="32" cy="${y}" rx="20" ry="8" fill="${f}" opacity="${op}"/>`;
      body = o && o.tile
        ? `<g fill="none" stroke="${c.fg}" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="32" cy="18" rx="15" ry="6"/><path d="M17 18v10c0 3.3 6.7 6 15 6s15-2.7 15-6V18"/><path d="M17 28v10c0 3.3 6.7 6 15 6s15-2.7 15-6V28" opacity=".7"/></g>`
        : layer(40, `url(#${id})`, .45) + layer(29, `url(#${id})`, .72) + layer(18, `url(#${id})`, 1) + `<ellipse cx="32" cy="16.6" rx="14" ry="5.2" fill="url(#${id}s)"/>`;
    } else if (kind === 'peak') {
      body = `<g fill="none" stroke="${o && o.tile === false ? `url(#${id})` : c.fg}" stroke-linecap="round" stroke-linejoin="round"><path d="M17 41V18l15 15 15-15v23" stroke-width="5.2"/><path d="M20 50h24" stroke-width="3.2" opacity=".6"/><path d="M26 55.5h12" stroke-width="3.2" opacity=".32"/></g>`;
    } else { // cube
      const top = o && o.tile ? '#fff' : `url(#${id})`;
      body = `<g stroke-linejoin="round" stroke-width="2.4">
        <path d="M10 20 32 32v26L10 46z" fill="${o && o.tile ? '#fff' : c.a}" stroke="${o && o.tile ? '#fff' : c.a}" opacity="${o && o.tile ? .55 : .62}"/>
        <path d="M54 20 32 32v26l22-12z" fill="${o && o.tile ? '#fff' : c.b}" stroke="${o && o.tile ? '#fff' : c.b}" opacity="${o && o.tile ? .8 : .95}"/>
        <path d="M32 8 54 20 32 32 10 20z" fill="${top}" stroke="${top}"/></g>
        <path d="M43 14 21 26M43 26 21 14" stroke="${o && o.tile ? c.a : '#fff'}" stroke-width="1.5" stroke-linecap="round" opacity="${o && o.tile ? .75 : .7}"/>`;
    }
    if (o && o.tile !== undefined && o.tile !== false) {
      // 应用图标：渐变圆角方块 + 顶部高光
      return `<svg width="${s}" height="${s}" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg">${defs}<rect x="1" y="1" width="62" height="62" rx="15" fill="url(#${id})"/><rect x="1" y="1" width="62" height="31" rx="15" fill="url(#${id}s)" opacity=".5"/>${body}</svg>`;
    }
    return `<svg width="${s}" height="${s}" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg">${defs}${body}</svg>`;
  }

  // peak 的「无底」写法：在浅色背景上直接用渐变描边（o.tile=false）
  const wordmark = (size, ink) => `<span class="wm" style="font-size:${size}px;color:${ink || 'var(--fg)'}"><b>Mini</b>DB</span>`;
  function lockup(kind, size, o) {
    const tile = kind === 'peak';
    return `<span class="lockup" style="display:inline-flex;align-items:center;gap:${Math.round(size * .3)}px">${glyph(kind, size * 1.15, tile ? { tile: true } : o)}${wordmark(size * .78)}</span>`;
  }
  const api = { glyph, wordmark, lockup };
  if (typeof module !== 'undefined') module.exports = api; else window.LOGO = api;
})();
