/* 示例数据：电商库 shop。内容刻意「不完美」——有 NULL、长文本、失败语句、慢查询，用来检验布局。 */
window.DB = {
  // 连接：type 决定图标底色；env 是环境；pin 固定；used 最近使用
  conns: [
    { id: 'shop-prod', name: 'shop-prod', type: 'MySQL', ver: '8.0.36', host: 'rds-prod.cn-east-1:3306', env: 'PROD', folder: '生产', used: '2 小时前', open: true, ro: false, ssh: true, pin: true, color: '#e5484d' },
    { id: 'analytics', name: 'analytics', type: 'PostgreSQL', ver: '16.2', host: 'pg-ro.internal:5432', env: 'PROD', folder: '生产', used: '昨天', ro: true, color: '#3b6fb6' },
    { id: 'ledger', name: 'ledger', type: 'TiDB', ver: '8.1', host: 'tidb-01.internal:4000', env: 'PROD', folder: '生产', used: '3 天前', ssh: true, color: '#7c5cd6' },
    { id: 'shop-stg', name: 'shop-staging', type: 'MySQL', ver: '8.0.36', host: 'stg-db.internal:3306', env: 'STG', folder: '测试', used: '周一', color: '#d9881a' },
    { id: 'warehouse', name: 'warehouse', type: 'ClickHouse', ver: '24.3', host: 'ch-01.internal:8123', env: 'STG', folder: '测试', used: '上周', color: '#c9a30a' },
    { id: 'crm', name: 'crm', type: 'MariaDB', ver: '11.4', host: 'crm-test.internal:3306', env: 'STG', folder: '测试', used: '11 月 2 日', color: '#1a8f87' },
    { id: 'shop-dev', name: 'shop-dev', type: 'MySQL', ver: '8.0.36', host: 'localhost:3306', env: 'DEV', folder: '本地', used: '刚刚', open: true, pin: true, color: '#2f9e5f' },
    { id: 'local', name: 'local-sqlite', type: 'SQLite', ver: '3.46', host: '~/Data/app.db', env: 'LOCAL', folder: '本地', used: '今天', color: '#3a86d1' },
    { id: 'sr', name: 'starrocks-lab', type: 'StarRocks', ver: '3.3', host: '127.0.0.1:9030', env: 'DEV', folder: '本地', used: '10 月 28 日', color: '#2a9d6a' },
  ],
  abbr: { MySQL: 'My', PostgreSQL: 'Pg', SQLite: 'SQ', MariaDB: 'Ma', ClickHouse: 'CH', TiDB: 'Ti', StarRocks: 'SR', 'SQL Server': 'MS' },
  dbs: [['shop', '9 张表', true], ['shop_test', '6 张表'], ['analytics', '14 张表'], ['information_schema'], ['mysql']],
  tables: [
    ['categories', 64], ['coupons', 212], ['orders', 12431], ['order_items', 31204], ['payments', 11870], ['products', 840], ['users', 5300],
  ],
  views: ['v_daily_sales', 'v_order_summary'],
  cols: [ // 订单表
    { n: 'id', t: 'bigint', k: 'pk', cls: 'pk r' }, { n: 'user_id', t: 'bigint', cls: 'num r', fk: 'users.id' }, { n: 'status', t: 'enum' },
    { n: 'total', t: 'decimal(10,2)', cls: 'num r' }, { n: 'created_at', t: 'datetime', cls: 'time' }, { n: 'note', t: 'text' },
  ],
  orders: (() => {
    const st = ['paid', 'pending', 'shipped', 'paid', 'refunded', 'paid', 'shipped', 'pending'];
    const notes = [null, '请尽快发货', '发票抬头：星河科技', null, '客户要求周末送达', null, '礼品包装', null, '重复下单，已联系客户确认'];
    return Array.from({ length: 30 }, (_, i) => {
      const id = 10231 - i, d = new Date(2026, 10, 15, 10, 24 - i * 37);
      const p = x => String(x).padStart(2, '0');
      return [id, 1000 + ((i * 97 + 31) % 900), st[i % st.length], (((i * 137) % 480) + 12.9).toFixed(2),
        `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`, notes[i % notes.length]];
    });
  })(),
  struct: [
    ['id', 'bigint unsigned', 1, 1, 0, '', '主键'], ['sku', 'varchar(32)', 0, 0, 0, '', '商品编码，唯一'], ['name', 'varchar(120)', 0, 0, 0, '', '名称'],
    ['category_id', 'int', 0, 0, 1, 'NULL', '→ categories.id'], ['price', 'decimal(10,2)', 0, 0, 0, '0.00', '单价，元'], ['stock', 'int', 0, 0, 0, '0', '库存'],
    ['status', "enum('draft','active','archived')", 0, 0, 0, "'draft'", '上下架状态'], ['weight_g', 'int', 0, 0, 1, 'NULL', '重量（克）'], ['legacy_flag', 'tinyint(1)', 0, 0, 1, 'NULL', '历史遗留，待删除'],
  ],
  indexes: [['PRIMARY', ['id'], 'BTREE', 1], ['uk_sku', ['sku'], 'BTREE', 1], ['idx_category', ['category_id'], 'BTREE', 0], ['idx_status_created', ['status', 'created_at'], 'BTREE', 0]],
  history: [
    { sql: "SELECT status, COUNT(*) FROM orders GROUP BY status", ok: 1, t: '10:24', d: '12 ms', r: '5 行', day: '今天' },
    { sql: "UPDATE orders SET status='shipped' WHERE id IN (10231, 10230)", ok: 1, t: '10:19', d: '3 ms', r: '影响 2 行', day: '今天' },
    { sql: "SELECT foo FROM orders LIMIT 5", ok: 0, t: '10:12', d: '', r: "Unknown column 'foo'", day: '今天' },
    { sql: "SHOW CREATE TABLE orders", ok: 1, t: '09:58', d: '2 ms', r: '1 行', day: '今天' },
    { sql: "SELECT * FROM users WHERE id = 1", ok: 1, t: '17:41', d: '1 ms', r: '1 行', day: '昨天' },
  ],
  saved: [['报表', ['月度对账', '日销售额 Top 10', '复购率']], ['排查', ['未支付超 30 分钟', '重复手机号']]],
  sql: `-- 近 7 天各状态订单的数量与金额
SELECT o.status,
       COUNT(*)            AS orders,
       SUM(o.total)        AS revenue,
       ROUND(AVG(o.total), 2) AS avg_total
FROM   orders o
JOIN   users  u ON u.id = o.user_id
WHERE  o.created_at >= NOW() - INTERVAL 7 DAY
GROUP  BY o.status
ORDER  BY revenue DESC;`,
  result: [['paid', 7420, '1,284,903.20', '173.18'], ['shipped', 3105, '502,118.40', '161.71'], ['pending', 1288, '203,411.90', '157.93'], ['refunded', 618, '98,204.00', '158.91']],
  // ER：实体与外键
  er: [
    { n: 'users', x: 40, y: 40, c: [['id', 'bigint', 'pk'], ['email', 'varchar(120)'], ['name', 'varchar(60)'], ['created_at', 'datetime']] },
    { n: 'orders', x: 330, y: 70, c: [['id', 'bigint', 'pk'], ['user_id', 'bigint', 'fk'], ['status', 'enum'], ['total', 'decimal'], ['created_at', 'datetime']] },
    { n: 'order_items', x: 640, y: 40, c: [['id', 'bigint', 'pk'], ['order_id', 'bigint', 'fk'], ['product_id', 'bigint', 'fk'], ['qty', 'int'], ['price', 'decimal']] },
    { n: 'products', x: 640, y: 270, c: [['id', 'bigint', 'pk'], ['sku', 'varchar(32)'], ['category_id', 'int', 'fk'], ['price', 'decimal']] },
    { n: 'categories', x: 330, y: 320, c: [['id', 'int', 'pk'], ['name', 'varchar(60)'], ['parent_id', 'int', 'fk']] },
    { n: 'payments', x: 40, y: 290, c: [['id', 'bigint', 'pk'], ['order_id', 'bigint', 'fk'], ['channel', 'varchar(20)'], ['amount', 'decimal']] },
  ],
  fks: [['orders', 'users'], ['order_items', 'orders'], ['order_items', 'products'], ['products', 'categories'], ['payments', 'orders'], ['categories', 'categories']],
  plan: [
    { d: 0, op: 'Sort', obj: 'revenue DESC', rows: '4', cost: 1820, ms: 0.6, hot: 0 },
    { d: 1, op: 'Aggregate', obj: 'GROUP BY o.status', rows: '4', cost: 1790, ms: 4.1, hot: 0 },
    { d: 2, op: 'Hash Join', obj: 'o.user_id = u.id', rows: '12,431', cost: 1604, ms: 38.7, hot: 1 },
    { d: 3, op: 'Index Range Scan', obj: 'orders · idx_status_created', rows: '12,431', cost: 902, ms: 9.2, hot: 0 },
    { d: 3, op: 'Index Scan', obj: 'users · PRIMARY', rows: '5,300', cost: 310, ms: 2.4, hot: 0 },
  ],
  ai: [
    { r: 'u', t: '最近 7 天哪个状态的订单金额最高？顺便看看有没有异常的退款。' },
    { r: 'a', tools: [['Zap', '读取结构', 'orders, payments', 'ok'], ['Play', '运行 SQL', 'SELECT status, SUM(total) …', 'ok']], t: '最近 7 天 **paid** 的金额最高，约 128.5 万；**refunded** 有 618 单、9.8 万，占比 7.6%，略高于上月的 5.9%。', sql: "SELECT DATE(created_at) d, COUNT(*) FROM orders\nWHERE status='refunded' AND created_at >= NOW() - INTERVAL 7 DAY\nGROUP BY d ORDER BY d;" },
  ],
  procs: [
    [1881, 'app_rw', 'shop', 'Query', '12.4 s', 'SELECT … FROM order_items oi JOIN orders o ON …'], [1907, 'app_ro', 'shop', 'Query', '4.1 s', 'SELECT COUNT(*) FROM payments WHERE …'], [1923, 'etl', 'shop', 'Sleep', '210 s', '—'],
  ],
  slow: [
    ['SELECT … FROM order_items oi JOIN orders o …', 214, '1.83 s', '12.4 s', '31,204', '10:22'], ['SELECT * FROM payments WHERE ref LIKE \'%…\'', 96, '920 ms', '4.1 s', '11,870', '10:18'], ['UPDATE products SET stock = stock - ? WHERE id = ?', 1480, '210 ms', '2.2 s', '1', '10:24'],
  ],
  settingsNav: ['通用', '数据表', '编辑器', '连接', '外观', '安全', '备份', 'AI', '快捷键', '关于'],
};
// 迷你折线：确定性伪随机，避免每次刷新都变
window.series = (n, seed = 1, base = 50, amp = 30) => { let s = seed; return Array.from({ length: n }, (_, i) => { s = (s * 9301 + 49297) % 233280; return base + Math.sin(i / 3 + seed) * amp * .6 + (s / 233280 - .5) * amp; }); };
window.spark = (arr, w = 120, h = 32, fill = true) => {
  const mn = Math.min(...arr), mx = Math.max(...arr), dx = w / (arr.length - 1);
  const pts = arr.map((v, i) => [i * dx, h - 3 - ((v - mn) / (mx - mn || 1)) * (h - 6)]);
  const d = pts.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ' ' + p[1].toFixed(1)).join('');
  return `<svg width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" style="display:block;overflow:visible"><defs><linearGradient id="g${w}${h}" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="var(--acc)" stop-opacity=".28"/><stop offset="1" stop-color="var(--acc)" stop-opacity="0"/></linearGradient></defs>${fill ? `<path d="${d}L${w} ${h}L0 ${h}Z" fill="url(#g${w}${h})"/>` : ''}<path d="${d}" fill="none" stroke="var(--acc)" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round"/></svg>`;
};
// SQL 简易高亮
window.hl = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;')
  .replace(/(--[^\n]*)/g, '<span class="c">$1</span>')
  .replace(/\b(SELECT|FROM|JOIN|ON|WHERE|GROUP|BY|ORDER|AND|OR|AS|INTERVAL|DAY|DESC|ASC|LIMIT|UPDATE|SET|IN|INSERT|INTO|VALUES|SHOW|CREATE|TABLE|COUNT|SUM|AVG|ROUND|NOW|DATE)\b/g, m => /^(COUNT|SUM|AVG|ROUND|NOW|DATE)$/.test(m) ? `<span class="f">${m}</span>` : `<span class="k">${m}</span>`)
  .replace(/('[^']*')/g, '<span class="s">$1</span>').replace(/(?<![\w#">])(\d+)(?![\w"])/g, '<span class="nu">$1</span>');
/* tile() 见 dbicons.js：数据库品牌图标块 */
window.tagOf = (e) => `<span class="tag ${({ PROD: 'prod', STG: 'stg', DEV: 'dev', TEST: 'test', LOCAL: 'local' })[e]}">${e}</span>`;
