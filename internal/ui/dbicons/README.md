# 数据库 logo

用来在连接列表、类型下拉、命令面板里标识数据库类型（nominative use，仅用于指明所连接的数据库）。
这些 logo 是各自所有者的商标。文件都已改成单色、跟随文字颜色（`currentColor`）。

| 文件 | 来源 | 许可 |
|---|---|---|
| `mysql.svg` | [devicon](https://github.com/devicons/devicon) `mysql-original`（海豚） | MIT |
| `postgresql.svg` | [Simple Icons](https://simpleicons.org) `postgresql` | CC0 |
| `sqlite.svg` | [Simple Icons](https://simpleicons.org) `sqlite` | CC0 |
| `tidb.svg` | [Simple Icons](https://simpleicons.org) `tidb`（经 Iconify 取得） | CC0 |
| `mariadb.svg` | 自绘的字母 M 标记（不是 MariaDB 官方 logo） | 本项目 |
| `starrocks.svg` | StarRocks 官方文档站 `docs.starrocks.io/img/logo.svg`；黄色面改为白，青色三面改为 70% 白 | 商标归 StarRocks 所有 |

新增一种数据库类型时：放一个单色 SVG 到这里，并在 `../dbicons.go` 的 `dbIconFile` 里登记。
