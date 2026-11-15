/* 页面：导出 / 导入向导（各四步，可点步骤切换）、备份 / 恢复对话框、用户与权限、DDL / 文档视图 */
(function () {
  const seg = window.seg;
  /* ---------- 向导公共骨架 ---------- */
  // 左侧步骤条：已完成显示 ✓，当前高亮，可点击跳转
  const steps = (w, n, arr) => `<div class="col gap8" style="width:170px;padding:16px 14px;border-right:1px solid var(--line);background:var(--surface-2);flex:none;align-self:stretch">${arr.map((t, i) => `<div class="row gap8 stepi" data-act="io-step" data-w="${w}" data-n="${i}" style="color:${i === n ? 'var(--fg)' : 'var(--fg-3)'};font-weight:${i === n ? 600 : 500};cursor:default"><span class="stepn ${i < n ? 'done' : i === n ? 'cur' : ''}">${i < n ? icon('Check', 11) : i + 1}</span>${t}</div>`).join('')}</div>`;
  // 页脚：左侧提示，右侧 上一步 / 下一步（最后一步换成主操作）
  const foot = (w, n, hint, main, mainAct) => `<div class="row gap8" style="padding:10px 16px;border-top:1px solid var(--line);background:var(--surface-2)"><span class="muted">${hint}</span><div class="sp"></div>${n > 0 ? `<button class="btn" data-act="io-step" data-w="${w}" data-n="${n - 1}">上一步</button>` : ''}${n < 3 ? `<button class="btn primary" data-act="io-step" data-w="${w}" data-n="${n + 1}">下一步 ${icon('ArrowRight', 12)}</button>` : mainAct ? `<button class="btn primary" ${mainAct}>${main}</button>` : ''}</div>`;
  const wrap = (w, n, names, body, ft) => `<div class="row grow" style="min-height:0;align-items:stretch">${steps(w, n, names)}<div class="grow col" style="min-width:0"><div class="scroll grow page-in" style="padding:16px 22px;gap:16px;display:flex;flex-direction:column">${body}</div>${ft}</div></div>`;
  const sw = (k, v) => `<div class="row"><span class="dim">${k}</span><span class="sp"></span><span class="switch ${v ? 'on' : ''}" data-act="sw"></span></div>`;
  const sel = (t, w = 170) => `<div class="select" style="width:${w}px;display:flex;align-items:center">${t}<span class="sp"></span>${icon('ChevronD', 12)}</div>`;
  const tbl = n => `<div class="row gap8" style="height:30px;padding:0 12px;border-bottom:1px solid var(--line-soft)"><span class="check ${n[2] ? 'on' : ''}" data-act="chk">${icon('Check', 10)}</span>${icon(n[3] === 'v' ? 'Eye' : 'Table', 13, n[3] === 'v' ? 'ico-v' : 'ico-t')}<b>${n[0]}</b><span class="muted sp" style="text-align:right">${n[1]}</span></div>`;
  const OBJ = [['orders', '12,431 行', 1], ['order_items', '31,204 行', 1], ['payments', '11,870 行', 1], ['users', '5,300 行', 1], ['products', '840 行', 0], ['categories', '64 行', 0], ['coupons', '212 行', 0], ['v_daily_sales', '视图', 0, 'v'], ['v_order_summary', '视图', 0, 'v']];
  const doneRow = (n, r, st) => `<div class="row gap8" style="height:34px;padding:0 12px;border-bottom:1px solid var(--line-soft)">${st === 'ok' ? `<span class="stepn done" style="width:16px;height:16px">${icon('Check', 10)}</span>` : st === 'run' ? '<i class="spin" style="width:13px;height:13px"></i>' : '<span class="dot" style="margin:0 4px"></span>'}${icon('Table', 13, 'ico-t')}<b>${n}</b><span class="sp"></span>${st === 'run' ? '<div class="pbar" style="width:120px"><i style="width:62%"></i></div>' : ''}<span class="mono ${st === 'wait' ? 'muted' : 'dim'}" style="min-width:110px;text-align:right">${r}</span></div>`;

  /* ---------- 导出 ---------- */
  const EX = ['选择对象', '格式与选项', '目标位置', '执行'];
  PAGES.export = {
    group: 'app', label: '导出', icon: 'Download', tab: { icon: 'Download', label: '导出' },
    tools: () => `${seg(['导出', '导入'], 0, 'io')}<div class="sp"></div><span class="muted" style="font-size:11px">不会修改源库</span>`,
    body: () => {
      const n = S.exStep | 0;
      const b = [
        `<div class="row gap8"><div class="search grow" style="max-width:320px"><span class="muted">${icon('Search', 13)}</span><input class="input" placeholder="筛选表和视图…" style="border:0;box-shadow:none;background:none;padding:0"></div><button class="btn sm">${icon('CheckAll', 12)}全选</button><button class="btn sm">反选</button><span class="sp"></span><span class="muted">已选 <b class="dim">4</b> / 9 · 约 59.8K 行</span></div>
         <div class="card" style="overflow:hidden">${OBJ.map(tbl).join('')}</div>
         <div class="row gap8"><span class="dim">导出的列</span><span class="muted" style="font-size:11px">orders · 6 / 6</span><span class="sp"></span><button class="btn ghost sm">自定义列…</button></div>`,
        `<div><div class="h3">格式</div><div class="row gap8" style="margin-top:8px">${[['SQL', '结构 + 数据', 1], ['CSV', '逐表一个文件', 0], ['JSON', '数组 / 行', 0], ['Excel', '.xlsx', 0], ['Markdown', '文档用', 0]].map(([t, s, on]) => `<div class="fmt ${on ? 'on' : ''}" data-act="fmt"><b>${t}</b><span class="muted" style="font-size:10.5px">${s}</span></div>`).join('')}</div></div>
         <div class="col gap8"><div class="h3">选项</div>${sw('包含建表语句', 1)}${sw('包含数据', 1)}${sw('每 1,000 行合并为一条 INSERT', 1)}${sw('写入 DROP TABLE IF EXISTS', 0)}${sw('压缩为 .gz', 0)}</div>`,
        `<div class="col gap8"><div class="h3">保存到</div><div class="row gap8"><div class="input mono grow" style="display:flex;align-items:center">~/Downloads/shop-2026-11-15</div><button class="btn">${icon('Folder', 13)}选择…</button></div></div>
         <div class="col gap8"><div class="h3">文件</div>${seg(['每表一个文件', '合并为一个文件', '打包 .zip'], 0)}<div class="field" style="grid-template-columns:96px 1fr;margin-top:6px"><label>文件名模板</label><div class="input mono" style="display:flex;align-items:center">{table}.sql</div></div><div class="field" style="grid-template-columns:96px 1fr"><label>行数上限</label><div class="input" style="width:140px;display:flex;align-items:center">全部行</div></div></div>
         <div class="card" style="padding:10px 14px;background:var(--surface-2)"><div class="muted" style="font-size:11px">将生成 4 个文件</div><div class="mono dim" style="font-size:11.5px;line-height:1.7;margin-top:4px">orders.sql · order_items.sql · payments.sql · users.sql</div></div>`,
        `<div class="row gap8"><span class="chip ok">${icon('Check', 10)}已完成 2</span><span class="chip acc">进行中 1</span><span class="chip">等待 1</span><span class="sp"></span><span class="mono dim">已写入 18.4 MB / 约 21 MB</span></div>
         <div class="pbar"><i style="width:68%"></i></div>
         <div class="card" style="overflow:hidden">${doneRow('orders', '12,431 行 · 4.2 MB', 'ok')}${doneRow('order_items', '31,204 行 · 10.6 MB', 'ok')}${doneRow('payments', '6,420 / 11,870 行', 'run')}${doneRow('users', '等待', 'wait')}</div>
         <div class="row gap8"><button class="btn">${icon('Stop', 12)}取消</button><span class="sp"></span><span class="muted" style="font-size:11px">完成后可在文件夹中查看；失败的表会单独列出，不影响其余表</span></div>`,
      ][n];
      const ft = foot('ex', n, ['已选 4 张表 · 预计 59.8K 行', 'SQL · 预计约 21 MB', '共 4 个文件 · 约 21 MB', '用时 3.1 s'][n], `导出 <span class="kbd">⌘⏎</span>`, `data-act="io-step" data-w="ex" data-n="3"`);
      return wrap('ex', n, EX, b, n === 3 ? foot2('ex', '完成后打开文件夹', `${icon('Folder', 12)}打开文件夹`) : ft);
    },
  };
  // 执行步骤的页脚：只有「上一步 / 完成」
  const foot2 = (w, hint, main) => `<div class="row gap8" style="padding:10px 16px;border-top:1px solid var(--line);background:var(--surface-2)"><span class="muted">${hint}</span><div class="sp"></div><button class="btn" data-act="io-step" data-w="${w}" data-n="0">再来一次</button><button class="btn primary" data-act="toast" data-msg="已打开 ~/Downloads/shop-2026-11-15" data-k="ok">${main}</button></div>`;

  /* ---------- 导入 ---------- */
  const IM = ['选择文件', '目标表', '字段映射', '导入'];
  PAGES.import = {
    group: 'app', label: '导入', icon: 'Upload', tab: { icon: 'Upload', label: '导入' },
    tools: () => `${seg(['导出', '导入'], 1, 'io')}<div class="sp"></div><span class="chip warn">${icon('Alert', 11)}会写入数据库</span>`,
    body: () => {
      const n = S.imStep | 0;
      const map = [['order_id', 'id', 'bigint', '10232'], ['uid', 'user_id', 'bigint', '1042'], ['state', 'status', 'enum', 'paid'], ['amount', 'total', 'decimal', '128.00'], ['ts', 'created_at', 'datetime', '2026-11-15 10:31:00'], ['remark', 'note', 'text', '—']];
      const b = [
        `<div class="col" style="border:1.5px dashed var(--acc-line);background:var(--acc-soft);border-radius:var(--r-xl);height:150px;align-items:center;justify-content:center;gap:6px"><span style="color:var(--acc-text)">${icon('Upload', 26)}</span><b>把文件拖到这里，或 <span style="color:var(--acc-text)">选择文件…</span></b><span class="muted" style="font-size:11px">支持 CSV · TSV · JSON · SQL · Excel</span></div>
         <div class="card row gap12" style="padding:12px 14px">${icon('File', 20)}<div class="col grow"><b>orders_2026-11.csv</b><span class="muted" style="font-size:11px">8.4 MB · 62,108 行 · UTF-8 · 逗号分隔 · 首行为列名（已自动识别）</span></div><button class="btn sm">更换</button></div>
         <div class="row gap8"><span class="dim">编码</span>${sel('UTF-8', 110)}<span class="dim" style="margin-left:8px">分隔符</span>${sel('逗号 ,', 100)}<span class="sp"></span>${sw2('首行为列名', 1)}</div>
         <div class="card" style="overflow:hidden"><table class="grid"><thead><tr>${['order_id', 'uid', 'state', 'amount', 'ts', 'remark'].map(h => `<th>${h}</th>`).join('')}</tr></thead><tbody>${map.slice(0, 3).map((r, i) => `<tr><td class="mono">${10232 - i}</td><td class="mono">${1042 + i}</td><td>${['paid', 'pending', 'paid'][i]}</td><td class="mono">${['128.00', '56.50', '9.90'][i]}</td><td class="mono">2026-11-15 10:3${i}:00</td><td class="muted">—</td></tr>`).join('')}</tbody></table></div>`,
        `<div class="row gap8"><span class="dim">导入到</span>${sel('orders', 190)}<span class="muted" style="font-size:11px">已有表 · 6 列</span><span class="sp"></span>${seg(['已有表', '新建表'], 0)}</div>
         <div class="col gap8"><div class="h3">写入方式</div>${seg(['追加', '覆盖', '更新'], 0)}<span class="muted" style="font-size:11px">追加：只插入新行；覆盖：先清空再导入（会先弹出确认）；更新：按主键 id 合并</span></div>
         <div class="col gap8"><div class="h3">选项</div>${sw('在单个事务中执行，失败整体回滚', 1)}${sw('遇到重复主键时跳过', 1)}${sw('导入前清空目标表', 0)}${sw('空字符串按 NULL 处理', 1)}</div>`,
        `<div class="row gap8"><span class="dim">导入到</span><b>orders</b>${seg(['追加', '覆盖', '更新'], 0)}<span class="sp"></span><span class="chip ok">${icon('Check', 10)}6 / 6 列已匹配</span></div>
         <div class="card" style="overflow:hidden"><table class="grid"><thead><tr><th>CSV 列</th><th></th><th>目标列</th><th>类型</th><th>预览</th></tr></thead><tbody>${map.map(r => `<tr><td class="mono">${r[0]}</td><td class="muted">${icon('ArrowRight', 12)}</td><td><div class="select" style="height:22px;display:flex;align-items:center;width:140px;font-family:var(--mono)">${r[1]}<span class="sp"></span>${icon('ChevronD', 11)}</div></td><td class="muted mono">${r[2]}</td><td class="mono dim">${r[3]}</td></tr>`).join('')}</tbody></table></div>`,
        `<div class="row"><span class="dim">导入进度</span><span class="sp"></span><span class="mono dim">38,420 / 62,108</span></div><div class="pbar"><i style="width:62%"></i></div>
         <div class="g3" style="display:grid;grid-template-columns:repeat(3,1fr);gap:10px">${[['已写入', '38,417', 'ok'], ['跳过（重复主键）', '3', 'warn'], ['错误', '0', 'ok']].map(([t, v, k]) => `<div class="card" style="padding:10px 14px"><div class="muted" style="font-size:11px">${t}</div><div style="font-size:20px;font-weight:700;color:var(--${k});margin-top:2px" class="mono">${v}</div></div>`).join('')}</div>
         <div class="card" style="padding:10px 14px"><div class="muted" style="font-size:11px;margin-bottom:4px">跳过的行</div><div class="mono dim" style="font-size:11.5px;line-height:1.7">第 1,204 行 · Duplicate entry '10031' for key 'PRIMARY'<br>第 8,992 行 · Duplicate entry '9120' for key 'PRIMARY'<br>第 21,557 行 · Duplicate entry '7710' for key 'PRIMARY'</div></div>
         <span class="muted" style="font-size:11px">剩余约 4 s · 在单个事务中执行，失败整体回滚</span>`,
      ][n];
      const ft = foot('im', n, ['已识别 62,108 行 · 6 列', '目标：orders · 追加', '在单个事务中执行，失败整体回滚', ''][n], '开始导入', `data-act="toast" data-msg="导入完成：38,417 行，用时 4.1 s" data-k="ok"`);
      return wrap('im', n, IM, b, ft.replace('下一步', n === 2 ? '开始导入' : '下一步'));
    },
  };
  function sw2(k, v) { return `<span class="row gap8"><span class="dim">${k}</span><span class="switch ${v ? 'on' : ''}" data-act="sw"></span></span>`; }

  /* ---------- 备份 / 恢复对话框（从活动栏「导入导出」菜单进入） ---------- */
  const dlg = (w, title, sub, body, foot) => `<div class="scrim" data-act="close-ov"><div class="modal" style="width:${w}px" onclick="event.stopPropagation()"><div class="mh"><span class="tile-ic">${icon(sub, 16)}</span><b class="h2 grow">${title}</b><button class="btn ghost icon sm" data-act="close-ov">${icon('X', 14)}</button></div><div class="mb col" style="gap:12px">${body}</div><div class="mf">${foot}</div></div></div>`;
  window.backupDlg = () => dlg(520, '备份数据库', 'Backup', `
    <div class="field" style="grid-template-columns:70px 1fr"><label>数据库</label>${sel('shop', 200)}</div>
    <div class="field" style="grid-template-columns:70px 1fr"><label>内容</label>${seg(['结构 + 数据', '仅结构', '仅数据'], 0)}</div>
    <div class="field" style="grid-template-columns:70px 1fr"><label>保存到</label><div class="row gap8"><div class="input mono grow" style="display:flex;align-items:center">~/Backups/shop-2026-11-15.sql.gz</div><button class="btn">${icon('Folder', 13)}</button></div></div>
    ${sw('包含存储过程、触发器、事件', 1)}${sw('使用单一事务（一致性快照）', 1)}${sw('压缩为 .gz', 1)}`,
    `<span class="muted" style="font-size:11px">9 张表 · 预计约 38 MB</span><div class="sp"></div><button class="btn" data-act="close-ov">取消</button><button class="btn primary" data-act="close-ov" data-toast="备份完成：shop → shop-2026-11-15.sql.gz（37.6 MB）">开始备份</button>`);
  window.restoreDlg = () => dlg(520, '恢复备份', 'Upload', `
    <div class="card row gap12" style="padding:10px 14px">${icon('File', 18)}<div class="col grow"><b>shop-2026-11-08.sql.gz</b><span class="muted" style="font-size:11px">36.9 MB · 9 张表 · 11 月 8 日 03:00 · MySQL 8.0.36</span></div><button class="btn sm">更换</button></div>
    <div class="field" style="grid-template-columns:70px 1fr"><label>恢复到</label>${sel('shop_test', 200)}</div>
    ${sw('恢复前清空目标库中的同名表', 0)}${sw('遇到错误时继续', 0)}
    <div class="card row gap8" style="padding:9px 12px;border-color:color-mix(in srgb,var(--warn) 40%,var(--line));background:var(--warn-soft);color:var(--warn)">${icon('Alert', 14)}<span style="color:var(--fg-2)">会向 <b>shop_test</b> 写入 9 张表，已存在的同名表会报错。建议先备份。</span></div>`,
    `<span class="muted" style="font-size:11px">在单个事务中执行（失败整体回滚）</span><div class="sp"></div><button class="btn" data-act="close-ov">取消</button><button class="btn danger" data-act="close-ov" data-toast="恢复完成：9 张表，用时 5.2 s">开始恢复</button>`);

  /* ---------- 用户与权限 ---------- */
  const U = [['root', '%', '管理员', 'warn'], ['app_rw', '10.0.%', '读写'], ['app_ro', '10.0.%', '只读'], ['report', '192.168.1.%', '只读'], ['backup', 'localhost', '备份'], ['legacy', '%', '已锁定', 'err']];
  PAGES.users = {
    group: 'app', label: '用户与权限', icon: 'Users', tab: { icon: 'Users', label: '用户与权限' },
    tools: () => `<div class="h3">用户与权限</div><span class="muted">shop-dev · MySQL 8.0.36</span><div class="sp"></div><button class="btn sm">${icon('Plus', 12)}新建用户</button><button class="btn ghost icon sm" data-tip="刷新">${icon('Refresh', 14)}</button>`,
    body: () => {
      const privs = ['SELECT', 'INSERT', 'UPDATE', 'DELETE', 'CREATE', 'DROP', 'ALTER', 'INDEX'];
      const rows = [['全局 *.*', [0, 0, 0, 0, 0, 0, 0, 0]], ['shop.*', [1, 1, 1, 1, 0, 0, 0, 1]], ['shop.orders', [1, 1, 1, 0, 0, 0, 0, 0]], ['shop.payments', [1, 0, 0, 0, 0, 0, 0, 0]], ['mysql.*', [0, 0, 0, 0, 0, 0, 0, 0]]];
      return `<div class="row grow" style="min-height:0;align-items:stretch">
        <div class="col" style="width:240px;border-right:1px solid var(--line);flex:none"><div style="padding:8px"><div class="search"><span class="muted">${icon('Search', 13)}</span><input class="input" placeholder="搜索用户…" style="border:0;box-shadow:none;background:none;padding:0"></div></div>
          <div class="scroll grow col" style="padding:0 6px;gap:1px">${U.map((u, i) => `<div class="tr ${i === 1 ? 'on' : ''}" style="height:44px;gap:9px"><span class="avatar" style="width:26px;height:26px;border-radius:50%;background:var(--acc-soft);color:var(--acc-text);display:grid;place-items:center;font-weight:700;font-size:11px;flex:none">${u[0][0].toUpperCase()}</span><div class="col grow" style="min-width:0;line-height:1.3"><b class="trunc">${u[0]}</b><span class="muted mono" style="font-size:10.5px">@${u[1]}</span></div><span class="chip ${u[3] || ''}">${u[2]}</span></div>`).join('')}</div></div>
        <div class="grow col" style="min-width:0"><div class="row gap12" style="padding:14px 20px;border-bottom:1px solid var(--line)"><span class="avatar" style="width:38px;height:38px;border-radius:50%;background:var(--acc-soft);color:var(--acc-text);display:grid;place-items:center;font-weight:700;font-size:15px">A</span><div class="col grow"><b style="font-size:14px">app_rw<span class="muted mono" style="font-weight:500">@10.0.%</span></b><span class="muted" style="font-size:11px">创建于 2025-03-02 · 最近登录 今天 10:24 · 连接数上限 50</span></div><button class="btn sm">${icon('Pencil', 12)}重置密码</button><button class="btn sm danger-soft">${icon('Trash', 12)}删除</button></div>
          <div class="row" style="padding:0 20px;height:36px;border-bottom:1px solid var(--line);flex:none"><div class="tabs" style="height:100%"><button>账户</button><button class="on">权限 <span class="n">4</span></button><button>资源限制</button><button>SQL 预览</button></div></div>
          <div class="scroll grow" style="padding:14px 20px;display:flex;flex-direction:column;gap:12px"><div class="row gap8"><div class="h3">授权矩阵</div><span class="muted" style="font-size:11px">按库 / 表授权；勾选「全部权限」会覆盖下面各项</span><span class="sp"></span><button class="btn sm">${icon('Plus', 12)}添加对象</button></div>
            <div class="card" style="overflow:hidden"><table class="grid"><thead><tr><th>对象</th>${privs.map(p => `<th style="text-align:center;font-size:10.5px">${p}</th>`).join('')}</tr></thead><tbody>${rows.map((r, i) => `<tr><td class="mono"><b>${r[0]}</b></td>${r[1].map((v, j) => `<td class="${i === 3 && j === 1 ? 'mod' : ''}" style="text-align:center"><span class="check ${v ? 'on' : ''}" data-act="chk">${icon('Check', 10)}</span></td>`).join('')}</tr>`).join('')}</tbody></table></div>
            ${sw('允许其再授权给他人（WITH GRANT OPTION）', 0)}</div></div></div>
        <div class="islandbar"><span class="dot warn"></span><b>2 项权限修改</b><span class="sub">授予 1 · 撤销 1</span><span class="vr" style="background:rgba(255,255,255,.2)"></span><button class="btn ghost sm" data-act="modal" data-m="sqlprev">${icon('Code', 12)}预览 SQL</button><button class="btn ghost sm">丢弃</button><button class="btn primary sm" data-act="toast" data-msg="已应用 2 项权限修改" data-k="ok">应用 <span class="kbd">⌘S</span></button></div>`;
    },
    status: () => `<span class="it">${icon('Shield', 11)}仅管理员账户可修改</span><span class="it">6 个用户</span>`,
  };

  /* ---------- DDL / 文档视图（表页的第 3、4 个标签） ---------- */
  const DDL = `CREATE TABLE \`orders\` (
  \`id\`         bigint        NOT NULL AUTO_INCREMENT,
  \`user_id\`    bigint        NOT NULL,
  \`status\`     enum('pending','paid','shipped','refunded') NOT NULL DEFAULT 'pending',
  \`total\`      decimal(10,2) NOT NULL DEFAULT 0.00,
  \`created_at\` datetime      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  \`note\`       text          NULL,
  PRIMARY KEY (\`id\`),
  KEY \`idx_orders_user\` (\`user_id\`),
  KEY \`idx_orders_created\` (\`created_at\`),
  CONSTRAINT \`fk_orders_user\` FOREIGN KEY (\`user_id\`) REFERENCES \`users\` (\`id\`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';`;
  PAGES.ddl = {
    group: 'app', label: 'DDL', icon: 'Code', noTab: true, tabOf: 'data',
    tools: () => `${viewTabs(2)}<div class="sp"></div><button class="btn ghost sm" data-act="toast" data-msg="已复制建表语句" data-k="ok">${icon('Copy', 13)}复制</button><button class="btn ghost sm" data-act="page" data-p="sql">${icon('Terminal', 13)}在新查询中打开</button><button class="btn ghost icon sm" data-tip="刷新">${icon('Refresh', 14)}</button>`,
    body: () => `<div class="scroll grow page-in" style="padding:14px 18px"><div class="card sql" style="padding:14px 16px;font-size:12.5px;line-height:1.75;white-space:pre">${hl(DDL)}</div><div class="row gap8" style="margin-top:12px"><span class="chip">${icon('Lock', 10)}只读</span><span class="muted" style="font-size:11px">这是服务器当前的建表语句；要改结构请到「结构」标签，改完在那里预览 ALTER</span></div></div>`,
    status: () => `<span class="it">InnoDB · utf8mb4 · 6 列 · 3 个索引 · 1 个外键</span>`,
  };
  /* 文档页（参考图）：右上「AI 补全注释」「复制 Markdown」；标题 + 表备注 chip；一句说明（引用其他表用代码样式，没写时给「AI 草拟」）；
     字段表三列：字段（代码小药丸）/ 类型（大写灰字）/ 说明（AI 草拟的说明尾部带 ✦ AI，人写的没有） */
  PAGES.doc = {
    group: 'app', label: '文档', icon: 'File', noTab: true, tabOf: 'data',
    tools: () => `${viewTabs(3)}<div class="sp"></div><button class="btn ghost sm" data-act="toast" data-msg="已让 AI 补全缺失的注释" data-k="ok">${icon('Sparkle', 13)}AI 补全注释</button><button class="btn ghost sm" data-act="toast" data-msg="已复制为 Markdown" data-k="ok">${icon('Copy', 13)}复制 Markdown</button>`,
    body: () => `<div class="scroll grow page-in docv"><div><div class="dh"><h2>users</h2><span class="chip info">用户表</span></div><div class="dd">存放注册用户的基础资料与账户状态，被 <code>orders</code>、<code>addresses</code> 引用。<span class="aitag">${icon('Sparkle', 12)}AI 草拟</span></div></div>
      <table><thead><tr><th style="width:22%">字段</th><th style="width:24%">类型</th><th>说明</th></tr></thead><tbody>${[['id', 'integer', '主键，自增', 0], ['name', 'varchar(64)', '用户显示名', 0], ['email', 'varchar(190)', '登录邮箱，唯一', 1], ['age', 'integer', '年龄（岁），可空', 1], ['status', 'enum', '账号状态：active / pending / banned', 1], ['balance', 'decimal(10,2)', '账户余额（元）', 1], ['created_at', 'datetime', '注册时间', 0]].map(r => `<tr><td><span class="fn">${r[0]}</span></td><td class="ty">${r[1]}</td><td>${r[2]}${r[3] ? `<span class="aitag">${icon('Sparkle', 11)}AI</span>` : ''}</td></tr>`).join('')}</tbody></table></div>`,
    status: () => `<span class="it">说明来自表和列的备注，在「结构」标签里编辑；带 ✦ AI 的是 AI 草拟、还没写入备注</span>`,
  };
})();
