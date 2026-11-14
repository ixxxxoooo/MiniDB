package ui

import (
	"embed"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 各数据库的 logo。放在 dbicons/ 下，来源与许可见 dbicons/README.md。
// 都是"跟随文字颜色"的单色 SVG：dbTile 把它们画成白色，放在彩色圆角块上，
// 连接的颜色（环境色）照样靠色块体现。

//go:embed dbicons/*.svg
var dbIconFiles embed.FS

// dbIconFile 是各数据库类型对应的文件名。
var dbIconFile = map[model.DBType]string{
	model.MySQL:     "mysql.svg",
	model.MariaDB:   "mariadb.svg",
	model.Postgres:  "postgresql.svg",
	model.SQLServer: "sqlserver.svg",
	model.SQLite:    "sqlite.svg",
	model.TiDB:      "tidb.svg",
	model.StarRocks: "starrocks.svg",
}

// dbGlyphs 是解析好的 logo，按数据库类型索引。
var dbGlyphs = func() map[model.DBType]*ui.SVG {
	out := make(map[model.DBType]*ui.SVG, len(dbIconFile))
	for kind, name := range dbIconFile {
		data, err := dbIconFiles.ReadFile("dbicons/" + name)
		if err != nil {
			panic("dbicons/" + name + ": " + err.Error())
		}
		out[kind] = ui.MustParseSVG(data)
	}
	return out
}()
