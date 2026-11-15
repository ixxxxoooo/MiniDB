/* 弹层（第二批）：命令面板（左列表 + 右预览）、已打开的标签、新建数据库、设置（分组导航 + 卡片）。
   在 overlays.js 之后加载：renderOV 先查 OV2，查不到才走老的。 */
(function () {
  const seg = window.seg;
  const hl = window.hl;

  /* ---------- 已打开的标签：标签太多时的「全部列表」 ---------- */
  const tabsel = () => {
    const ids = allTabs(), q = (S.tq || '').toLowerCase();
    const cur = id => id.startsWith('x:') ? id === S.xtab : (id === S.page || id === (PAGES[S.page] || {}).tabOf) && !S.xtab;
    const rows = ids.filter(id => !q || tabLabel(id).label.toLowerCase().includes(q)).map(id => {
      const t = tabLabel(id);
      return `<div class="row-t ${cur(id) ? 'on' : ''} ${t.k ? 'tk-' + t.k : ''}" data-act="tab-pick" data-p="${id}">${icon(t.icon, 14)}<span class="trunc">${t.label}</span>${t.mod ? '<i class="mod"></i>' : ''}<span class="x" data-act="noop" data-tip="关闭">${icon('X', 11)}</span></div>`;
    }).join('') || '<div class="muted" style="padding:14px;text-align:center">没有匹配的标签</div>';
    return `<div class="scrim" style="background:transparent;backdrop-filter:none;place-items:start end" data-act="close-ov"><div class="card tpop" style="position:relative;margin:40px 52px 0 0" onclick="event.stopPropagation()">
      <div class="sec-h" style="padding:6px 8px 4px">已打开的标签 · ${ids.length}</div>
      <div class="search" style="margin:2px 2px 6px;height:26px">${icon('Search', 13)}<input placeholder="按名称筛选标签…" value="${S.tq || ''}" oninput="S.tq=this.value;renderOV()"></div>
      <div class="scroll" style="max-height:300px">${rows}</div>
      <div class="msep" style="margin:6px 4px"></div>
      <div class="cact" style="display:flex;align-items:center;gap:10px;padding:7px 8px;border-radius:8px;cursor:pointer" data-act="tabs-close-others">${icon('X', 13)}关闭其他标签</div></div></div>`;
  };

  /* ---------- 新建数据库 ---------- */
  const newdb = () => `<div class="scrim" data-act="close-ov"><div class="modal" style="width:420px" onclick="event.stopPropagation()"><div class="mh"><b class="h2 grow">新建数据库</b><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div><div class="mb col gap10" style="gap:10px"><div class="field" style="grid-template-columns:70px 1fr"><label>名称</label><div class="input focus" style="display:flex;align-items:center">shop_archive<span class="caret"></span></div></div><div class="field" style="grid-template-columns:70px 1fr"><label>字符集</label><div class="select" style="display:flex;align-items:center">utf8mb4<span class="sp"></span>${icon('ChevronD', 12)}</div></div><div class="field" style="grid-template-columns:70px 1fr"><label>排序规则</label><div class="select" style="display:flex;align-items:center">utf8mb4_0900_ai_ci<span class="sp"></span>${icon('ChevronD', 12)}</div></div></div><div class="mf"><span class="muted" style="font-size:11px">会执行 CREATE DATABASE，语句出现在控制台日志里</span><div class="sp"></div><button class="btn" data-act="close-ov">取消</button><button class="btn primary" data-act="close-ov" data-toast="已创建数据库 shop_archive">创建</button></div></div></div>`;

  /* ---------- 命令面板：左边分组列表，右边预览当前项 ---------- */
  const SCOPES = ['全部', '表', '查询', '命令', '历史', '连接'];
  const items = () => {
    const base = PAL_ITEMS().filter(it => it.g !== '历史');
    const hist = DB.history.map(h => ({ g: '历史', ic: 'History', t: h.sql, sub: `${h.day} ${h.t}`, h, run: () => go('sql') }));
    const saved = DB.saved.flatMap(([f, qs]) => qs.map(n => ({ g: '查询', ic: 'File', t: n, sub: f, run: () => go('sql') })));
    return [...base, ...saved, ...hist];
  };
  const rank = (t, q) => { t = t.toLowerCase(); return t === q ? 0 : t.startsWith(q) ? 1 : t.includes(q) ? 2 : 9; };
  window.paletteList = () => {
    const q = (S.q || '').trim(), cmd = q.startsWith('>'), qq = (cmd ? q.slice(1) : q).trim().toLowerCase();
    const scope = SCOPES[S.palScope || 0];
    let list = items().filter(it => (!cmd || it.g === '命令') && (scope === '全部' || it.g === scope) && (!qq || it.t.toLowerCase().includes(qq)));
    if (qq) list.sort((a, b) => rank(a.t, qq) - rank(b.t, qq));
    else if (scope === '全部' && !cmd) list = list.filter((it, i, a) => a.filter(x => x.g === it.g).indexOf(it) < 3);
    list = list.slice(0, 12);
    if (qq && !cmd) list.push({ g: 'AI', ic: 'Sparkle', t: `问 AI：“${q}”`, k: '⌘⏎', run: () => { S.dock = 'ai'; go('sql'); }, ai: 1 });
    S.palItems = list; S.palSel = Math.min(S.palSel || 0, list.length - 1);
    const mark = t => qq ? t.replace(new RegExp('(' + qq.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'i'), '<mark>$1</mark>') : t;
    let last = '';
    return list.map((it, i) => {
      const head = it.g !== last && !it.ai ? `<div class="gh">${last = it.g}</div>` : '';
      const lead = it.c ? tile(it.c, 'sm') : `<span class="pic ${it.ai ? 'ai' : ''}">${icon(it.ic, 14)}</span>`;
      const title = it.g === '历史' ? `<span class="sql trunc pal-sql">${hl(it.t)}</span>` : `<span class="trunc pal-t">${it.ai ? it.t : mark(it.t)}</span>`;
      const sub = it.g === '历史' ? '' : it.sub ? `<span class="trunc pal-s">${it.sub}</span>` : '';
      return `${head}<div class="it ${i === S.palSel ? 'on' : ''}" data-act="pal-pick" data-i="${i}">${lead}<span class="col grow" style="min-width:0;line-height:1.3">${title}${sub}</span>${it.tag ? tagOf(it.tag) : ''}${it.g === '历史' ? `<span class="pal-s">${it.h.t}</span>` : ''}${it.k ? `<span class="kbd">${it.k}</span>` : ''}</div>`;
    }).join('') || '<div class="muted" style="padding:32px 20px;text-align:center">没有匹配项<br><span style="font-size:11px">试试输入表名，或以 &gt; 开头只搜命令</span></div>';
  };
  /* 右侧预览：表 → 字段与行数；历史 → 高亮 SQL + 执行结果；连接 → 地址与版本；命令 → 说明与快捷键；AI → 提示 */
  window.palettePrev = () => {
    const it = (S.palItems || [])[S.palSel || 0];
    if (!it) return `<div class="pv-empty">${icon('Search', 22)}<span>选一项看预览</span></div>`;
    const head = (ic, t, s) => `<div class="pv-h">${ic}<div class="col" style="min-width:0"><b class="trunc">${t}</b><span class="muted trunc" style="font-size:11px">${s || ''}</span></div></div>`;
    if (it.g === '表') {
      const more = [['created_by', 'bigint'], ['updated_at', 'datetime'], ['deleted_at', 'datetime'], ['channel', 'varchar(32)'], ['coupon_id', 'bigint'], ['paid_at', 'datetime'], ['shipped_at', 'datetime'], ['remark', 'varchar(255)'], ['ext', 'json']].map(([n, t]) => ({ n, t }));
      const cols = [...DB.cols, ...more].map(c => `<div class="pv-r"><span class="mono">${c.n}</span><span class="muted mono" style="font-size:10.5px">${c.t}</span></div>`).join('');
      return head(`<span class="pic">${icon(it.ic, 15)}</span>`, it.t, it.sub) + `<div class="pv-b"><div class="h3">字段</div>${cols}<div class="muted" style="font-size:11px;margin-top:8px">⏎ 打开　·　⌘⏎ 新标签</div></div>`;
    }
    if (it.g === '历史') {
      return head(`<span class="pic">${icon('History', 15)}</span>`, it.h.day + ' ' + it.h.t, it.h.ok ? `${it.h.d} · ${it.h.r}` : '失败') + `<div class="pv-b"><div class="card sql" style="padding:10px 12px;background:var(--surface-2);font-size:12px;white-space:pre-wrap;word-break:break-word">${hl(it.h.sql)}</div><div class="row gap8" style="margin-top:10px">${it.h.ok ? `<span class="chip ok">${icon('Check', 10)}成功</span>` : `<span class="chip err">${icon('X', 10)}失败</span>`}<span class="muted" style="font-size:11px">${it.h.r}</span></div><div class="muted" style="font-size:11px;margin-top:8px">⏎ 放进新查询　·　⌘C 复制语句</div></div>`;
    }
    if (it.g === '连接') return head(tile(it.c, 'lg'), it.c.name, `${it.c.type} ${it.c.ver}`) + `<div class="pv-b"><div class="pv-r"><span class="muted">地址</span><span class="mono">${it.c.host}</span></div><div class="pv-r"><span class="muted">环境</span>${tagOf(it.c.env)}</div><div class="pv-r"><span class="muted">最近使用</span><span>${it.c.used}</span></div><div class="muted" style="font-size:11px;margin-top:8px">⏎ 连接</div></div>`;
    if (it.ai) return head(`<span class="pic ai">${icon('Sparkle', 15)}</span>`, '问 AI', '基于当前库的结构回答') + `<div class="pv-b"><div class="dim" style="line-height:1.6">AI 会读取 shop 的表结构来回答，写操作只给 SQL，由你确认后执行。</div></div>`;
    return head(`<span class="pic">${icon(it.ic, 15)}</span>`, it.t, it.sub || it.g) + `<div class="pv-b"><div class="muted" style="font-size:11px">${it.k ? `快捷键 <span class="kbd">${it.k}</span>　·　` : ''}⏎ 执行</div></div>`;
  };
  window.palRefresh = () => { const l = $('#pal-list'), p = $('#pal-prev'); if (l) l.innerHTML = paletteList(); if (p) p.innerHTML = palettePrev(); const on = $('#pal-list .it.on'); if (on) on.scrollIntoView({ block: 'nearest' }); };
  const palette = () => { const list = paletteList(); return `<div class="scrim top" data-act="close-ov"><div class="pal pal2" onclick="event.stopPropagation()">
    <div class="in">${icon('Search', 17)}<input id="palq" autofocus placeholder="搜索表、查询、历史，或输入 > 运行命令" value="${S.q || ''}"><span class="kbd">esc</span></div>
    <div class="pscopes">${SCOPES.map((s, i) => `<button class="${i === (S.palScope || 0) ? 'on' : ''}" data-act="pal-scope" data-i="${i}">${s}</button>`).join('')}</div>
    <div class="pbody"><div id="pal-list" class="scroll plist">${list}</div><div id="pal-prev" class="pprev">${palettePrev()}</div></div>
    <div class="pfoot"><span><span class="kbd">↑</span><span class="kbd">↓</span>选择</span><span><span class="kbd">⏎</span>打开</span><span><span class="kbd">⌘⏎</span>新标签</span><span class="sp"></span><span><span class="kbd">&gt;</span>只看命令</span><span><span class="kbd">tab</span>切换范围</span></div></div></div>`; };

  /* ---------- 设置：分组导航 + 卡片分区 ---------- */
  const NAV = [['常规', ['通用', '外观']], ['工作区', ['数据表', '编辑器', '连接']], ['安全与数据', ['安全', '备份']], ['智能', ['AI']], ['其他', ['快捷键', '关于']]];
  const ICS = { 外观: 'Palette', 通用: 'Settings', 数据表: 'Table', 编辑器: 'Terminal', 连接: 'Plug', 安全: 'Lock', 备份: 'Save', AI: 'Sparkle', 快捷键: 'Keyboard', 关于: 'Info' };
  const DESC = { 通用: '语言、启动方式与更新', 外观: '布局、配色与表格的观感', 数据表: '数据网格的显示与编辑习惯', 编辑器: 'SQL 编辑器的输入与格式化', 连接: '保活、重连与超时', 安全: '默认的安全级别与危险操作的确认', 备份: '连接配置的导出与导入', AI: '模型服务与权限', 快捷键: '点「修改」后按下新的组合键', 关于: '' };
  const sw = (on, act = 'sw') => `<span class="switch ${on ? 'on' : ''}" data-act="${act}"></span>`;
  const sel = (t, w = 200) => `<div class="select" style="width:${w}px;height:26px;display:flex;align-items:center">${t}<span class="sp"></span>${icon('ChevronD', 12)}</div>`;
  const row = (t, d, ctl) => `<div class="setrow"><div class="t"><b>${t}</b>${d ? `<div class="d">${d}</div>` : ''}</div>${ctl}</div>`;
  const card = (title, rows) => `<div class="setcard">${title ? `<div class="setcard-h">${title}</div>` : ''}${rows.join('')}</div>`;
  const PAGE = {
    通用: () => [card('语言与启动', [row('界面语言', '', sel('简体中文')), row('启动时', '重新打开上次关闭时的连接', sel('恢复上次打开的连接', 220))]), card('更新与隐私', [row('检查更新', '每天一次，后台静默检查', sw(1)), row('发送匿名崩溃报告', '不含任何连接信息和数据', sw(0))])],
    外观: () => {
      const TH = [['light', '浅色', '#f4f5f9', '#ffffff'], ['dark', '深色', '#15171d', '#1f232c'], ['paper', '纸墨', '#f1ebdd', '#faf6ec'], ['aurora', '极光', '#0e1a2c', '#16294a']];
      const ACC = ['indigo', 'blue', 'teal', 'rose', 'amber', 'mono'], C1 = { indigo: '#5b5df0', blue: '#2f7cf6', teal: '#0fa89a', rose: '#e5457a', amber: '#e08a0a', mono: '#555' }, C2 = { indigo: '#8b5cf6', blue: '#22b8e8', teal: '#3bc9a0', rose: '#f0804a', amber: '#f2b13c', mono: '#999' };
      return [
        card('布局', [`<div class="row gap10" style="gap:10px;padding:12px 0 6px">${[['A', '精炼工作台'], ['B', '轨道'], ['C', '阿特拉斯']].map(([k, n]) => `<div class="schemecard ${S.scheme === k ? 'on' : ''}" data-act="scheme" data-k="${k}"><div class="pv" style="background:var(--bg)"><i style="width:8px;background:var(--surface-3)"></i><i style="width:22px;background:var(--surface-2)"></i><i style="flex:1;background:var(--surface);border:1px solid var(--line)"></i></div><div class="lb">${k} · ${n}</div></div>`).join('')}</div>`]),
        card('配色', [`<div class="row gap10" style="gap:10px;padding:12px 0 8px">${TH.map(([k, n, a, b]) => `<div class="themecard ${S.theme === k ? 'on' : ''}" data-act="set-theme" data-k="${k}"><div class="tp" style="background:${a}"><i style="background:${b}"></i><i style="background:${b};opacity:.7"></i></div><span>${n}</span></div>`).join('')}</div>`, row('强调色', '只影响按钮、选中态与链接；连接色不受影响', `<div class="row gap8">${ACC.map(a => `<button class="sw ${S.accent === a ? 'on' : ''}" data-act="set-accent" data-k="${a}" style="width:20px;height:20px;background:linear-gradient(135deg,${C1[a]},${C2[a]})"></button>`).join('')}</div>`)]),
        card('表格与密度', [row('界面密度', '紧凑 26px / 标准 30px 行高', seg(['紧凑', '标准'], 0, 'noop')), row('表格行样式', '', seg(['细行线', '斑马纹'], S.zebra ? 1 : 0, 'zebra')), row('数字右对齐', '', sw(1)), row('NULL 显示为灰色斜体', '', sw(1))]),
        card('动态效果', [row('减弱动态效果', '关闭过渡与回弹，系统开启「减弱动态」时自动生效', sw(S.reduce, 'reduce'))]),
      ];
    },
    数据表: () => [card('网格', [row('每页行数', '', sel('200', 120)), row('日期时间格式', '', sel('YYYY-MM-DD HH:mm:ss', 220))]), card('编辑', [row('双击单元格进入编辑', '', sw(1)), row('空值显示为 NULL（斜体灰色）', '', sw(1))])],
    编辑器: () => [card('输入', [row('字体', '', sel('JetBrains Mono 12.5', 220)), row('Tab 宽度', '', sel('2 空格', 120)), row('输入时自动补全', '', sw(1))]), card('格式化', [row('格式化时关键字大写', '', sw(1))])],
    连接: () => [card('保活与重连', [row('保活间隔', '', sel('30 秒', 120)), row('断线自动重连', '最多重试 3 次，退避 1 / 2 / 4 秒', sw(1)), row('连接超时', '', sel('10 秒', 120))])],
    安全: () => [card('安全级别', [row('默认安全级别', '新建连接时使用；每个连接可单独改', sel('提醒', 160)), row('生产环境强制二次确认', 'DROP / TRUNCATE / 无 WHERE 的 UPDATE、DELETE', sw(1))]), card('日志', [row('把控制台日志保存到磁盘', '会对密码和字面量脱敏', sw(1))])],
    备份: () => [card('连接配置', [row('导出所有连接', '密码用口令加密', '<button class="btn sm">导出…</button>'), row('导入连接', '', '<button class="btn sm">选择文件…</button>')])],
    AI: () => [card('模型服务', [row('服务地址', '', sel('https://api.example.com/v1', 260)), row('模型', '', sel('claude-4.5', 200))]), card('权限', [row('只读模式', '写操作只给 SQL，由你审阅后执行', sw(1))])],
    快捷键: () => [['通用', [['打开命令面板', '⌘K'], ['设置', '⌘,'], ['AI 助手', '⌘I'], ['切换侧栏', '⌘B']]], ['编辑', [['新建查询', '⌘T'], ['执行语句', '⌘⏎'], ['提交修改', '⌘S']]], ['工具', [['监控', '⌥⌘M']]]].map(([t, ks]) => card(t, ks.map(([a, k]) => `<div class="setrow"><div class="t">${a}</div><span class="kbd">${k}</span><button class="btn ghost sm">修改</button></div>`))),
    关于: () => [`<div class="col" style="align-items:center;gap:8px;padding:34px 0">${LOGO.glyph('peak', 72, { tile: true })}${LOGO.wordmark(24)}<span class="muted">版本 0.3.0 (build 412) · MyGo 0.9</span><div class="row gap8" style="margin-top:8px"><button class="btn sm">检查更新</button><button class="btn sm">更新日志</button></div></div>`],
  };
  const settings = () => {
    const cur = S.setTab || '外观', q = (S.sq || '').trim();
    const nav = NAV.map(([g, ns]) => { const ns2 = ns.filter(n => !q || n.includes(q)); return ns2.length ? `<div class="setgrp">${g}</div>${ns2.map(n => `<div class="tr ${n === cur ? 'on' : ''}" data-act="set-tab" data-k="${n}" style="height:30px">${icon(ICS[n], 14)}${n}</div>`).join('')}` : ''; }).join('');
    return `<div class="scrim" data-act="close-ov"><div class="modal setwin" onclick="event.stopPropagation()"><div class="setnav"><div class="search" style="margin:2px 0 10px;height:28px">${icon('Search', 13)}<input placeholder="搜索设置…" value="${S.sq || ''}" oninput="S.sq=this.value;renderOV()"></div>${nav}</div>
      <div class="col grow" style="min-width:0"><div class="row" style="height:64px;padding:0 26px;border-bottom:1px solid var(--line);gap:12px"><span class="tile-ic" style="width:32px;height:32px">${icon(ICS[cur], 17)}</span><div class="col grow"><b class="h2" style="font-size:16px">${cur}</b>${DESC[cur] ? `<span class="muted" style="font-size:11.5px">${DESC[cur]}</span>` : ''}</div><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div>
      <div class="scroll grow setbody">${(PAGE[cur] || (() => []))().join('')}</div></div></div></div>`;
  };

  window.OV2 = { tabsel, newdb, palette, settings };
})();
