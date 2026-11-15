/* 页面：欢迎页（三种形态）、数据、行预览、结构、概览 */
(function () {
  const seg = (items, on, act = 'seg') => `<div class="seg">${items.map((t, i) => `<button class="${i === on ? 'on' : ''}" data-act="${act}" data-i="${i}">${t}</button>`).join('')}</div>`;
  window.seg = seg;
  const viewTabs = (on) => `<div class="vseg">${[['数据', 'data'], ['结构', 'structure'], ['DDL', 'ddl'], ['文档', 'doc']].map(([t, p], i) => `<button class="${i === on ? 'on' : ''}" data-act="page" data-p="${p}">${t}</button>`).join('')}</div>`;
  window.viewTabs = viewTabs;

  /* ---------------- 欢迎页 ---------------- */
  const card = (c, big) => `<div class="ccard card" data-act="conn-open" data-id="${c.id}" data-ctx="conn">
    <div class="row gap8">${tile(c, 'lg')}<div class="col grow" style="min-width:0"><b class="trunc" style="font-size:13px">${c.name}</b><span class="muted trunc" style="font-size:11px">${c.type} ${c.ver}</span></div>${tagOf(c.env)}</div>
    <div class="mono trunc dim" style="font-size:11px;margin-top:9px">${c.host}${c.ro ? ' · 只读' : ''}${c.ssh ? ' · SSH' : ''}</div>
    <div class="row gap6" style="margin-top:7px;font-size:11px"><span class="dot ${c.open ? 'ok' : ''}"></span><span class="muted">${c.open ? '已连接' : c.used}</span>${c.pin ? `<span class="muted" style="margin-left:auto">${icon('Pin', 11)}</span>` : ''}<span class="go btn primary sm" style="margin-left:auto">连接 <span class="kbd">⏎</span></span></div></div>`;
  const group = (f, big) => `<div class="eyebrow" style="margin:14px 0 8px;display:flex;gap:6px;align-items:center">${icon('ChevronD', 11)}${f}<span style="font-weight:500">${DB.conns.filter(c => c.folder === f).length}</span></div><div class="cgrid stagger">${DB.conns.filter(c => c.folder === f).map(c => card(c)).join('')}</div>`;

  function welcomeA() {
    return `<div class="win A" id="win"><div class="traffic"><i></i><i></i><i></i></div>
    <div class="top" style="justify-content:center;padding:0">${LOGO.lockup(S.logo || 'peak', 17)}<div style="position:absolute;right:10px"><button class="btn ghost icon" data-act="theme">${icon(({ light: 'Sun', dark: 'Moon', paper: 'File', aurora: 'Sparkle' })[S.theme], 15)}</button><button class="btn ghost icon" data-act="modal" data-m="settings">${icon('Settings', 15)}</button></div></div>
    <div class="body"><div class="side" style="width:216px;padding:10px 8px;gap:2px">
      <div class="tr on">${icon('Plug', 14)}全部连接<span class="cnt">9</span></div><div class="tr">${icon('Clock', 14)}最近使用</div><div class="tr">${icon('Pin', 14)}已固定<span class="cnt">2</span></div>
      <div class="row" style="padding:12px 8px 4px"><span class="eyebrow grow">文件夹</span><button class="btn ghost icon sm">${icon('Plus', 12)}</button></div>
      ${['生产', '测试', '本地'].map((f, i) => `<div class="tr">${icon('Folder', 14, '')}${f}<span class="cnt">3</span></div>`).join('')}
      <div class="sp"></div><div class="card" style="padding:10px 12px;background:var(--surface-3);box-shadow:none"><b style="font-size:11.5px">提示</b><div class="muted" style="font-size:11px;line-height:1.5;margin-top:3px">把「生产」库放进「生产」文件夹，连接后窗口顶部会出现红色环境带。</div></div></div>
    <div class="main" style="background:var(--surface)"><div class="row gap8" style="padding:16px 22px 4px"><div class="h1 grow">连接</div>
      <div class="search" style="width:240px">${icon('Search', 13)}<input placeholder="搜索连接…"><span class="kbd">⌘F</span></div>${seg(['卡片', '列表'], 0)}<button class="btn">${icon('Upload', 13)}导入</button><button class="btn primary" data-act="modal" data-m="conn">${icon('Plus', 13)}新建连接<span class="kbd">⌘N</span></button></div>
      <div class="scroll grow" style="padding:0 22px 14px">${['生产', '测试', '本地'].map(group).join('')}</div>
      <div class="status" style="background:var(--surface-2)"><span>双击或 ⏎ 连接 · 右键更多 · 拖动移入文件夹</span><div class="sp"></div><span>9 个连接 · 密码保存在系统钥匙串</span></div></div></div></div>`;
  }
  function welcomeB() {
    const pins = DB.conns.filter(c => c.pin), rec = DB.conns.slice(0, 5);
    return `<div class="win B" id="win"><div class="traffic" style="height:50px"><i></i><i></i><i></i></div><div class="top"><div class="right"><button class="btn ghost icon" data-act="theme">${icon(({ light: 'Sun', dark: 'Moon', paper: 'File', aurora: 'Sparkle' })[S.theme], 15)}</button><button class="btn ghost icon" data-act="modal" data-m="settings">${icon('Settings', 15)}</button></div></div>
    <div class="scroll grow" style="padding:0 0 30px"><div style="max-width:760px;margin:6px auto 0;padding:0 24px" class="page-in">
      <div class="col" style="align-items:center;gap:14px;margin-top:6px">${LOGO.glyph(S.logo || 'peak', 54, (S.logo || 'peak') === 'peak' ? { tile: true } : {})}<div class="h1" style="font-size:26px;letter-spacing:-.03em">晚上好，今天连哪个库？</div>
      <div class="glass search" style="width:100%;height:46px;border-radius:99px;padding:0 18px;font-size:14px;gap:10px" data-act="palette">${icon('Search', 17)}<span class="grow muted">搜索连接，或粘贴 mysql:// 连接串直接新建</span><span class="kbd">⌘K</span></div></div>
      <div class="eyebrow" style="margin:26px 0 10px">已固定</div><div class="row gap12">${pins.map(c => `<div class="card glass pin-t" data-act="conn-open" data-id="${c.id}" data-ctx="conn"><div class="row">${tile(c, 'xl')}<span style="margin-left:auto">${tagOf(c.env)}</span></div><b style="margin-top:12px;font-size:14px">${c.name}</b><span class="muted" style="font-size:11.5px">${c.open ? '已连接' : c.used} · ${c.type}</span></div>`).join('')}<div class="card pin-t add" data-act="modal" data-m="conn" style="border-style:dashed;align-items:center;justify-content:center;color:var(--fg-3);box-shadow:none;background:transparent">${icon('Plus', 20)}<span style="margin-top:6px">新建连接</span></div></div>
      <div class="eyebrow" style="margin:26px 0 8px">最近使用</div><div class="card glass" style="padding:4px;border-radius:var(--r-xl)">${rec.map(c => `<div class="tr" style="height:44px;border-radius:14px;padding:0 12px;gap:12px" data-act="conn-open" data-id="${c.id}" data-ctx="conn">${tile(c, 'lg')}<div class="col grow" style="min-width:0"><b style="color:var(--fg)">${c.name}</b><span class="muted mono trunc" style="font-size:11px">${c.host}</span></div>${tagOf(c.env)}<span class="muted" style="width:70px;text-align:right;font-size:11.5px">${c.open ? '<span class="chip ok">已连接</span>' : c.used}</span>${icon('ArrowRight', 14)}</div>`).join('')}</div>
      <div class="row gap8 muted" style="margin-top:18px;justify-content:center;font-size:11.5px">${icon('Folder', 13)} 全部 9 个连接 · 3 个文件夹 <button class="btn ghost sm" style="margin-left:6px">查看全部</button></div></div></div></div>`;
  }
  function welcomeC() {
    const sel = DB.conns[0];
    return `<div class="win C" id="win"><div class="traffic"><i></i><i></i><i></i></div><div class="top"><span class="ctab on">${icon('Plug', 12)}连接管理</span><div class="right"><button class="btn ghost icon sm" data-act="theme">${icon(({ light: 'Sun', dark: 'Moon', paper: 'File', aurora: 'Sparkle' })[S.theme], 14)}</button><button class="btn ghost icon sm" data-act="modal" data-m="settings">${icon('Settings', 14)}</button></div></div>
    <div class="tool"><div class="search" style="width:260px;height:24px">${icon('Search', 12)}<input placeholder="过滤连接  类型:mysql  环境:prod"><span class="kbd">⌘F</span></div><span class="vr"></span><button class="btn sm primary" data-act="modal" data-m="conn">${icon('Plus', 12)}新建</button><button class="btn sm">${icon('Upload', 12)}导入</button><button class="btn sm">${icon('Folder', 12)}新建文件夹</button><div class="sp"></div>${seg(['列表', '卡片'], 0)}</div>
    <div class="body"><div class="side" style="width:188px"><div class="sec-h" style="margin-top:6px">视图</div><div class="tr on">${icon('Plug', 13)}全部<span class="cnt">9</span></div><div class="tr">${icon('Clock', 13)}最近使用</div><div class="tr">${icon('Pin', 13)}已固定<span class="cnt">2</span></div><div class="tr">${icon('Zap', 13)}已连接<span class="cnt">2</span></div>
      <div class="sec-h" style="margin-top:8px">环境</div>${[['PROD', 3], ['STG', 3], ['DEV', 2], ['LOCAL', 1]].map(([e, n]) => `<div class="tr"><span style="width:13px">${tagOf(e).replace('tag ', 'tag ').replace('height:16px', '')}</span><span style="margin-left:34px">${n}</span></div>`).join('')}</div>
    <div class="center scroll"><table class="grid"><thead><tr><th class="n">#</th><th>名称</th><th>类型</th><th>地址</th><th>环境</th><th>最近使用</th><th>状态</th></tr></thead><tbody>
      ${DB.conns.map((c, i) => `<tr class="${i === 0 ? 'sel' : ''}" data-act="conn-open" data-id="${c.id}" data-ctx="conn"><td class="n">${i + 1}</td><td><span class="row gap6">${tile(c, 'sm')}<b>${c.name}</b>${c.pin ? icon('Pin', 11) : ''}</span></td><td>${c.type} <span class="muted">${c.ver}</span></td><td class="mono">${c.host}${c.ssh ? ' <span class="chip">SSH</span>' : ''}${c.ro ? ' <span class="chip warn">只读</span>' : ''}</td><td>${tagOf(c.env)}</td><td class="muted">${c.used}</td><td>${c.open ? '<span class="st"><i style="background:var(--ok)"></i>已连接</span>' : '<span class="muted">—</span>'}</td></tr>`).join('')}</tbody></table></div>
    <div class="rdock" style="width:250px"><div class="pad col gap8" style="padding:14px">${tile(sel, 'xl')}<div class="h2">${sel.name}</div><div class="muted">${sel.type} ${sel.ver}</div>
      <div class="hr" style="margin:6px 0"></div>${[['主机', sel.host], ['用户', 'app_rw'], ['默认库', 'shop'], ['SSH', 'bastion-01 · 密钥'], ['安全级别', '提醒 2']].map(([k, v]) => `<div class="row" style="font-size:11.5px"><span class="muted" style="width:62px">${k}</span><span class="mono trunc">${v}</span></div>`).join('')}
      <div class="row gap6" style="margin-top:8px"><button class="btn primary grow" data-act="conn-open" data-id="${sel.id}">连接 <span class="kbd">⏎</span></button><button class="btn">编辑</button></div></div></div></div>
    <div class="status"><span class="it">9 个连接</span><span class="it">已选 1</span><div class="sp"></div><span class="it">⏎ 连接 · ⌘E 编辑 · ⌘D 复制 · ⌫ 删除</span></div></div>`;
  }
  PAGES.welcome = { group: 'app', label: '欢迎页 · 连接管理', icon: 'Plug', noTab: true, tabPage: false, chrome: () => ({ A: welcomeA, B: welcomeB, C: welcomeC })[S.scheme](), body: () => '' };

  /* ---------------- 数据 ---------------- */
  function gridHTML(rows = DB.orders, o = {}) {
    const cols = DB.cols;
    const stc = { paid: 'ok', pending: 'warn', shipped: 'info', refunded: 'err' };
    const sc = { paid: 'var(--ok)', pending: 'var(--warn)', shipped: 'var(--info)', refunded: 'var(--err)' };
    return `<table class="grid ${S.zebra ? 'zebra' : ''}" id="grid"><thead><tr><th class="n">#</th>${cols.map(c => `<th class="${c.cls && c.cls.includes('r') ? 'r' : ''}"><span class="tyb ${tyKind(c.t)}">${tyLabel(c.t)}</span><span class="cn">${c.n}</span>${c.k ? `<span class="ky">${icon('Key', 11)}</span><span class="so">${icon('ArrowUp', 11)}</span>` : ''}</th>`).join('')}</tr></thead><tbody>
    ${rows.map((r, i) => `<tr data-i="${i}" class="${i === S.rowSel ? 'sel' : ''}" data-ctx="row"><td class="n">${i + 1}</td>${r.map((v, j) => {
      const c = cols[j]; let cl = c.cls || '', inner = v;
      if (v === null) { cl += ' null'; inner = 'NULL'; } else if (c.n === 'status') inner = `<span class="st"><i style="background:${sc[v]}"></i>${v}</span>`; else if (c.n === 'total') inner = Number(v).toLocaleString('en', { minimumFractionDigits: 2 });
      return `<td class="${cl}${(S.mods || []).some(m => m.i === i && m.j === j) ? ' mod' : ''}" data-j="${j}" title="">${inner}</td>`;
    }).join('')}</tr>`).join('')}</tbody></table>`;
  }
  window.gridHTML = gridHTML;
  const tyKind = t => /int|serial/.test(t) ? 'int' : /dec|float|double|num/.test(t) ? 'flt' : /date|time/.test(t) ? 'time' : 'txt';
  const tyLabel = t => ({ int: '123', flt: '1.0', time: icon('Clock', 11), txt: 'Aa' })[tyKind(t)];
  window.filterWhere = () => 'WHERE ' + S.fconds.map(c => `${c[0]} ${c[1]} ${/^-?\d+(\.\d+)?$/.test(c[2]) ? c[2] : "'" + c[2] + "'"}`).join(S.fmode === 'or' ? ' OR ' : ' AND ');
  const filterPop = () => `<div class="scrim" style="background:transparent;backdrop-filter:none;place-items:start" data-act="close-ov"><div class="card fpop" style="left:${S.scheme === 'A' ? 292 : 120}px;top:${S.scheme === 'A' ? 96 : 116}px" onclick="event.stopPropagation()">
      <div class="fh"><b>筛选</b><span class="muted">${S.fmode === 'or' ? '满足任一条件' : '满足全部条件'}</span><span class="sp"></span><div class="seg" id="fmode">${[['and', '且'], ['or', '或']].map(([k, t]) => `<button class="${S.fmode === k ? 'on' : ''}" data-act="f-mode" data-k="${k}" style="height:24px;padding:0 12px">${t}</button>`).join('')}</div></div>
      ${S.fconds.map((c, i) => `<div class="frow"><div class="fsel">${c[0]}<span class="sp"></span>${icon('ChevUD', 12)}</div><div class="fsel">${c[1]}<span class="sp"></span>${icon('ChevUD', 12)}</div><div class="fsel val">${c[2]}</div><span class="rm" data-act="f-rm" data-i="${i}">${icon('X', 13)}</span></div>`).join('')}
      <div class="fadd" data-act="f-add">${icon('Plus', 13)}再加一条</div>
      <div class="fsql sql">${hl(filterWhere())}</div>
      <div class="ff"><span class="lnk" data-act="f-tosql">${icon('Code', 13)}把筛选放进新查询</span><span class="sp"></span><button class="btn ghost" data-act="f-clear">取消筛选</button><button class="btn primary" data-act="f-apply">应用 <span class="kbd">⏎</span></button></div></div></div>`;
  window.filterPop = filterPop;
  function filterBar() {
    return `<div class="row gap6" style="padding:6px 10px;border-bottom:1px solid var(--line);background:var(--surface-2);animation:slide-up var(--dur-2) var(--ease)">${icon('Filter', 13)}
      <div class="fchip">status <b>=</b> paid <span data-act="rm-filter">${icon('X', 10)}</span></div><div class="fchip">total <b>&gt;</b> 100 <span>${icon('X', 10)}</span></div>
      <button class="btn ghost sm">${icon('Plus', 12)}添加条件</button><div class="sp"></div><span class="muted" style="font-size:11px">匹配 8,214 行</span><button class="btn sm primary">应用 <span class="kbd">⏎</span></button></div>`;
  }
  const pendingBar = () => S.pending > 0 ? `<div class="islandbar" id="pending"><span class="dot warn"></span><b>${S.pending} 项待提交</b><span class="sub">改 ${S.pending} · 增 0 · 删 0</span><span class="vr" style="background:rgba(255,255,255,.2)"></span><button class="btn ghost sm" data-act="modal" data-m="sqlprev">${icon('Code', 12)}预览 SQL</button><button class="btn ghost sm" data-act="discard">丢弃</button><button class="btn primary sm" data-act="commit">${icon('Check', 12)}提交 <span class="kbd">⌘S</span></button></div>` : '';
  window.pendingBar = pendingBar;
  PAGES.data = {
    group: 'app', label: '数据', icon: 'Table', tab: { icon: 'Table', label: 'orders' },
    tools: () => `${viewTabs(0)}${S.fapplied && S.fconds.length ? `<div class="fgroup" data-act="fbar">${icon('Filter', 13)}${S.fconds.map((c, i) => `<span class="fchip">${c[0]} ${c[1]} ${c[2]}<span data-act="f-rm-applied" data-i="${i}">${icon('X', 10)}</span></span>`).join('')}<span class="more">再加一条</span><span class="kbd">⌘F</span></div>` : `<div class="filterpill" data-act="fbar">${icon('Filter', 12)}<span class="ft">筛选：点击添加条件，或直接输入 WHERE 子句</span><span class="kbd">⌘F</span></div>`}<div class="sp"></div><span class="tbr">
      <button class="btn ghost sm" data-tip="选择显示的列 ⌥⌘F">${icon('Columns', 13)}<span class="lbl">列 6/6</span></button><button class="btn ghost icon sm sec" data-act="add-row" data-tip="新增行">${icon('Plus', 14)}</button><button class="btn ghost icon sm sec" data-tip="删除所选行">${icon('Trash', 14)}</button><span class="vr sec"></span>
      <button class="btn ghost sm" data-act="page" data-p="sql" data-tip="用 SQL 打开这张表">${icon('Code', 13)}<span class="lbl">SQL</span></button><button class="btn ghost icon sm sec ${S.dock === 'inspect' ? 'on' : ''}" data-act="inspect" data-tip="行预览 Space">${icon('Panel', 14)}</button><button class="btn ghost icon sm" data-act="refresh" data-tip="刷新 ⌘R">${icon('Refresh', 14)}</button>
      <button class="btn ghost icon sm more-btn" data-act="menu-at" data-k="tbmore" data-tip="更多">${icon('More', 14)}</button></span>`,
    body: () => `<div class="scroll grow" id="gridwrap" style="position:relative">${gridHTML()}</div>${pendingBar()}`,
    status: () => `<span class="it">第 1–30 行 · 共 12,431 行 · 4 ms</span><span class="it"><button class="btn ghost icon sm" style="height:18px;width:18px">${icon('ChevronR', 11).replace('<svg', '<svg style="transform:rotate(180deg)"')}</button>1 / 415<button class="btn ghost icon sm" style="height:18px;width:18px">${icon('ChevronR', 11)}</button></span>`,
    after: () => { window.bindGrid && window.bindGrid(); },
  };
  /* 行预览「JSON」页：整行当 JSON 显示，带语法高亮（键紫、字符串绿、数字橙、null 灰），只读可选中 */
  const inspectJSON = (r) => {
    const cols = [['id', r[0]], ['user_id', r[1]], ['status', r[2]], ['total', r[3]], ['created_at', r[4]], ['note', r[5]]];
    const num = new Set(['id', 'user_id', 'total']);
    const v = (k, x) => x === null ? '<span class="jn0">null</span>' : num.has(k) ? `<span class="jn">${x}</span>` : `<span class="js">"${x}"</span>`;
    return `<div class="scroll grow jsonv mono">[\n  {\n${cols.map(([k, x], i) => `    <span class="jk">"${k}"</span>: ${v(k, x)}${i < cols.length - 1 ? ',' : ''}`).join('\n')}\n  }\n]</div>`;
  };
  window.inspectDock = () => {
    const r = DB.orders[S.rowSel] || DB.orders[3];
    const f = (n, t, v, extra = '') => `<div class="col gap4"><div class="row"><span class="muted" style="font-size:11px">${n}</span><span class="muted sp" style="text-align:right;font-size:10px">${t}</span></div>${extra || `<div class="input mono" style="display:flex;align-items:center">${v === null ? '<i class="muted">NULL</i>' : v}</div>`}</div>`;
    return `<div class="row gap8" style="height:36px;padding:0 12px;border-bottom:1px solid var(--line);flex:none"><b>行 ${S.rowSel + 1}</b><span class="muted">orders · id ${r[0]}</span><div class="sp"></div>${seg(['字段', 'JSON'], S.inspTab || 0, 'insp-tab')}<button class="btn ghost icon sm" data-act="inspect">${icon('X', 12)}</button></div>
      ${S.inspTab === 1 ? inspectJSON(r) : ''}<div class="scroll grow col gap12" style="padding:12px 14px;${S.inspTab === 1 ? 'display:none' : ''}">${f('id', 'bigint · PK', r[0])}${f('user_id', 'bigint · → users.id', r[1])}${f('status', 'enum', '', `<div class="select" style="display:flex;align-items:center;gap:6px"><i class="dot ${({ paid: 'ok', pending: 'warn' })[r[2]] || 'acc'}" style="box-shadow:none"></i>${r[2]}<span class="sp"></span>${icon('ChevronD', 12)}</div>`)}${f('total', 'decimal(10,2)', r[3])}${f('created_at', 'datetime', r[4])}${f('note', 'text', r[5], `<div class="input" style="height:72px;padding:8px 9px;line-height:1.5">${r[5] || '<i class="muted">NULL</i>'}</div>`)}</div>`;
  };

  /* ---------------- 结构 ---------------- */
  PAGES.structure = {
    group: 'app', label: '表结构', icon: 'Columns', tab: { icon: 'Columns', label: 'products · 结构' },
    tools: () => `${viewTabs(1)}<div class="row gap6" style="margin-left:4px"><span class="muted">表名</span><div class="input" style="width:130px;height:24px;display:flex;align-items:center">products</div><span class="muted">引擎</span><div class="select" style="width:84px;height:24px;display:flex;align-items:center">InnoDB</div><div class="input" style="width:140px;height:24px;display:flex;align-items:center;color:var(--fg-3)">商品表</div></div><div class="sp"></div><button class="btn ghost icon sm">${icon('Refresh', 14)}</button>`,
    body: () => `<div class="row" style="padding:8px 12px 6px;gap:8px"><b>列</b><span class="muted">9</span><span class="muted" style="font-size:11px">拖动 ⠿ 调整顺序</span><div class="sp"></div><button class="btn sm">${icon('Plus', 12)}添加列</button></div>
      <div class="scroll" style="max-height:330px;flex:none;border-bottom:1px solid var(--line)"><table class="grid"><thead><tr><th class="n">#</th><th>名称</th><th>类型</th><th style="text-align:center">主键</th><th style="text-align:center">自增</th><th style="text-align:center">可空</th><th>默认值</th><th>备注</th></tr></thead><tbody>
      ${DB.struct.map((r, i) => `<tr class="${i === 7 ? 'new' : i === 8 ? 'del' : i === 4 ? 'sel' : ''}"><td class="n"><span class="muted">${icon('Grip', 10)}</span> ${i + 1}</td><td class="mono ${r[2] ? 'pk' : ''}">${r[2] ? icon('Key', 11) + ' ' : ''}${r[0]}</td><td class="mono ${i === 4 ? 'cell-sel' : ''}" style="color:var(--code-fn)">${r[1]}</td>${[2, 3, 4].map(k => `<td style="text-align:center"><span class="check ${r[k] ? 'on' : ''}">${icon('Check', 10)}</span></td>`).join('')}<td class="mono ${r[5] === 'NULL' ? 'null' : ''}">${r[5]}</td><td class="dim">${r[6]}</td></tr>`).join('')}</tbody></table></div>
      <div class="row" style="padding:0 12px;height:34px;gap:14px;border-bottom:1px solid var(--line);flex:none"><div class="tabs" style="height:100%"><button class="on">索引 <span class="n">4</span></button><button>外键 <span class="n">1</span></button><button>触发器 <span class="n">0</span></button></div><div class="sp"></div><button class="btn sm">${icon('Plus', 12)}添加索引</button></div>
      <div class="scroll grow"><table class="grid"><thead><tr><th class="n">#</th><th>索引名</th><th>索引列</th><th>类型</th><th style="text-align:center">唯一</th></tr></thead><tbody>${DB.indexes.map((r, i) => `<tr class="${i === 3 ? 'new' : ''}"><td class="n">${i + 1}</td><td class="mono">${r[0]}</td><td>${r[1].map(c => `<span class="chip" style="margin-right:4px">${c}</span>`).join('')}</td><td class="mono muted">${r[2]}</td><td style="text-align:center"><span class="check ${r[3] ? 'on' : ''}">${icon('Check', 10)}</span></td></tr>`).join('')}</tbody></table></div>
      <div class="islandbar"><span class="dot warn"></span><b>4 项结构修改</b><span class="sub">加列 1 · 删列 1 · 改列 1 · 加索引 1</span><span class="vr" style="background:rgba(255,255,255,.2)"></span><button class="btn ghost sm" data-act="modal" data-m="sqlprev">${icon('Code', 12)}预览 ALTER</button><button class="btn ghost sm">丢弃</button><button class="btn primary sm" data-act="toast" data-msg="已应用 4 项结构修改（耗时 182 ms）" data-k="ok">应用 <span class="kbd">⌘S</span></button></div>`,
    status: () => `<span class="it">提交顺序：删外键 / 触发器 → 改列和索引 → 建外键 / 触发器</span>`,
  };

  /* ---------------- 项目概览 ---------------- */
  PAGES.overview = {
    group: 'app', label: '项目概览', icon: 'Layers', tab: { icon: 'Layers', label: '项目概览' },
    tools: () => `${seg(['全部 9', '表 7', '视图 2'], 0)}<div class="search" style="width:200px;height:24px;margin-left:6px">${icon('Search', 12)}<input placeholder="筛选名称或备注…"></div><div class="sp"></div><span class="muted" style="font-size:11px">点表头排序 · 双击打开 · 右键更多</span><button class="btn ghost icon sm">${icon('Refresh', 14)}</button>`,
    body: () => `<div class="row gap12 stagger" style="padding:12px 14px 4px">${[['表', '7', '', '另有 2 个视图', [3, 4, 4, 5, 6, 7, 7]], ['总大小', '2.34', 'GB', '数据 1.9 GB · 索引 0.44 GB', [20, 22, 25, 25, 27, 29, 34]], ['估算行数', '61.6', 'K', '最大：order_items 31.2K', [10, 12, 18, 20, 25, 31, 40]], ['最近更新', '10:24', '', 'orders · 刚刚', [5, 8, 6, 9, 7, 12, 14]]].map(([t, n, u, s, ar]) => `<div class="card grow" style="padding:10px 14px;position:relative;overflow:hidden"><div class="muted" style="font-size:11px">${t}</div><div class="row" style="align-items:flex-end;gap:4px"><span style="font-size:24px;font-weight:650;letter-spacing:-.03em;line-height:1.2">${n}</span><span class="dim" style="margin-bottom:3px">${u}</span></div><div class="muted" style="font-size:10.5px">${s}</div><div style="position:absolute;right:8px;bottom:8px;opacity:.9">${spark(ar, 70, 26)}</div></div>`).join('')}</div>
      <div class="scroll grow" style="margin-top:8px"><table class="grid"><thead><tr><th class="n">#</th><th>名称 ↑</th><th>类型</th><th>引擎</th><th class="r">行数</th><th style="width:240px">数据大小</th><th class="r">索引</th><th>更新于</th></tr></thead><tbody>
      ${[['order_items', 31204, 62, '612 MB', '48 MB', '10:21'], ['orders', 12431, 40, '402 MB', '71 MB', '10:24'], ['payments', 11870, 36, '280 MB', '31 MB', '10:18'], ['users', 5300, 20, '86 MB', '12 MB', '昨天'], ['products', 840, 8, '24 MB', '4 MB', '周一'], ['coupons', 212, 2, '1.4 MB', '0.2 MB', '11-02'], ['categories', 64, 1, '0.1 MB', '—', '10-28']].map((r, i) => `<tr class="${i === 1 ? 'sel' : ''}"><td class="n">${i + 1}</td><td><span class="row gap6">${icon('Table', 13, 'ico-t')}<b>${r[0]}</b></span></td><td class="muted">table</td><td class="muted">InnoDB</td><td class="r mono">${r[1].toLocaleString()}</td><td><div class="row gap8"><div class="pbar grow"><i style="width:${r[2]}%"></i></div><span class="mono dim" style="width:56px;text-align:right">${r[3]}</span></div></td><td class="r mono dim">${r[4]}</td><td class="muted">${r[5]}</td></tr>`).join('')}
      ${DB.views.map((v, i) => `<tr><td class="n">${8 + i}</td><td><span class="row gap6">${icon('Eye', 13, 'ico-v')}<b>${v}</b></span></td><td class="muted">view</td><td class="muted">—</td><td class="r muted">—</td><td></td><td class="r muted">—</td><td class="muted">—</td></tr>`).join('')}</tbody></table></div>`,
    status: () => `<span class="it">7 张表 · 2 个视图 · 2.34 GB</span>`,
  };
})();
