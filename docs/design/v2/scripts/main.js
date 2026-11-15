/* 交互总线：事件委托、数据网格编辑、ER 拖拽、主题切换、快捷键、提示卡 */
(function () {
  const THEMES = ['light', 'dark', 'paper', 'aurora'];
  const viewT = fn => (document.startViewTransition && !S.reduce ? document.startViewTransition(fn) : fn());

  /* ---------- 基础动作 ---------- */
  window.go = (p, o = {}) => { S.xtab = null; if (PAGES[p] && PAGES[p].asModal) { if (PAGES[S.page] && PAGES[S.page].asModal) S.page = 'data'; return openModal(PAGES[p].asModal); } S.page = p; S.modal = null; S.menu = null; if (o.dock !== undefined) S.dock = o.dock; render(); };
  window.openModal = m => { S.modal = m; S.menu = null; if (m && m.startsWith('wiz:')) S[m.slice(4) + 'Step'] = 0; if (m === 'backup') S.bakStep = 0; if (m === 'restore') S.rstStep = 0; S._focusPal = true; if (m === 'palette') { S.q = ''; S.palSel = 0; S.palScope = 0; } if (m === 'conn') { S.ctab = 0; S.test = 0; } renderOV(); };
  window.dockTo = k => { const dk = $('.rdock-a,.rdock'); if (S.dock === k && dk && !S.reduce) { dk.classList.add('closing'); setTimeout(() => { S.dock = null; render(false); }, 160); } else { S.dock = S.dock === k ? null : k; render(false); } };
  window.closeOV = () => { const sc = $('#ov > .scrim'); if (sc && !S.reduce && !sc.classList.contains('out') && S.modal) { sc.classList.add('out'); setTimeout(() => { S.modal = null; S.menu = null; renderOV(); }, 120); } else { S.modal = null; S.menu = null; renderOV(); } };
  window.cycleTheme = () => setTheme(THEMES[(THEMES.indexOf(S.theme) + 1) % 4]);
  window.setTheme = t => viewT(() => { S.theme = t; render(false); });
  window.toast = (t, k = 'ok', s = '', undo = false) => {
    const o = { id: Date.now() + Math.random(), t, k, s, undo }; S.toasts.push(o); if (S.toasts.length > 3) S.toasts.shift(); renderOV();
    setTimeout(() => { o.out = true; renderOV(); setTimeout(() => { S.toasts = S.toasts.filter(x => x !== o); renderOV(); }, 220); }, 3400);
  };
  window.connect = id => {
    const c = DB.conns.find(x => x.id === id); S.modal = null;
    toast(`正在连接 ${c.name}…`, 'info', c.host);
    setTimeout(() => { c.open = true; S.conn = id; S.page = 'data'; S.tabs = ['overview', 'data', 'structure', 'sql', 'er']; render(); toast(`已连接 ${c.name}`, 'ok', c.env === 'PROD' ? '生产环境：窗口顶部显示红色环境带' : `${c.type} ${c.ver} · 38 ms`); }, 650);
  };
  const EASES = { 默认: 'cubic-bezier(.2,.8,.2,1)', 回弹: 'cubic-bezier(.34,1.45,.64,1)', 线性: 'linear', 慢入: 'cubic-bezier(.4,0,.2,1)' };
  window.toastLocal = t => { const b = $('#verdict'); if (!b) return toast(t); const n = document.createElement('div'); n.className = 'chip ok'; n.style.marginTop = '8px'; n.textContent = t; b.after(n); setTimeout(() => n.remove(), 2400); };
  const markSeg = el => { [...el.parentElement.children].forEach(b => b.classList.toggle('on', b === el)); };

  const ACTS = {
    page: d => go(d.p),
    'io-step': d => { S[d.w + 'Step'] = +d.n; renderOV(); },
    io: d => go(+d.i === 0 ? 'export' : 'import'),
    'menu-at': (d, el) => { const win = $('#win-host'), r = el.getBoundingClientRect(), wr = win.getBoundingClientRect(), k = SCALE || 1; S.modal = null; S.menu = { k: d.k, x: (r.left - wr.left) / k, y: (r.bottom - wr.top) / k + 4 }; render(false); }, 'set-safe': d => { S.safe = d.t; S.menu = null; render(false); toast('安全级别：' + d.t, 'ok'); },
    'rail-menu': (d, el) => { const win = $('#win-host'), r = el.getBoundingClientRect(), wr = win.getBoundingClientRect(), k = SCALE || 1; S.modal = null; S.menu = { k: 'transfer', x: (r.right - wr.left) / k + 6, y: (r.top - wr.top) / k }; render(false); }, theme: () => cycleTheme(), 'set-theme': d => setTheme(d.k),
    'set-accent': d => { S.accent = d.k; render(false); }, scheme: d => viewT(() => { S.scheme = d.k; render(false); }),
    modal: d => openModal(d.m), palette: () => openModal('palette'), 'close-ov': () => closeOV(), bell: () => openModal('bell'), 'conn-pop': () => openModal('connpop'), 'db-pop': () => openModal('dbpop'),
    'go-welcome': () => go('welcome'), 'set-conn': d => { S.conn = d.id; S.modal = null; render(); toast(`已切换到 ${curConn().name}`, 'info'); },
    'conn-open': d => connect(d.id), side: d => { S.side = d.k; if (S.scheme === 'B') S.drawer = true; render(false); },
    'tab-x': d => { S.xtab = d.p; S.page = 'data'; S.modal = null; render(false); }, 'tab-pick': d => { if (d.p.startsWith('x:')) { S.xtab = d.p; S.page = 'data'; } else { S.xtab = null; S.page = d.p; } S.modal = null; render(false); }, 'tabs-close-others': () => { S.manyTabs = false; S.xtab = null; S.tabs = [S.page]; S.modal = null; render(false); toast('已关闭其他标签', 'info'); }, 'res-sort': d => { /* 查询结果点列头排序：先等一会儿（模拟慢查询），等待时列头和结果区顶部都有流光，和数据页加载一致 */
      if (S.rwait != null) return; const i = +d.i, cur = S.rsort && S.rsort[0] === i ? S.rsort : null;
      S.rwait = i; render(false); const box = $('#resbox'); if (box) box.insertAdjacentHTML('afterbegin', '<div class="loadbar"><i></i></div>');
      setTimeout(() => { S.rsort = [i, cur ? !cur[1] : false]; S.rwait = null; render(false); }, 1300);
    }, 'insp-tab': d => { S.inspTab = +d.i; render(false); }, 'side-toggle': () => { const sd = $('.side'); if (sd && !S.sideHidden && !S.reduce) { sd.classList.add('closing'); setTimeout(() => { S.sideHidden = true; render(false); }, 210); } else { S.sideHidden = !S.sideHidden; render(false); } }, drawer: () => { S.drawer = !S.drawer; render(false); },
    console: () => { S.console = !S.console; render(false); }, ai: () => dockTo('ai'), inspect: () => dockTo('inspect'),
    'new-tab': () => { S.tabsEnd = true; if (!S.tabs.includes('sql')) S.tabs.push('sql'); go('sql'); toast('已新建查询', 'info', 'shop-dev · ⌘T'); },
    'close-tab': d => { if (d.p.startsWith('x:')) { return; } S.xtab = null; const i = S.tabs.indexOf(d.p); S.tabs.splice(i, 1); if (S.page === d.p) S.page = S.tabs[Math.max(0, i - 1)] || 'welcome'; render(false); },
    'open-table': () => { S.rowSel = 3; go('data'); }, tog: d => { S.tree[d.k] = !S.tree[d.k]; render(false); },
    fbar: () => openModal('fpop'),
    'f-mode': d => { S.fmode = d.k; renderOV(); }, 'f-rm': d => { S.fconds.splice(+d.i, 1); renderOV(); }, 'f-rm-applied': d => { S.fconds.splice(+d.i, 1); if (!S.fconds.length) S.fapplied = false; render(false); toast('已移除筛选条件', 'info'); },
    'f-add': () => { S.fconds.push([['total', 'created_at', 'user_id'][S.fconds.length % 3], '=', '']); renderOV(); },
    'f-clear': () => { S.fapplied = false; S.modal = null; render(false); renderOV(); toast('已取消筛选', 'info'); },
    'f-apply': () => { S.fapplied = S.fconds.length > 0; S.modal = null; render(false); renderOV(); toast('已应用筛选', 'ok', filterWhere()); },
    'f-tosql': () => { S.modal = null; go('sql'); toast('已放进新查询', 'info', filterWhere()); },
    'rm-filter': () => { S.fapplied = false; render(false); toast('已移除过滤条件', 'info'); },
    refresh: () => { const g = $('#gridwrap'); if (g) { g.style.position = 'relative'; g.insertAdjacentHTML('afterbegin', '<div class="loadbar"><i></i></div>'); g.style.opacity = .55; setTimeout(() => { g.style.opacity = ''; toast('已刷新', 'ok', '12,431 行 · 6 ms'); }, 380); } },
    run: () => { const p = $('.sqlpane'); if (!p) return; const b = document.createElement('div'); b.className = 'indet'; b.style.cssText = 'position:absolute;left:0;right:0;top:0;height:2px;z-index:5'; p.style.position = 'relative'; p.appendChild(b); setTimeout(() => { b.remove(); toast('执行成功', 'ok', '12 ms · 4 行'); }, 620); },
    commit: () => { S.mods = []; S.pending = 0; render(false); toast('已提交 3 项修改', 'ok', 'UPDATE × 3 · 7 ms', true); }, discard: () => { (S.mods || []).reverse().forEach(x => { DB.orders[x.i][x.j] = x.old; }); S.mods = []; S.pending = 0; render(false); toast('已丢弃修改', 'info'); },
    'add-row': () => { S.pending += 1; render(false); toast('已新增一行（未提交）', 'info'); },
    toast: d => toast(d.msg || '完成', d.k || 'info'), 'toast-undo': () => { S.pending = 3; render(false); toast('已撤销提交', 'info'); },
    'menu-pick': d => { S.menu = null; if (d.t.includes('关联')) { go('data'); toast('已在新标签中打开 users #1042', 'info'); } else if (d.t === '在右侧预览') { S.dock = 'inspect'; render(false); } else { renderOV(); toast(d.t, 'info', '设计稿：此处执行对应操作'); } },
    'danger-drop': () => { S.menu = null; S.modal = 'confirm:drop'; renderOV(); }, 'danger-trunc': () => { S.menu = null; S.modal = 'confirm:trunc'; renderOV(); },
    'confirm-ok': d => { S.modal = null; renderOV(); toast(d.msg, 'ok'); },
    'pal-pick': d => { const it = S.palItems[+d.i]; S.modal = null; renderOV(); it && it.run(); }, 'pal-scope': d => { S.palScope = +d.i; S.palSel = 0; S._focusPal = true; renderOV(); },
    'set-tab': d => { S.setTab = d.k; renderOV(); }, ctab: d => { S.ctab = +d.i; S.test = 0; renderOV(); }, ctype: d => { S.ctype = +d.i; renderOV(); },
    'test-conn': () => { S.test = 1; renderOV(); setTimeout(() => { S.test = 2; renderOV(); }, 900); },
    reduce: () => { S.reduce = !S.reduce; document.documentElement.style.setProperty('--m', S.reduce ? 0 : 1); renderOV(); }, zebra: d => { S.zebra = +d.i === 1; render(false); renderOV(); },
    mon: d => { S.mon = +d.i; render(false); }, 'inline-ai': () => { S.inlineAI = !S.inlineAI; render(false); },
    'pick-logo': d => { S.logo = d.k; render(false); }, pick: d => { if (d.g === 'scheme') S.scheme = d.k; else if (d.g === 'logo') S.logo = d.k; else S.theme = d.k; render(false); },
    'go-modal': d => { S.page = 'data'; S.modal = d.m; S.menu = null; render(false); }, 'go-app': () => go('data'),
    ease: (d, el) => { markSeg(el); document.documentElement.style.setProperty('--ease', EASES[d.k]); }, reduce2: (d, el) => { el.classList.toggle('on'); S.reduce = el.classList.contains('on'); document.documentElement.style.setProperty('--m', S.reduce ? 0 : 1); const r = $('#mrange'); if (r) { r.value = S.reduce ? 0 : 1; $('#mv').textContent = r.value + '×'; } },
    replay: (d, el) => { const box = el.closest('.card').querySelector('.demo'); box.innerHTML = ''; requestAnimationFrame(() => { box.innerHTML = motionDemo(d.k); }); },
    'copy-verdict': () => { const t = $('#verdict').textContent; navigator.clipboard && navigator.clipboard.writeText(t); toastLocal('已复制：' + t); },
    sw: (d, el) => el.classList.toggle('on'), fmt: (d, el) => markSeg(el), chk: (d, el) => el.classList.toggle('on'), seg: (d, el) => markSeg(el), noop: (d, el) => markSeg(el), resizer: () => { },
  };
  document.addEventListener('click', e => {
    if (e.target.matches('.shot img')) { const l = document.createElement('div'); l.id = 'lightbox'; l.innerHTML = `<img src="${e.target.src}">`; l.onclick = () => l.remove(); document.body.appendChild(l); return; }
    const el = e.target.closest('[data-act]'); if (!el) return;
    if (el.dataset.act === 'close-ov' && e.target !== el && e.target.closest('.modal,.pal,.menu') && !e.target.closest('[data-act="close-ov"]')) return;
    const f = ACTS[el.dataset.act]; if (!f) return;
    if (el.dataset.nav) setTimeout(() => go(el.dataset.nav), 0);
    if (el.dataset.toast && el.dataset.act === 'close-ov') setTimeout(() => toast(el.dataset.toast), 0);
    f(el.dataset, el);
  });
  document.addEventListener('contextmenu', e => {
    const t = e.target.closest('[data-ctx]'); const win = $('#win-host'); if (!t || !win) return; e.preventDefault();
    const r = win.getBoundingClientRect(), k = SCALE || 1; let x = (e.clientX - r.left) / k, y = (e.clientY - r.top) / k;
    x = Math.min(x, 1280 - 230); y = Math.min(y, 800 - 260);
    if (t.dataset.ctx === 'row') { S.rowSel = +t.dataset.i; $$('#grid tr').forEach(tr => tr.classList.toggle('sel', tr === t)); }
    S.menu = { k: t.dataset.ctx, x, y }; renderOV();
  });

  /* ---------- 工具提示（小气泡）：见 styles/v3.css「悬浮提示」 ---------- */
  let tipEl, tipTimer, tipLast = 0;
  const tipHTML = t => {
    const m = (t.dataset.tip || '').match(/^(.*?)\s+((?:[⌘⌥⌃⇧⏎↵⌫⇥⎋]+[A-Za-z0-9,.\/\\=\-]*|Space|Esc)(?:\s?[⌘⌥⌃⇧]+[A-Za-z0-9]+)*)$/);
    const text = m ? m[1] : t.dataset.tip, keys = m ? m[2] : '', d = t.dataset.tipD;
    const kc = keys ? `<span class="tk">${[...keys.replace(/Space/, '␣')].map(k => `<i>${k}</i>`).join('')}</span>` : '';
    return d ? `<div class="tip rich"><b>${text}${keys ? ' ' + kc : ''}</b><span>${d}</span></div>` : `<div class="tip">${text}${kc}</div>`;
  };
  const hideTip = () => { clearTimeout(tipTimer); if (tipEl) { const e = tipEl; tipEl = null; e.classList.add('out'); setTimeout(() => e.remove(), 80); } };
  document.addEventListener('mouseover', e => {
    const t = e.target.closest('[data-tip]'); hideTip(); if (!t) return;
    const show = () => {
      const win = $('#win-host'); if (!win || !document.contains(t)) return;
      const r = t.getBoundingClientRect(), wr = win.getBoundingClientRect(), k = SCALE || 1;
      const h = document.createElement('div'); h.innerHTML = tipHTML(t); tipEl = h.firstChild; win.appendChild(tipEl);
      let x = (r.left - wr.left) / k + r.width / 2 / k - tipEl.offsetWidth / 2, y = (r.bottom - wr.top) / k + 6;
      if (y + tipEl.offsetHeight > 796) y = (r.top - wr.top) / k - tipEl.offsetHeight - 6; /* 放不下翻到上方 */
      tipEl.style.left = Math.max(6, Math.min(x, 1274 - tipEl.offsetWidth)) + 'px'; tipEl.style.top = y + 'px'; tipLast = Date.now();
    };
    if (Date.now() - tipLast < 300 || S.reduce) show(); else tipTimer = setTimeout(show, 450); /* 刚看过一个提示：不再等 */
  });
  document.addEventListener('mousedown', hideTip); document.addEventListener('scroll', hideTip, true);

  /* ---------- 数据网格：选中、编辑、待提交 ---------- */
  window.bindGrid = () => {
    const g = $('#grid'); if (!g) return;
    g.onclick = e => {
      const td = e.target.closest('td:not(.n)'), tr = e.target.closest('tr[data-i]'); if (!tr) return;
      $$('tr.sel', g).forEach(x => x.classList.remove('sel')); tr.classList.add('sel'); S.rowSel = +tr.dataset.i;
      $$('.cell-sel', g).forEach(x => x.classList.remove('cell-sel')); td && td.classList.add('cell-sel');
      if (S.dock === 'inspect') $('.rdock,.rdock-a,[style*="z-index:22"]').innerHTML = inspectDock();
    };
    g.ondblclick = e => {
      const td = e.target.closest('td:not(.n)'); if (!td || td.querySelector('input')) return;
      const old = td.innerHTML, txt = td.textContent.trim(); td.classList.add('editing');
      td.innerHTML = `<input class="cell-in" value="${txt === 'NULL' ? '' : txt}">`; const i = td.firstChild; i.focus(); i.select();
      const done = ok => {
        td.classList.remove('editing');
        if (ok && i.value !== txt) {
          const ri = +td.parentElement.dataset.i, cj = +td.dataset.j; (S.mods = S.mods || []).push({ i: ri, j: cj, old: DB.orders[ri][cj] }); DB.orders[ri][cj] = i.value; S.pending++;
          render(false); const f = $(`#grid tbody tr[data-i="${ri}"] td[data-j="${cj}"]`); if (f) f.classList.add('flash');
        } else td.innerHTML = old;
      };
      i.onkeydown = ev => { if (ev.key === 'Enter') { ev.preventDefault(); i.onblur = null; done(true); } if (ev.key === 'Escape') { i.onblur = null; done(false); } };
      i.onblur = () => done(true);
    };
  };

  /* ---------- ER：拖动表节点 ---------- */
  window.bindER = () => {
    $$('.ernode').forEach(n => {
      n.onmousedown = e => {
        const k = SCALE || 1, sx = e.clientX, sy = e.clientY, ox = n.offsetLeft, oy = n.offsetTop; n.style.zIndex = 5; n.classList.add('drag');
        const mv = ev => { n.style.left = ox + (ev.clientX - sx) / k + 'px'; n.style.top = oy + (ev.clientY - sy) / k + 'px'; n.dataset.x = parseFloat(n.style.left); n.dataset.y = parseFloat(n.style.top); drawER(); };
        const up = () => { removeEventListener('mousemove', mv); removeEventListener('mouseup', up); n.classList.remove('drag'); };
        addEventListener('mousemove', mv); addEventListener('mouseup', up);
      };
    });
  };

  /* ---------- SQL 面板分割条拖动 ---------- */
  document.addEventListener('mousedown', e => {
    const r = e.target.closest('[data-act="resizer"]'); if (!r) return; const pane = $('.sqlpane'), k = SCALE || 1, sy = e.clientY, h0 = pane.offsetHeight;
    const mv = ev => { S.sqlH = Math.max(90, Math.min(480, h0 + (ev.clientY - sy) / k)); pane.style.flex = `0 0 ${S.sqlH}px`; };
    const up = () => { removeEventListener('mousemove', mv); removeEventListener('mouseup', up); };
    addEventListener('mousemove', mv); addEventListener('mouseup', up);
  });

  /* ---------- 面板输入 ---------- */
  document.addEventListener('input', e => {
    if (e.target.id !== 'palq') return; S.q = e.target.value; S.palSel = 0; window.palRefresh ? palRefresh() : $('#pal-list').innerHTML = paletteList();
  });
  document.addEventListener('keydown', e => {
    const mod = e.metaKey || e.ctrlKey, k = e.key.toLowerCase();
    if (S.modal === 'palette') {
      const n = (S.palItems || []).length;
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); S.palSel = (S.palSel + (e.key === 'ArrowDown' ? 1 : n - 1)) % Math.max(n, 1); window.palRefresh ? palRefresh() : $('#pal-list').innerHTML = paletteList(); return; }
      if (e.key === 'Tab') { e.preventDefault(); S.palScope = ((S.palScope || 0) + 1) % 6; S.palSel = 0; renderOV(); return; }
      if (e.key === 'Enter') { e.preventDefault(); const it = S.palItems[S.palSel]; S.modal = null; renderOV(); it && it.run(); return; }
    }
    if (e.key === 'Escape' && (S.modal || S.menu)) { closeOV(); return; }
    const tag = (e.target.tagName || '').toLowerCase(); if (['input', 'textarea'].includes(tag)) return;
    if (mod && k === 'k') { e.preventDefault(); openModal('palette'); }
    else if (mod && k === ',') { e.preventDefault(); openModal('settings'); }
    else if (mod && k === 'i') { e.preventDefault(); ACTS.ai(); }
    else if (mod && k === 'b') { e.preventDefault(); S.scheme === 'B' ? ACTS.drawer() : ACTS['side-toggle'](); }
    else if (mod && k === 't') { e.preventDefault(); ACTS['new-tab'](); }
    else if (mod && k === 's' && S.pending) { e.preventDefault(); ACTS.commit(); }
    else if (!mod && !e.altKey && ['1', '2', '3'].includes(e.key)) { S.scheme = 'ABC'[+e.key - 1]; viewT(() => render(false)); }
    else if (!mod && k === 't') cycleTheme();
  });

  /* ---------- 控制条 / 页面导航（顶部）事件 ---------- */
  document.addEventListener('click', e => {
    const b = e.target.closest('[data-sch],[data-th],[data-ac],.pages .pg'); if (!b) return;
    if (b.dataset.sch) viewT(() => { S.scheme = b.dataset.sch; render(false); });
    else if (b.dataset.th) setTheme(b.dataset.th);
    else if (b.dataset.ac) { S.accent = b.dataset.ac; render(false); }
    else if (b.dataset.p) { S.page = b.dataset.p; S.modal = null; S.menu = null; if (PAGES[b.dataset.p].open) { Object.assign(S, { page: PAGES[b.dataset.p].open.page, dock: PAGES[b.dataset.p].open.dock }); } render(); scrollTo({ top: 0 }); }
  });
  /* 页面里的 rm 之类的演示动作 */
  addEventListener('load', () => { document.documentElement.style.setProperty('--m', S.reduce ? 0 : 1); render(false); });
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) S.reduce = true;
})();
