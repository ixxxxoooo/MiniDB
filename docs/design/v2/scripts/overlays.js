/* 弹层：命令面板、右键菜单、连接 / 库切换、各类对话框、提示卡（toast） */
(function () {
  const mi = (ic, t, k, cls = '', act = '') => `<div class="mi ${cls}" data-act="${act || 'menu-pick'}" data-t="${t}">${ic ? icon(ic, 13) : '<span style="width:13px"></span>'}<span>${t}</span>${k ? `<span class="kbd">${k}</span>` : ''}</div>`;
  const sep = '<div class="msep"></div>';
  const nav = (ic, t, act, d, k) => `<div class="mi" data-act="${act}" ${d}>${icon(ic, 13)}<span>${t}</span>${k ? `<span class="kbd">${k}</span>` : ''}</div>`;
  const MENUS = {
    transfer: () => nav('Download', '导出表…', 'page', 'data-p="export"', '⇧⌘E') + nav('Upload', '导入文件…', 'page', 'data-p="import"') + sep + nav('Backup', '备份数据库…', 'modal', 'data-m="backup"') + nav('History', '恢复备份…', 'modal', 'data-m="restore"'),
    conn: () => mi('Zap', '连接', '⏎') + mi('Pencil', '编辑…', '⌘E') + mi('Copy', '复制连接') + mi('Pin', '固定到顶部') + mi('Folder', '移动到…') + mi('Link', '复制连接 URL') + sep + mi('Trash', '删除…', '⌫', 'danger'),
    row: () => mi('Copy', '复制', '⌘C') + mi('Code', '复制为 INSERT') + mi('Bars', '复制为 JSON') + sep + mi('Link', '查看关联：users #1042') + mi('Panel', '在右侧预览', 'Space') + sep + mi('Plus', '新增行', '⌘↵') + mi('Copy', '复制此行', '⌘D') + mi('Trash', '删除行', '⌫', 'danger'),
    table: () => mi('Table', '打开', '⏎') + mi('Plus', '在新标签中打开', '⌘⏎') + mi('Pin', '固定到顶部') + sep + mi('Columns', '查看结构') + mi('Network', 'ER 图') + mi('Code', '复制建表语句') + mi('Download', '导出…', '⇧⌘E') + sep + mi('Undo', '清空表…', '⌥⌫', 'danger', 'danger-trunc') + mi('Trash', '删除表…', '⌫', 'danger', 'danger-drop'),
  };

  /* ---------- 命令面板 ---------- */
  const ITEMS = () => [
    ...DB.tables.map(([n, c]) => ({ g: '表', ic: 'Table', t: n, sub: `shop · ${c > 9999 ? (c / 1000).toFixed(1) + 'k' : c} 行`, hint: '打开 ⏎', run: () => go('data') })),
    ...DB.views.map(n => ({ g: '表', ic: 'Eye', t: n, sub: '视图', run: () => go('data') })),
    { g: '命令', ic: 'Terminal', t: '新建查询', sub: '在 shop 上', k: '⌘T', run: () => go('sql') },
    { g: '命令', ic: 'Download', t: '导出 orders…', k: '⇧⌘E', run: () => go('export') },
    { g: '命令', ic: 'Layers', t: '项目概览', run: () => go('overview') }, { g: '命令', ic: 'Network', t: 'ER 图', run: () => go('er') }, { g: '命令', ic: 'Activity', t: '监控', k: '⌥⌘M', run: () => go('monitor') },
    { g: '命令', ic: 'Moon', t: '切换主题', k: '⌘⇧L', run: () => cycleTheme() }, { g: '命令', ic: 'Settings', t: '打开设置', k: '⌘,', run: () => openModal('settings') },
    { g: '历史', ic: 'History', t: "SELECT status, COUNT(*) FROM orders GROUP BY status", sub: '10:24', run: () => go('sql') },
    ...DB.conns.map(c => ({ g: '连接', ic: 'Plug', t: c.name, sub: c.host, tag: c.env, c, run: () => connect(c.id) })),
  ];
  window.paletteList = () => {
    const q = (S.q || '').trim(), cmd = q.startsWith('>'), qq = (cmd ? q.slice(1) : q).trim().toLowerCase();
    const scope = ['全部', '表', '查询', '命令', '历史', '连接'][S.palScope || 0];
    let list = ITEMS().filter(it => (!cmd || it.g === '命令') && (scope === '全部' || it.g === scope) && (!qq || it.t.toLowerCase().includes(qq)));
    if (!qq && !cmd) list = list.filter((it, i, a) => a.filter(x => x.g === it.g).indexOf(it) < 3);
    list = list.slice(0, 9);
    if (qq && !cmd) list.push({ g: 'AI', ic: 'Sparkle', t: `问 AI：“${q}”`, k: '⌘⏎', run: () => { S.dock = 'ai'; go('sql'); }, ai: 1 });
    S.palItems = list; S.palSel = Math.min(S.palSel || 0, list.length - 1);
    let last = '';
    const mark = t => qq ? t.replace(new RegExp('(' + qq.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'i'), '<mark>$1</mark>') : t;
    return list.map((it, i) => (it.g !== last && it.g !== 'AI' ? `<div class="gh">${last = it.g}</div>` : '') + `<div class="it ${i === S.palSel ? 'on' : ''}" data-act="pal-pick" data-i="${i}">${it.c ? tile(it.c, 'sm') : icon(it.ic, 15)}<span class="trunc ${it.g === '历史' ? 'mono' : ''}" style="${it.g === '历史' ? 'font-size:11.5px' : ''}">${it.ai ? it.t : mark(it.t)}</span>${it.sub ? `<span class="muted trunc" style="font-size:11px;max-width:210px">${it.sub}</span>` : ''}${it.tag ? tagOf(it.tag) : ''}<span class="sp"></span>${i === S.palSel && !it.k ? '<span style="font-size:11px">打开 ⏎</span>' : it.k ? `<span class="kbd">${it.k}</span>` : ''}</div>`).join('') || '<div class="muted" style="padding:26px;text-align:center">没有匹配项<br><span style="font-size:11px">试试输入表名，或以 &gt; 开头只搜命令</span></div>';
  };
  const palette = () => `<div class="scrim top" data-act="close-ov"><div class="pal" onclick="event.stopPropagation()"><div class="in">${icon('Search', 17)}<input id="palq" autofocus placeholder="搜索表、查询、历史，或输入 > 运行命令" value="${S.q || ''}"><span class="kbd">esc</span></div>
    <div class="row gap6" style="padding:8px 10px 4px">${['全部', '表', '查询', '命令', '历史', '连接'].map((s, i) => `<button class="btn sm ${i === (S.palScope || 0) ? 'soft' : 'ghost'}" style="border-radius:99px;height:22px" data-act="pal-scope" data-i="${i}">${s}</button>`).join('')}</div>
    <div id="pal-list" class="scroll" style="max-height:340px;padding:2px 0 6px">${paletteList()}</div>
    <div class="row gap12 muted" style="height:32px;padding:0 14px;border-top:1px solid var(--line);font-size:11px;background:color-mix(in srgb,var(--surface-2) 70%,transparent)"><span><span class="kbd">↑</span> <span class="kbd">↓</span> 选择</span><span><span class="kbd">⏎</span> 打开</span><span><span class="kbd">⌘⏎</span> 新标签</span><span class="sp"></span><span><span class="kbd">&gt;</span> 只看命令</span></div></div></div>`;

  /* ---------- 连接 / 库切换（弹层） ---------- */
  const connPop = () => {
    const pos = S.scheme === 'A' ? 'left:56px;top:78px' : S.scheme === 'B' ? 'left:84px;top:100px' : 'left:10px;top:76px';
    return `<div class="scrim" style="background:transparent;backdrop-filter:none;place-items:start" data-act="close-ov"><div class="card" style="position:absolute;${pos};width:340px;border-radius:var(--r-xl);box-shadow:var(--shadow-2);padding:6px;animation:pop var(--dur-2) var(--ease);transform-origin:top left" onclick="event.stopPropagation()">
      <div class="search" style="margin:2px 2px 6px">${icon('Search', 13)}<input placeholder="搜索连接 / 库…" autofocus></div><div class="sec-h">已打开</div>
      ${DB.conns.filter(c => c.open).map(c => `<div class="tr ${c.id === S.conn ? 'on' : ''}" style="height:34px" data-act="set-conn" data-id="${c.id}">${tile(c)}<div class="col" style="line-height:1.25"><b style="color:var(--fg)">${c.name}</b><span class="muted" style="font-size:10.5px">${c.host.split(':')[0]}</span></div><span style="margin-left:auto" class="row gap6">${tagOf(c.env)}${c.id === S.conn ? icon('Check', 13) : ''}</span></div>`).join('')}
      <div class="sec-h" style="margin-top:4px">SHOP-DEV 的数据库</div>${DB.dbs.map((d, i) => `<div class="tr" style="height:26px">${i === 0 ? icon('Check', 12) : '<span style="width:12px"></span>'}<span style="${i === 0 ? 'font-weight:600;color:var(--fg)' : ''}">${d[0]}</span><span class="cnt">${d[1] || ''}</span></div>`).join('')}
      <div class="msep" style="margin:4px 6px"></div><div class="tr" data-act="modal" data-m="conn">${icon('Plus', 13)}新建连接…<span class="kbd" style="margin-left:auto">⌘N</span></div><div class="tr" data-act="go-welcome">${icon('Plug', 13)}全部连接…</div><div class="tr" style="color:var(--err)">${icon('Unplug', 13)}断开 shop-dev</div></div></div>`;
  };
  /* 方案 A 的连接卡弹层：已打开的连接 + 打开 / 新建 */
  const connPopA = () => `<div class="scrim" style="background:transparent;backdrop-filter:none;place-items:start" data-act="close-ov"><div class="card cpop" style="left:84px;top:40px;width:360px" onclick="event.stopPropagation()">
      <div class="sec-h">已打开的连接 · ${DB.conns.filter(c => c.open).length}</div>
      ${DB.conns.filter(c => c.open).map(c => `<div class="crow ${c.id === S.conn ? 'on' : ''}" data-act="set-conn" data-id="${c.id}">${tile(c, 'lg')}<div class="meta"><span class="row gap6"><b>${c.name}</b>${tagOf(c.env)}</span><span class="s">${c.type} ${c.ver} · ${c.id === S.conn ? 1 : 4 + c.name.length % 14} ms</span></div><span class="okdot"><i></i></span><span class="x" data-act="close-ov" data-toast="已断开 ${c.name}">${icon('X', 13)}</span></div>`).join('')}
      <div class="msep" style="margin:6px 6px"></div>
      <div class="cact" data-act="go-welcome">${icon('Plug', 15)}打开连接…<span class="kbd">⇧⌘K</span></div><div class="cact" data-act="modal" data-m="conn">${icon('Plus', 15)}新建连接…<span class="kbd">⌘N</span></div></div></div>`;
  /* 方案 A 的库切换弹层：搜索 + 库列表（大小 · 表数）+ 新建库 */
  const DBINFO = { shop: '248 MB · 24 张表', shop_test: '3.1 GB · 12 张表', analytics: '812 MB · 41 张表', information_schema: '— · 79 张表', mysql: '— · 38 张表' };
  const dbPopA = () => `<div class="scrim" style="background:transparent;backdrop-filter:none;place-items:start" data-act="close-ov"><div class="card dpop" style="left:56px;top:104px;width:312px" onclick="event.stopPropagation()">
      <div class="dsrch">${icon('Search', 14)}<span>切换数据库…</span><span class="kbd" style="margin-left:auto">⌘K</span></div>
      ${DB.dbs.map((d, i) => `<div class="drow ${i === 0 ? 'on' : ''}" data-act="close-ov" data-toast="已切换到 ${d[0]}">${icon('Database', 16)}<div class="meta"><b style="font-weight:${i === 0 ? 600 : 500}">${d[0]}</b><span class="s">${DBINFO[d[0]] || ''}</span></div>${i === 0 ? icon('Check', 15, 'ck') : ''}</div>`).join('')}
      <div class="msep" style="margin:6px 6px"></div><div class="cact" data-act="modal" data-m="newdb">${icon('DatabasePlus', 15)}新建数据库…</div></div></div>`;
  const bellPop = () => `<div class="scrim" style="background:transparent;backdrop-filter:none;place-items:start end" data-act="close-ov"><div class="card" style="margin:44px 14px;width:320px;border-radius:var(--r-xl);box-shadow:var(--shadow-2);animation:pop var(--dur-2) var(--ease);transform-origin:top right;overflow:hidden" onclick="event.stopPropagation()"><div class="row" style="padding:10px 14px"><b>通知</b><span class="sp"></span><button class="btn ghost sm">全部已读</button></div>
    ${[['ok', 'CheckC', '导出完成', 'orders.sql · 21 MB', '2 分钟前'], ['info', 'Download', '有新版本 0.3.0', '更新内容：界面全面重设计', '1 小时前'], ['warn', 'Alert', 'shop-prod 慢查询增多', '平均耗时 1.8 s，较昨天 +40%', '昨天']].map(r => `<div class="row gap10" style="padding:9px 14px;border-top:1px solid var(--line-soft);gap:10px;align-items:flex-start"><span class="toast ${r[0]}" style="all:unset"><span class="ti" style="width:24px;height:24px;border-radius:50%;display:grid;place-items:center;background:var(--${r[0] === 'info' ? 'acc-soft' : r[0] + '-soft'});color:var(--${r[0] === 'info' ? 'acc-text' : r[0]})">${icon(r[1], 13)}</span></span><div class="col grow"><b>${r[2]}</b><span class="muted" style="font-size:11px">${r[3]}</span></div><span class="muted" style="font-size:10.5px">${r[4]}</span></div>`).join('')}</div></div>`;

  /* ---------- 对话框 ---------- */
  const SQLPREV = `<span class="c">-- 将在事务中依次执行</span>
ALTER TABLE \`products\`
  ADD COLUMN \`weight_g\` int NULL COMMENT '重量（克）',
  DROP COLUMN \`legacy_flag\`,
  MODIFY COLUMN \`price\` decimal(12,2) NOT NULL DEFAULT 0.00,
  ADD INDEX \`idx_status_created\` (\`status\`, \`created_at\`);`;
  const sqlprev = () => `<div class="scrim" data-act="close-ov"><div class="modal" style="width:560px" onclick="event.stopPropagation()"><div class="mh"><b class="h2 grow">预览 SQL</b><span class="chip warn">4 条语句</span><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div>
    <div class="mb"><div class="card sql" style="padding:12px 14px;background:var(--surface-2);font-size:12px">${S.modal === 'sqlprev' && S.page === 'data' ? hl("UPDATE `orders` SET `total` = 128.00 WHERE `id` = 10225;\nUPDATE `orders` SET `status` = 'shipped' WHERE `id` = 10230;\nUPDATE `orders` SET `note` = '礼品包装' WHERE `id` = 10228;") : SQLPREV}</div></div>
    <div class="mf"><span class="muted" style="font-size:11px">${icon('Lock', 11)} 这些语句也会出现在控制台日志里</span><div class="sp"></div><button class="btn" data-act="close-ov">${icon('Copy', 12)}复制</button><button class="btn primary" data-act="close-ov">关闭</button></div></div></div>`;
  const confirm = (kind) => {
    const K = { drop: ['删除表 orders？', '这会永久删除表和其中 12,431 行数据，无法撤销。', 'DROP TABLE `orders`;', '删除表'], trunc: ['清空表 orders？', '表结构会保留，12,431 行数据将被永久删除。', 'TRUNCATE TABLE `orders`;', '清空'], kill: ['终止进程 1881？', '正在运行的语句会被中断，已执行的部分不会回滚。', 'KILL 1881;', '终止'], danger: ['在 shop-prod 上执行写操作', '当前连接是生产环境，安全级别为「提醒 2」。请确认这条语句：', "UPDATE orders SET status = 'cancelled'\nWHERE created_at < '2026-01-01';", '确认执行'] }[kind] || [];
    const warn = kind === 'danger';
    return `<div class="scrim" data-act="close-ov"><div class="modal" style="width:400px" onclick="event.stopPropagation()"><div style="padding:18px 18px 8px;display:flex;gap:12px"><div style="width:34px;height:34px;border-radius:50%;display:grid;place-items:center;flex:none;background:var(--${warn ? 'warn' : 'err'}-soft);color:var(--${warn ? 'warn' : 'err'})">${icon(warn ? 'ShieldAlert' : 'Alert', 17)}</div><div class="col gap4"><b class="h2">${K[0]}</b><span class="dim" style="line-height:1.55">${K[1]}</span></div></div>
    <div style="padding:6px 18px 14px"><div class="card sql" style="padding:10px 12px;background:var(--surface-2);font-size:12px">${hl(K[2])}</div>${warn ? `<div class="col gap6" style="margin-top:12px"><span class="muted" style="font-size:11px">输入环境名 <b class="mono">shop-prod</b> 以确认</span><div class="input mono focus" style="display:flex;align-items:center">shop-p<span class="caret"></span></div></div>` : ''}</div>
    <div class="mf"><span class="muted" style="font-size:11px"><span class="kbd">esc</span> 取消 · <span class="kbd">⌘⏎</span> ${K[3]}</span><div class="sp"></div><button class="btn" data-act="close-ov" autofocus>取消</button><button class="btn ${warn ? 'primary' : 'danger'}" data-act="confirm-ok" data-msg="${K[3]}已执行">${K[3]}</button></div></div></div>`;
  };
  const connForm = () => {
    const T = ['MySQL', 'PostgreSQL', 'SQLite', 'MariaDB', 'ClickHouse', 'TiDB', 'StarRocks', 'SQL Server'], colors = ['#e5484d', '#3b6fb6', '#3a86d1', '#1a8f87', '#c9a30a', '#7c5cd6', '#2a9d6a', '#7a7f8c'];
    const tab = S.ctab || 0;
    const test = S.test === 1 ? `<span class="row gap6 dim"><i class="spin"></i>正在连接…</span>` : S.test === 2 ? `<span class="chip ok" style="height:22px;animation:pop var(--dur-2) var(--spring)">${icon('Check', 11)}连接成功 · 38 ms · MySQL 8.0.36</span>` : '';
    const f = (l, v, o = {}) => `<div class="field"><label>${l}</label><div class="${o.sel ? 'select' : 'input'} ${o.mono ? 'mono' : ''} ${o.focus ? 'focus' : ''}" style="display:flex;align-items:center;gap:6px">${v}${o.sel ? '<span class="sp"></span>' + icon('ChevronD', 12) : ''}${o.eye ? '<span class="sp"></span>' + icon('Eye', 13) : ''}</div></div>`;
    const pages = [`${f('名称', 'shop-prod')}<div class="field"><label>环境</label><div class="row gap12">${seg(['LOCAL', 'DEV', 'STG', 'PROD'], 3, 'noop')}<span class="muted" style="font-size:11px;line-height:1.4">生产环境会显示红色环境带，<br>默认安全级别为「提醒 2」</span></div></div>
      <div class="field"><label>主机 / 端口</label><div class="row gap8"><div class="input mono grow">rds-prod.cn-east-1.internal</div><div class="input mono" style="width:84px">3306</div></div></div>${f('用户', 'app_rw')}${f('密码', '••••••••••••', { eye: 1, focus: 1 })}${f('默认库', 'shop')}
      <div class="field"><label></label><div class="row gap12" style="flex-wrap:wrap"><label class="row gap6"><span class="switch on" data-act="sw"></span><span><b>只读模式</b><br><span class="muted" style="font-size:10.5px">拒绝所有写操作</span></span></label><label class="row gap6"><span class="switch on" data-act="sw"></span><span><b>AI 只共享库结构</b><br><span class="muted" style="font-size:10.5px">不读取表数据</span></span></label><span class="chip warn" style="height:26px;border-radius:var(--r)">${icon('Shield', 11)}提醒 2 ${icon('ChevronD', 11)}</span></div></div>`,
      `<div class="field"><label>通过 SSH 隧道</label><span class="switch on" data-act="sw"></span></div>${f('SSH 主机 / 端口', 'bastion-01.corp.example.com : 22', { mono: 1 })}${f('用户', 'deploy')}<div class="field"><label>认证</label>${seg(['密码', '私钥'], 1, 'noop')}</div>${f('私钥文件', '~/.ssh/id_ed25519', { mono: 1 })}${f('口令', '••••••••', { eye: 1 })}<div class="field"><label></label><span class="chip ok">${icon('Check', 11)}隧道可用 · 22 ms</span></div>`,
      `<div class="field"><label>SSL 模式</label>${seg(['关闭', '优先', '必须', '校验 CA'], 2, 'noop')}</div>${f('CA 证书', '/etc/ssl/rds-ca.pem', { mono: 1 })}${f('客户端证书', '未选择', { sel: 1 })}`,
      `${f('连接超时', '10 秒')}${f('保活间隔', '30 秒')}${f('字符集', 'utf8mb4', { sel: 1 })}${f('初始化 SQL', "SET time_zone = '+08:00'", { mono: 1 })}`];
    return `<div class="scrim" data-act="close-ov"><div class="modal" style="width:700px" onclick="event.stopPropagation()"><div class="mh"><b class="h2">新建连接</b><span class="muted grow">选择数据库类型，填写后先测试再保存</span><button class="btn ghost icon sm" data-tip="从 URL 导入">${icon('Link', 14)}</button><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div>
      <div style="padding:2px 16px 8px"><div class="typegrid">${T.map((t, i) => `<div class="ty ${i === (S.ctype || 0) ? 'on' : ''}" data-act="ctype" data-i="${i}"><span class="dbtile lg" style="background:linear-gradient(135deg,${colors[i]},color-mix(in srgb,${colors[i]} 70%,#000))">${DB.abbr[t]}</span>${t}</div>`).join('')}</div></div>
      <div class="row" style="padding:0 16px;height:34px;border-bottom:1px solid var(--line)"><div class="tabs" style="height:100%;gap:18px">${['常规', 'SSH 隧道', 'SSL', '高级'].map((t, i) => `<button class="${i === tab ? 'on' : ''}" data-act="ctab" data-i="${i}">${t}</button>`).join('')}</div></div>
      <div class="mb col gap10" style="padding:14px 18px;min-height:268px;gap:10px" id="cf-body">${pages[tab]}</div>
      <div class="mf"><button class="btn" data-act="test-conn">${icon('Zap', 12)}测试连接</button>${test}<div class="sp"></div><button class="btn" data-act="close-ov">取消</button><button class="btn" data-act="close-ov">保存</button><button class="btn primary" data-act="close-ov" data-toast="已保存并连接 shop-prod">保存并连接</button></div></div></div>`;
  };
  const SET = {
    通用: [['界面语言', '简体中文', 'select'], ['启动时', '恢复上次打开的连接', 'select'], ['检查更新', 1, 'sw', '每天一次，后台静默检查'], ['发送匿名崩溃报告', 0, 'sw']],
    数据表: [['每页行数', '200', 'select'], ['双击单元格进入编辑', 1, 'sw'], ['空值显示为 NULL（斜体灰色）', 1, 'sw'], ['日期时间格式', 'YYYY-MM-DD HH:mm:ss', 'select']],
    编辑器: [['字体', 'JetBrains Mono 12.5', 'select'], ['输入时自动补全', 1, 'sw'], ['格式化时关键字大写', 1, 'sw'], ['Tab 宽度', '2 空格', 'select']],
    连接: [['保活间隔', '30 秒', 'select'], ['断线自动重连', 1, 'sw', '最多重试 3 次，退避 1 / 2 / 4 秒'], ['连接超时', '10 秒', 'select']],
    安全: [['默认安全级别', '提醒 2', 'select'], ['生产环境强制二次确认', 1, 'sw', 'DROP / TRUNCATE / 无 WHERE 的 UPDATE、DELETE'], ['把控制台日志保存到磁盘', 1, 'sw', '会对密码和字面量脱敏']],
    备份: [['导出所有连接', '导出…', 'btn', '密码用口令加密'], ['导入连接', '选择文件…', 'btn']],
    AI: [['服务地址', 'https://api.example.com/v1', 'select'], ['模型', 'claude-4.5', 'select'], ['只读模式', 1, 'sw', '写操作只给 SQL，由你审阅后执行']],
    快捷键: [],
    关于: [],
  };
  const settings = () => {
    const nav = DB.settingsNav, cur = S.setTab || '外观';
    const look = `<div class="setrow"><div class="t"><b>界面布局</b><div class="d">三种布局：精炼工作台 / 轨道 / 阿特拉斯。即时切换，不丢失当前标签。</div></div></div>
      <div class="row gap10" style="gap:10px;padding:8px 0 14px">${[['A', '精炼工作台', ['var(--surface-3)', 'var(--surface-2)', 'var(--surface)']], ['B', '轨道', ['transparent', 'var(--glass)', 'var(--surface)']], ['C', '阿特拉斯', ['var(--surface-3)', 'var(--surface-2)', 'var(--surface)']]].map(([k, n, c]) => `<div class="schemecard ${S.scheme === k ? 'on' : ''}" data-act="scheme" data-k="${k}"><div class="pv" style="background:var(--bg)">${k === 'B' ? `<i style="width:8px;background:var(--glass);border:1px solid var(--line);border-radius:6px"></i><i style="flex:1;background:var(--surface);border:1px solid var(--line);border-radius:6px"></i>` : `<i style="width:8px;background:${c[0]}"></i><i style="width:${k === 'C' ? 26 : 22}px;background:${c[1]}"></i><i style="flex:1;background:${c[2]};border:1px solid var(--line)"></i>`}</div><div class="lb">${k} · ${n}</div></div>`).join('')}</div>
      <div class="setrow"><div class="t"><b>主题</b><div class="d">跟随系统时会在日落时自动切换</div></div><div class="row gap6">${[['light', '浅色'], ['dark', '深色'], ['paper', '纸墨'], ['aurora', '极光']].map(([k, n]) => `<button class="btn sm ${S.theme === k ? 'soft' : ''}" data-act="set-theme" data-k="${k}">${n}</button>`).join('')}</div></div>
      <div class="setrow"><div class="t"><b>强调色</b><div class="d">只影响按钮、选中态与链接；连接色不受影响</div></div><div class="row gap8">${['indigo', 'blue', 'teal', 'rose', 'amber', 'mono'].map(a => `<button class="sw ${S.accent === a ? 'on' : ''}" data-act="set-accent" data-k="${a}" style="width:20px;height:20px;background:linear-gradient(135deg,${({ indigo: '#5b5df0', blue: '#2f7cf6', teal: '#0fa89a', rose: '#e5457a', amber: '#e08a0a', mono: '#555' })[a]},${({ indigo: '#8b5cf6', blue: '#22b8e8', teal: '#3bc9a0', rose: '#f0804a', amber: '#f2b13c', mono: '#999' })[a]})"></button>`).join('')}</div></div>
      <div class="setrow"><div class="t"><b>表格行样式</b></div>${seg(['细行线', '斑马纹'], S.zebra ? 1 : 0, 'zebra')}</div>
      <div class="setrow"><div class="t"><b>界面密度</b><div class="d">紧凑 26px / 标准 30px 行高</div></div>${seg(['紧凑', '标准'], 0, 'noop')}</div>
      <div class="setrow"><div class="t"><b>减弱动态效果</b><div class="d">关闭过渡与回弹，系统开启「减弱动态」时自动生效</div></div><span class="switch ${S.reduce ? 'on' : ''}" data-act="reduce"></span></div>`;
    const keys = [['打开命令面板', '⌘K'], ['新建查询', '⌘T'], ['执行语句', '⌘⏎'], ['提交修改', '⌘S'], ['AI 助手', '⌘I'], ['切换侧栏', '⌘B'], ['监控', '⌥⌘M'], ['设置', '⌘,']];
    let body;
    if (cur === '外观') body = look;
    else if (cur === '快捷键') body = keys.map(([a, k]) => `<div class="setrow"><div class="t">${a}</div><span class="kbd">${k}</span><button class="btn ghost sm">修改</button></div>`).join('');
    else if (cur === '关于') body = `<div class="col" style="align-items:center;gap:8px;padding:30px 0">${LOGO.glyph('peak', 72, { tile: true })}${LOGO.wordmark(24)}<span class="muted">版本 0.3.0 (build 412) · MyGo 0.9</span><div class="row gap8" style="margin-top:8px"><button class="btn sm">检查更新</button><button class="btn sm">更新日志</button></div></div>`;
    else body = (SET[cur] || []).map(r => `<div class="setrow"><div class="t"><b>${r[0]}</b>${r[3] ? `<div class="d">${r[3]}</div>` : ''}</div>${r[2] === 'sw' ? `<span class="switch ${r[1] ? 'on' : ''}" data-act="sw"></span>` : r[2] === 'btn' ? `<button class="btn sm">${r[1]}</button>` : `<div class="select" style="width:210px;height:26px;display:flex;align-items:center">${r[1]}<span class="sp"></span>${icon('ChevronD', 12)}</div>`}</div>`).join('');
    const ics = { 外观: 'Palette', 通用: 'Settings', 数据表: 'Table', 编辑器: 'Terminal', 连接: 'Plug', 安全: 'Lock', 备份: 'Save', AI: 'Sparkle', 快捷键: 'Keyboard', 关于: 'Info' };
    const order = ['通用', '外观', '数据表', '编辑器', '连接', '安全', '备份', 'AI', '快捷键', '关于'];
    return `<div class="scrim" data-act="close-ov"><div class="modal" style="width:780px;height:560px;flex-direction:row" onclick="event.stopPropagation()"><div class="setnav"><div class="h3" style="padding:4px 8px 8px">设置</div>${order.map(n => `<div class="tr ${n === cur ? 'on' : ''}" data-act="set-tab" data-k="${n}" style="height:28px">${icon(ics[n], 14)}${n}</div>`).join('')}</div>
      <div class="col grow" style="min-width:0"><div class="row" style="height:46px;padding:0 20px;border-bottom:1px solid var(--line)"><b class="h2 grow">${cur}</b><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div><div class="scroll grow" style="padding:6px 22px 18px">${body}</div></div></div></div>`;
  };
  const newtable = () => `<div class="scrim" data-act="close-ov"><div class="modal" style="width:420px" onclick="event.stopPropagation()"><div class="mh"><b class="h2 grow">新建表</b><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div><div class="mb col gap10" style="gap:10px"><div class="field" style="grid-template-columns:70px 1fr"><label>表名</label><div class="input focus" style="display:flex;align-items:center">invoices<span class="caret"></span></div></div><div class="field" style="grid-template-columns:70px 1fr"><label>从模板</label>${seg(['空表', 'id + 时间戳', '复制 orders'], 1, 'noop')}</div></div><div class="mf"><span class="muted" style="font-size:11px">进入结构页继续编辑列，应用前不会写库</span><div class="sp"></div><button class="btn" data-act="close-ov">取消</button><button class="btn primary" data-act="close-ov" data-nav="structure">创建并编辑</button></div></div></div>`;
  const exportQ = () => `<div class="scrim" data-act="close-ov"><div class="modal" style="width:440px" onclick="event.stopPropagation()"><div class="mh"><b class="h2 grow">导出结果</b><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div><div class="mb col gap10" style="gap:12px"><div class="row gap8">${[['CSV', 1], ['JSON', 0], ['Excel', 0], ['SQL', 0]].map(([t, on]) => `<div class="fmt ${on ? 'on' : ''}" style="flex:1;width:auto" data-act="fmt"><b>${t}</b></div>`).join('')}</div><div class="row"><span class="dim">包含表头</span><span class="sp"></span><span class="switch on" data-act="sw"></span></div><div class="row"><span class="dim">编码</span><span class="sp"></span><div class="select" style="width:140px;height:26px;display:flex;align-items:center">UTF-8 (BOM)<span class="sp"></span>${icon('ChevronD', 12)}</div></div></div><div class="mf"><span class="muted">4 行</span><div class="sp"></div><button class="btn" data-act="close-ov">取消</button><button class="btn primary" data-act="close-ov" data-toast="已导出 4 行到 ~/Downloads/result.csv">导出</button></div></div></div>`;

  function toastsHTML() { return `<div class="toasts">${S.toasts.map(t => `<div class="toast ${t.k} ${t.out ? 'out' : ''}"><span class="ti">${icon(({ ok: 'Check', err: 'X', info: 'Info', warn: 'Alert' })[t.k], 12)}</span><div class="col grow"><b>${t.t}</b>${t.s ? `<span class="muted" style="font-size:11px">${t.s}</span>` : ''}</div>${t.undo ? `<button class="btn ghost sm" data-act="toast-undo">撤销</button>` : ''}</div>`).join('')}</div>`; }

  window.renderOV = () => {
    const m = S.modal; let h = '';
    if (m === 'palette') h = palette(); else if (m === 'conn') h = connForm(); else if (m === 'settings') h = settings(); else if (m === 'sqlprev') h = sqlprev(); else if (m === 'newtable') h = newtable(); else if (m === 'export') h = exportQ(); else if (m === 'backup') h = window.backupDlg(); else if (m === 'restore') h = window.restoreDlg();
    else if (m === 'fpop') h = window.filterPop(); else if (m === 'connpop') h = S.scheme === 'A' ? connPopA() : connPop(); else if (m === 'dbpop') h = S.scheme === 'A' ? dbPopA() : connPop(); else if (m === 'bell') h = bellPop(); else if (m && m.startsWith('confirm:')) h = confirm(m.slice(8));
    else if (m === 'kill') h = confirm('kill');
    if (S.menu) h += `<div class="scrim" style="background:transparent;backdrop-filter:none;place-items:start" data-act="close-ov" oncontextmenu="return false"><div class="menu" style="left:${S.menu.x}px;top:${S.menu.y}px" onclick="event.stopPropagation()">${MENUS[S.menu.k]()}</div></div>`;
    h += toastsHTML();
    $('#ov').innerHTML = h;
    const pq = $('#palq'); if (pq && S._focusPal !== false) { pq.focus(); pq.setSelectionRange(pq.value.length, pq.value.length); }
  };
  window.MENU_KEYS = MENUS;
})();
