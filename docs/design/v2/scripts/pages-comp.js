/* 设计系统页：组件库——常见组件的样子、状态与用法（按钮、输入、选择、开关、分段与标签、标记、提示气泡、菜单、提示卡、进度、弹窗骨架）。
   所有样式来自 ui.css / v3.css，这里只是把它们摆在一起，落地时 Go 端的控件要和这里一致。 */
(function () {
  const sec = (t, d, body) => `<h2 style="margin-top:30px">${t}</h2>${d ? `<p class="lead" style="font-size:12.5px;margin:2px 0 10px">${d}</p>` : ''}<div class="card" style="padding:16px 18px;display:flex;flex-direction:column;gap:14px">${body}</div>`;
  const row = (label, items) => `<div class="row" style="gap:14px;align-items:center;flex-wrap:wrap"><span class="muted" style="width:92px;font-size:11px;flex:none">${label}</span>${items}</div>`;
  const tipS = (t, k) => `<span class="tip static">${t}${k ? `<span class="tk">${[...k].map(x => `<i>${x}</i>`).join('')}</span>` : ''}</span>`;
  PAGES.components = {
    group: 'sys', label: '组件库', icon: 'Layers',
    body: () => `<div class="doc page-in"><div class="eyebrow">COMPONENTS</div><h1 style="font-size:30px;letter-spacing:-.03em;margin:4px 0 8px">常见组件：一套形状、三档尺寸</h1><p class="lead">控件只有 22 / 26 / 30 三档高度（操作按钮 / 与输入框同排 / 对话框底部）；圆角 6，面板 10，对话框 12；强调色固定，不跟随连接颜色。</p>
      ${sec('按钮', '主按钮每个界面最多一个；危险操作用红色实心，默认焦点放在「取消」；图标按钮必须有提示气泡。',
        row('类型', `<button class="btn primary">${icon('Play', 12)}执行<span class="kbd">⌘⏎</span></button><button class="btn">默认</button><button class="btn soft">${icon('Sparkle', 12)}AI 编辑<span class="kbd">⌘K</span></button><button class="btn ghost">${icon('Copy', 13)}幽灵</button><button class="btn danger">删除</button>`) +
        row('尺寸', `<button class="btn sm">22 · 操作</button><button class="btn">26 · 同排</button><button class="btn lg">30 · 对话框</button>`) +
        row('状态', `<button class="btn">常态</button><button class="btn" style="background:var(--hover)">悬停</button><button class="btn" style="transform:scale(.96)">按下</button><button class="btn" disabled style="opacity:.45">禁用</button><button class="btn ghost icon on">${icon('Panel', 14)}</button><button class="btn ghost icon">${icon('Refresh', 14)}</button>`))}
      ${sec('输入', '输入框 26 高（对话框里 32）；聚焦时描边变强调色并带 3px 光环；错误时描边变红并在下面给一句话；搜索框左边带放大镜、右边带键帽。',
        row('文本', `<div class="input" style="width:180px">orders</div><div class="input focus" style="width:180px">sho<span class="caret"></span></div><div class="input" style="width:180px;border-color:var(--err)">1 = </div><div class="input" style="width:180px;opacity:.5">禁用</div>`) +
        row('搜索 / 选择', `<div class="search" style="width:180px">${icon('Search', 13)}<input placeholder="筛选表…"><span class="kbd">/</span></div><div class="select" style="width:160px;display:flex;align-items:center">utf8mb4<span class="sp"></span>${icon('ChevronD', 12)}</div>`))}
      ${sec('开关、勾选、单选', '开关即时生效；勾选用于多选清单；单选用于互斥的几项。',
        row('控件', `<span class="switch on"></span><span class="switch"></span><span class="check on">${icon('Check', 10)}</span><span class="check"></span><span class="radio on"></span><span class="radio"></span>`))}
      ${sec('分段与标签', '分段开关用于同一份数据的不同视图；下划线标签用于同一区域的几页；视图切换（数据 / 结构 / DDL / 文档）轨道高 26，与同一行的控件等高。',
        row('分段', `${seg(['数据', '图表'], 0)}${seg(['细行线', '斑马纹'], 1)}`) + row('视图切换', viewTabs(0)) + row('下划线标签', `<div class="tabs" style="height:28px"><button class="on">结果 1 <span class="n">4</span></button><button>消息 <span class="n">1</span></button><button>执行计划</button></div>`))}
      ${sec('标记', '环境标记 PROD / STG / DEV / LOCAL；状态芯片；键帽。',
        row('环境', `${tagOf('PROD')}${tagOf('STG')}${tagOf('DEV')}${tagOf('LOCAL')}`) + row('芯片', `<span class="chip ok">${icon('Check', 10)}成功</span><span class="chip warn">${icon('Alert', 10)}提醒</span><span class="chip err">失败</span><span class="chip info">InnoDB</span><span class="chip acc">进行中</span>`) + row('键帽', `<span class="kbd">⌘</span><span class="kbd">⇧⌘E</span><span class="kbd">esc</span><button class="btn primary">执行<span class="kbd">⌘⏎</span></button>`))}
      ${sec('悬浮提示（tooltip）', '悬停 450ms 后出现；三种写法：纯文字、文字 + 快捷键键帽、标题 + 说明。图标按钮、被截断的名称都要有；提示里不放可点击的内容。',
        row('纯文字', tipS('刷新')) + row('带快捷键', tipS('新建查询', '⌘T') + tipS('导出表…', '⇧⌘E')) +
        row('标题 + 说明', `<span class="tip static rich"><b>安全级别：提醒</b><span>危险语句（DROP、无 WHERE 的 UPDATE / DELETE）会先提醒，确认后才执行。</span></span>`))}
      ${sec('菜单', '菜单项 26 高，左图标右键帽；悬停是强调色实心行；危险项红字；分组之间一条细线。',
        `<div class="menu" style="position:static;width:230px">${[['Plus', '新建表…', '⌘N'], ['Download', '导出…', '⇧⌘E']].map(([i, t, k]) => `<div class="mi">${icon(i, 13)}<span>${t}</span><span class="kbd">${k}</span></div>`).join('')}<div class="msep"></div><div class="mi danger">${icon('Trash', 13)}<span>删除表…</span></div></div>`)}
      ${sec('提示卡与进度', '提示卡停在右下角 3.4 秒；进度条用于可估算的任务，流光条用于等待（数据加载、结果排序）。',
        row('提示卡', `<div class="toast ok" style="position:static;animation:none"><span class="ti">${icon('Check', 13)}</span><div class="col"><b>已复制</b><span class="muted" style="font-size:11px">12 行 · TSV</span></div></div>`) + row('进度', `<div class="pbar" style="width:220px"><i style="width:62%"></i></div><div style="position:relative;width:220px;height:2px;background:var(--surface-3)"><div class="loadbar"><i></i></div></div>`))}
    </div>`,
  };
})();
