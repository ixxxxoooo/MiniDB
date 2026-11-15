/* ============================================================
   外壳与状态：S 是唯一状态；render() 重画窗口，overlay() 重画弹层。
   三个方案共用同一份页面内容（PAGES），只是外壳（chrome）不同。
   ============================================================ */
window.S = Object.assign({
  scheme: 'A', theme: 'light', accent: 'indigo', page: 'data', conn: 'shop-dev',
  tabs: ['overview', 'data', 'structure', 'sql', 'er'], side: 'objects', drawer: true, dock: null, console: false,
  tree: { tables: true, views: true }, pending: 0, logo: 'peak', zebra: false, fbar: true, reduce: false, inlineAI: false, mon: 0, io: 0, ac: 0, sqlH: 300, sideHidden: false, sel: null, rowSel: 3, filter: [], menu: null, modal: null, toasts: [],
}, JSON.parse(localStorage.getItem('minidb-v2') || '{}'));
S.dock = null; S.menu = null; S.modal = null; S.toasts = [];

window.PAGES = {};    // 由 pages-*.js 填充：{ group, label, icon, tab:{icon,label}, tools(), body(), status?, dock?, side? }
window.GROUPS = [['app', '应用'], ['sys', '设计系统']];
const $ = (s, r = document) => r.querySelector(s);
window.$ = $; window.$$ = (s, r = document) => [...r.querySelectorAll(s)];
window.save = () => localStorage.setItem('minidb-v2', JSON.stringify({ scheme: S.scheme, theme: S.theme, accent: S.accent, page: S.page, conn: S.conn, logo: S.logo, zebra: S.zebra, reduce: S.reduce }));
const curConn = () => DB.conns.find(c => c.id === S.conn) || DB.conns[6];
window.curConn = curConn;
const isProd = () => curConn().env === 'PROD';

/* ---------- 侧栏内容 ---------- */
function objects() {
  const T = DB.tables, open = S.tree;
  const row = (n, c, ic = 'Table', cls = 'ico-t', on) => `<div class="tr ${on ? 'on' : ''}" data-act="open-table" data-ctx="table" data-n="${n}">${icon(ic, 13, cls)}<span class="trunc">${n}</span><span class="cnt mono">${c == null ? '' : (c > 9999 ? (c / 1000).toFixed(1) + 'k' : c)}</span></div>`;
  return `<div class="sec-h" data-act="tog" data-k="tables" style="cursor:pointer">${icon('ChevronR', 11, 'chev ' + (open.tables ? 'open' : ''))}表 <span class="cnt" style="margin-left:auto;font-weight:500">${T.length}</span></div>
    ${open.tables ? `<div class="stagger">${T.map(([n, c]) => row(n, c, 'Table', 'ico-t', n === 'orders')).join('')}</div>` : ''}
    <div class="sec-h" data-act="tog" data-k="views" style="cursor:pointer;margin-top:4px">${icon('ChevronR', 11, 'chev ' + (open.views ? 'open' : ''))}视图 <span class="cnt" style="margin-left:auto;font-weight:500">${DB.views.length}</span></div>
    ${open.views ? DB.views.map(v => row(v, null, 'Eye', 'ico-v')).join('') : ''}`;
}
function queries() {
  return DB.saved.map(([f, items]) => `<div class="sec-h">${icon('ChevronR', 11, 'chev open')}${f}<span class="cnt" style="margin-left:auto">${items.length}</span></div>${items.map((q, i) => `<div class="tr ${f === '报表' && i === 1 ? 'on' : ''}" data-act="page" data-p="sql">${icon('File', 13, 'ico-q')}<span class="trunc">${q}</span></div>`).join('')}`).join('') +
    `<div style="padding:10px"><div class="muted" style="font-size:11px;line-height:1.5;padding:8px 10px;border-radius:var(--r);background:var(--surface-3)">在「历史」里右键语句，选「收藏」，就会出现在这里。</div></div>`;
}
function history() {
  let day = '';
  return DB.history.map(h => { const hd = h.day !== day ? `<div class="sec-h">${icon('ChevronR', 11, 'chev open')}${day = h.day}</div>` : ''; return hd + `<div class="tr" style="height:auto;padding:5px 8px;align-items:flex-start;flex-direction:column;gap:2px" data-act="page" data-p="sql"><div class="sql trunc" style="font-size:11px;width:100%;line-height:1.4">${hl(h.sql)}</div><div class="row gap6" style="font-size:10.5px"><span class="dot ${h.ok ? 'ok' : 'err'}" style="box-shadow:none"></span><span class="${h.ok ? 'muted' : ''}" style="${h.ok ? '' : 'color:var(--err)'}">${h.ok ? '成功' : '失败'}</span><span class="muted">${h.t} · ${h.d || h.r}</span></div></div>`; }).join('');
}
function sideBody() { return S.side === 'queries' ? queries() : S.side === 'history' ? history() : objects(); }

function connHeader(compact) {
  const c = curConn();
  return `<div class="conn-h" data-act="conn-pop" style="cursor:pointer">${tile(c)}<div class="col grow" style="min-width:0"><div class="row gap6"><span class="nm trunc">${c.name}</span>${tagOf(c.env)}</div><div class="sub trunc">${c.type} ${c.ver} · ${c.host.split(':')[0]}</div></div><button class="sw-btn">${icon('Swap', 13)}</button></div>
  ${compact ? '' : `<div class="dbsel" data-act="db-pop">${icon('Database', 13)}<span class="grow">shop</span><span class="chip ${c.ro ? 'warn' : ''}" style="height:16px">${icon(c.ro ? 'Lock' : 'ShieldCheck', 10)} ${c.ro ? '只读' : '提醒 2'}</span>${icon('ChevronD', 12)}</div>`}`;
}
function filterBox() { return `<div class="search" style="margin:0 10px 6px;height:26px">${icon('Filter', 13)}<input placeholder="筛选表…"><span class="kbd">/</span></div>`; }

/* ---------- 连接树（方案 C：多连接同屏） ---------- */
function connTree() {
  const folders = ['生产', '测试', '本地'];
  return folders.map(f => `<div class="sec-h">${icon('ChevronR', 11, 'chev open')}${f}</div>` + DB.conns.filter(c => c.folder === f).map(c => {
    const on = c.id === S.conn;
    const kids = on ? `<div style="margin-left:14px;border-left:1px solid var(--line);padding-left:4px">
      <div class="tr">${icon('ChevronR', 11, 'chev open')}${icon('Database', 12)}<span>shop</span><span class="cnt">9</span></div>
      <div style="margin-left:14px" class="stagger">${DB.tables.map(([n, k]) => `<div class="tr ${n === 'orders' ? 'on' : ''}" data-act="open-table" data-ctx="table" data-n="${n}" style="height:22px">${icon('Table', 12, 'ico-t')}<span>${n}</span><span class="cnt mono">${k > 9999 ? (k / 1000).toFixed(1) + 'k' : k}</span></div>`).join('')}</div></div>` : '';
    return `<div class="tr ${on ? 'on' : ''}" data-act="set-conn" data-id="${c.id}" style="height:24px">${icon('ChevronR', 11, 'chev ' + (on ? 'open' : ''))}${tile(c, 'sm')}<span class="trunc">${c.name}</span>${c.open ? '<span class="dot ok" style="margin-left:auto;box-shadow:none"></span>' : ''}${tagOf(c.env)}</div>` + kids;
  }).join('')).join('');
}

/* ---------- 顶部标签 ---------- */
function tabLabel(id) { const p = PAGES[id]; const t = p.tab || {}; return { icon: t.icon || p.icon, label: t.label || p.label, mod: id === 'data' && S.pending > 0 }; }
function tabsHTML(kind) {
  return S.tabs.map(id => {
    const t = tabLabel(id), on = id === S.page;
    if (kind === 'ptab') return `<button class="ptab ${on ? 'on' : ''}" data-act="page" data-p="${id}">${icon(t.icon, 13)}${t.label}${t.mod ? '<i class="mod"></i>' : ''}</button>`;
    if (kind === 'ctab') return `<span class="stab ${on ? 'on' : ''}" data-act="page" data-p="${id}">${icon(t.icon, 12)}${t.label}${t.mod ? '<i class="mod" style="width:6px;height:6px;border-radius:50%;background:var(--warn)"></i>' : ''}<span class="x" data-act="close-tab" data-p="${id}">${icon('X', 10)}</span></span>`;
    return `<span class="dtab ${on ? 'on' : ''}" data-act="page" data-p="${id}">${icon(t.icon, 13)}<span class="trunc">${t.label}</span>${t.mod ? '<i class="mod"></i>' : ''}<span class="x" data-act="close-tab" data-p="${id}">${icon('X', 11)}</span></span>`;
  }).join('');
}

const RAIL = [['objects', 'Database', '对象'], ['queries', 'File', '查询'], ['history', 'History', '历史'], '-', ['p:overview', 'Layers', '项目概览'], ['p:er', 'Network', 'ER 图'], ['p:monitor', 'Activity', '监控'], ['p:export', 'Download', '导入导出']];
function railHTML(cls) {
  return RAIL.map(r => r === '-' ? `<div class="dsep ${cls}" style="width:18px;height:1px;background:var(--line);margin:4px 0"></div>` : (() => {
    const [k, ic, lb] = r, isP = k.startsWith('p:'), on = isP ? S.page === k.slice(2) : (S.side === k && !(S.scheme === 'B' && !S.drawer));
    return `<button class="rbtn ${on ? 'on' : ''}" data-act="${isP ? 'page' : 'side'}" data-p="${isP ? k.slice(2) : ''}" data-k="${k}" data-tip="${lb}">${icon(ic, 17)}</button>`;
  })()).join('');
}
const themeIcon = () => ({ light: 'Sun', dark: 'Moon', paper: 'File', aurora: 'Sparkle' })[S.theme];

/* ---------- 三个方案的窗口 ---------- */
function toolsOf(p) { return p.tools ? p.tools() : ''; }
function statusItems(p) { return p.status ? p.status() : `<span class="it"><i class="dot ok"></i>已连接 · 4 ms</span>`; }

function chromeA(p, body) {
  const c = curConn();
  return `<div class="win A" id="win">${isProd() ? '<div class="envband"></div>' : ''}<div class="traffic"><i></i><i></i><i></i></div>
  <div class="top"><button class="btn ghost icon" data-act="side-toggle" data-tip="侧栏">${icon('Sidebar', 15)}</button>
    <div class="tabs-a">${tabsHTML('dtab')}<button class="newtab" data-act="new-tab" data-tip="新建查询 ⌘T">${icon('Plus', 14)}</button></div>
    <div class="pill-search" data-act="palette">${icon('Search', 13)}<span>搜索表、命令…</span><span class="kbd">⌘K</span></div>
    <button class="btn ghost icon" data-act="console" data-tip="控制台">${icon('Console', 15)}</button>
    <button class="btn ghost icon ${S.dock === 'ai' ? 'on' : ''}" data-act="ai" data-tip="AI 助手 ⌘I">${icon('Sparkle', 15)}</button>
    <button class="btn ghost icon" style="position:relative" data-act="bell" data-tip="通知">${icon('Bell', 15)}<i class="dot acc" style="position:absolute;margin:-8px -8px 0 0;right:9px;top:8px;box-shadow:0 0 0 2px var(--surface-3)"></i></button></div>
  <div class="body"><div class="rail">${railHTML('')}<div class="sp"></div>
      <button class="rbtn" data-act="theme" data-tip="切换主题">${icon(themeIcon(), 17)}</button><button class="rbtn" data-act="modal" data-m="settings" data-tip="设置 ⌘,">${icon('Settings', 17)}</button></div>
    ${(p.noSide || S.sideHidden) ? '' : `<div class="side">${connHeader()}${filterBox()}<div class="scroll grow" style="padding:0 6px 6px">${sideBody()}</div>
      <div class="sidefoot"><button class="btn ghost sm" data-act="modal" data-m="newtable">${icon('Plus', 13)}新建表</button><div class="sp"></div><button class="btn ghost icon sm" data-tip="导入">${icon('Download', 13)}</button><button class="btn ghost icon sm">${icon('More', 13)}</button></div></div>`}
    <div class="main">${p.noTools ? '' : `<div class="toolbar">${toolsOf(p)}</div>`}<div class="pgwrap pgbody" id="pg">${body}</div></div>
    ${S.dock ? `<div class="rdock-a">${dockHTML()}</div>` : ''}</div>
  <div class="status">${statusItems(p)}<div class="sp"></div><span class="it">${icon('Clock', 11)}事务：自动提交</span><span class="it"><i class="dot ${isProd() ? 'err' : 'ok'}"></i>${c.env}</span><span class="it">${DB.tables.length} 张表</span></div></div>`;
}

function chromeB(p, body) {
  const c = curConn();
  return `<div class="win B ${S.drawer ? 'drawer-open' : ''}" id="win">${isProd() ? '<div class="envband"></div>' : ''}<div class="traffic" style="height:50px"><i></i><i></i><i></i></div>
  <div class="top"><div class="mid"><div class="ptabs">${tabsHTML('ptab')}<button class="ptab" data-act="new-tab" style="padding:0 9px">${icon('Plus', 14)}</button></div></div>
    <div class="right"><div class="pill-search bsearch glass" data-act="palette">${icon('Search', 13)}<span>搜索或输入命令</span><span class="kbd">⌘K</span></div>
      <button class="btn ghost icon ${S.dock === 'ai' ? 'on' : ''}" data-act="ai" data-tip="AI 助手">${icon('Sparkle', 15)}</button><button class="btn ghost icon" data-act="bell">${icon('Bell', 15)}</button><div class="avatar" data-act="modal" data-m="settings">LW</div></div></div>
  <div class="dock glass">${railHTML('')}<div class="dsep"></div><button class="rbtn" data-act="theme" data-tip="主题">${icon(themeIcon(), 17)}</button><button class="rbtn" data-act="modal" data-m="settings">${icon('Settings', 17)}</button></div>
  ${S.drawer && !p.noSide ? `<div class="drawer glass">${connHeader(true)}<div style="display:flex;gap:6px;padding:0 10px 6px"><div class="search grow" style="border-radius:99px">${icon('Search', 13)}<input placeholder="在 ${c.name} 里搜索…"></div></div><div class="scroll grow" style="padding:0 6px">${sideBody()}</div><div class="sidefoot"><button class="btn ghost sm" data-act="modal" data-m="newtable">${icon('Plus', 13)}新建表</button><div class="sp"></div><button class="btn ghost icon sm" data-act="drawer">${icon('Sidebar', 13)}</button></div></div>` : ''}
  <div class="card-main" style="${S.drawer && !p.noSide ? '' : 'left:70px'}">${p.noTools ? '' : `<div class="toolbar">${toolsOf(p)}</div>`}<div class="pgwrap pgbody" id="pg">${body}</div>${S.dock ? '' : ''}</div>
  ${S.dock ? `<div class="glass" style="position:absolute;right:22px;top:62px;bottom:68px;width:320px;border-radius:20px;z-index:22;display:flex;flex-direction:column;overflow:hidden;animation:slide-r var(--dur-3) var(--ease) both">${dockHTML()}</div>` : ''}
  <div class="island glass"><span class="it row gap6"><i class="dot ok pulse"></i>${c.name} · 4 ms</span>${p.status ? p.status() : ''}<span class="row gap4"><button class="btn ghost sm" data-act="console">${icon('Console', 13)}控制台</button></span></div></div>`;
}

function chromeC(p, body) {
  const c = curConn();
  const open = DB.conns.filter(x => x.open);
  return `<div class="win C" id="win">${isProd() ? '<div class="envband"></div>' : ''}<div class="traffic"><i></i><i></i><i></i></div>
  <div class="top">${open.map(x => `<span class="ctab ${x.id === S.conn ? 'on' : ''}" data-act="set-conn" data-id="${x.id}"><i class="cdot" style="background:${x.color}"></i>${x.name}${tagOf(x.env)}</span>`).join('')}<button class="newtab" style="align-self:center" data-act="modal" data-m="conn">${icon('Plus', 14)}</button>
    <div class="right"><div class="pill-search" data-act="palette" style="min-width:210px">${icon('Search', 13)}<span>转到…</span><span class="kbd">⌘K</span></div>
      <button class="btn ghost icon sm" data-act="console">${icon('Console', 14)}</button><button class="btn ghost icon sm ${S.dock === 'ai' ? 'on' : ''}" data-act="ai">${icon('Sparkle', 14)}</button><button class="btn ghost icon sm" data-act="theme">${icon(themeIcon(), 14)}</button><button class="btn ghost icon sm" data-act="modal" data-m="settings">${icon('Settings', 14)}</button></div></div>
  <div class="tool"><div class="crumb">${icon('Server', 12)}<b>${c.name}</b>${icon('ChevronR', 10)}${icon('Database', 12)}shop${icon('ChevronR', 10)}${icon('Table', 12)}<b>${tabLabel(S.page).label}</b></div><span class="vr"></span>${toolsOf(p)}</div>
  <div class="body">${(p.noSide || S.sideHidden) ? '' : `<div class="side"><div class="row gap4" style="padding:6px 8px">${[['objects', 'Database'], ['queries', 'File'], ['history', 'History']].map(([k, i]) => `<button class="btn icon sm ${S.side === k ? 'on' : 'ghost'}" data-act="side" data-k="${k}">${icon(i, 13)}</button>`).join('')}<div class="sp"></div><button class="btn ghost icon sm" data-act="page" data-p="overview" data-tip="概览">${icon('Layers', 13)}</button><button class="btn ghost icon sm" data-act="page" data-p="er">${icon('Network', 13)}</button><button class="btn ghost icon sm" data-act="page" data-p="monitor">${icon('Activity', 13)}</button></div>
      ${filterBox()}<div class="scroll grow" style="padding:0 6px 6px">${S.side === 'objects' ? connTree() : sideBody()}</div></div>`}
    <div class="center"><div class="strip">${tabsHTML('ctab')}</div><div class="pgwrap pgbody" id="pg">${body}</div>${S.console ? consoleHTML() : ''}</div>
    ${S.dock ? `<div class="rdock">${dockHTML()}</div>` : ''}</div>
  <div class="status ${isProd() ? 'prod' : ''}"><span class="it">${icon('Plug', 11)}${c.name}</span>${statusItems(p)}<div class="sp"></div><span class="it">UTF-8</span><span class="it">自动提交</span><span class="it">${c.env}</span></div></div>`;
}

function consoleHTML() {
  return `<div class="bdock" style="${S.scheme === 'C' ? '' : ''}"><div class="row" style="height:28px;padding:0 10px;gap:12px;border-bottom:1px solid var(--line);background:var(--surface-2)"><div class="tabs" style="height:100%"><button class="on">控制台</button><button>消息</button><button>历史</button></div><div class="sp"></div><button class="btn ghost icon sm" data-act="console">${icon('X', 12)}</button></div>
    <div class="scroll grow mono" style="padding:6px 10px;font-size:11.5px;line-height:1.7;color:var(--fg-2)">${[['10:24:07', 'SELECT status, COUNT(*) FROM orders GROUP BY status', '12 ms · 5 行', 1], ['10:24:03', 'BEGIN', '', 1], ['10:19:41', "UPDATE orders SET status='shipped' WHERE id IN (10231, 10230)", '3 ms · 2 行', 1], ['10:12:09', 'SELECT foo FROM orders LIMIT 5', "Unknown column 'foo'", 0], ['10:12:02', 'COMMIT', '', 1]].map(l => `<div class="row gap8"><span class="muted">${l[0]}</span><span class="dot ${l[3] ? 'ok' : 'err'}" style="box-shadow:none"></span><span class="trunc" style="color:var(--fg)">${l[1]}</span><span class="muted" style="margin-left:auto;white-space:nowrap;${l[3] ? '' : 'color:var(--err)'}">${l[2]}</span></div>`).join('')}</div></div>`;
}

/* 右侧停靠：行预览 / AI */
function dockHTML() { return S.dock === 'ai' ? window.aiDock() : window.inspectDock(); }

window.render = function (anim) {
  const p = PAGES[S.page]; if (!p) return;
  const root = $('#app');
  root.dataset.scheme = S.scheme; root.dataset.theme = S.theme; root.dataset.accent = S.accent;
  document.documentElement.dataset.theme = S.theme; document.documentElement.dataset.accent = S.accent; document.documentElement.dataset.scheme = S.scheme;
  const sysPage = p.group === 'sys';
  $('#stage').style.display = sysPage ? 'none' : ''; $('#doc').style.display = sysPage ? '' : 'none';
  $$('.pages .pg').forEach(b => b.classList.toggle('on', b.dataset.p === S.page));
  $$('[data-sch]').forEach(b => b.classList.toggle('on', b.dataset.sch === S.scheme)); $$('[data-th]').forEach(b => b.classList.toggle('on', b.dataset.th === S.theme)); $$('[data-ac]').forEach(b => b.classList.toggle('on', b.dataset.ac === S.accent));
  if (sysPage) { $('#doc').innerHTML = `<div class="page-in">${p.body()}</div>`; if (p.after) p.after(); fit(); return; }
  if (p.tabPage !== false && !S.tabs.includes(S.page) && !p.noTab) S.tabs.push(S.page);
  const body = p.body();
  $('#winmount').innerHTML = p.chrome ? p.chrome() : (S.scheme === 'A' ? chromeA : S.scheme === 'B' ? chromeB : chromeC)(p, body);
  if (anim === false) { const pg = $('#pg'); if (pg) pg.style.animation = 'none'; }
  if (p.after) p.after();
  renderOV(); fit(); save();
};
window.fit = function () {
  const k = Math.min(1, (innerWidth - 40) / 1280), inner = $('.stage-in');
  inner.style.width = 1280 * k + 'px'; inner.style.height = 800 * k + 'px';
  $('#win-host').style.cssText = `width:1280px;height:800px;transform-origin:top left;transform:scale(${k})`;
  window.SCALE = k;
};
addEventListener('resize', () => fit());
