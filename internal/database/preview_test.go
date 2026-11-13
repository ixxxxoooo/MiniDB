package database

import (
	"reflect"
	"testing"
)

func TestPreviewChanges(t *testing.T) {
	got := PreviewChanges("mysql", "shop", "users",
		[]map[string]interface{}{{"id": 9}},
		[]map[string]interface{}{{"name": "O'Brien", "age": int64(3), "bio": nil}},
		[]RowChange{{PrimaryKey: map[string]interface{}{"id": 1, "tenant": "a"}, Changes: map[string]interface{}{"name": "x", "email": nil}}},
	)
	want := []string{
		"DELETE FROM `shop`.`users` WHERE `id` = 9;",
		"INSERT INTO `shop`.`users` (`age`, `bio`, `name`) VALUES (3, NULL, 'O''Brien');",
		"UPDATE `shop`.`users` SET `email` = NULL, `name` = 'x' WHERE `id` = 1 AND `tenant` = 'a';",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	pg := PreviewChanges("postgres", "app", "t", nil, nil, []RowChange{{PrimaryKey: map[string]interface{}{"id": nil}, Changes: map[string]interface{}{"a": true}}})
	if pg[0] != `UPDATE "t" SET "a" = 1 WHERE "id" IS NULL;` {
		t.Fatalf("postgres: %q", pg)
	}
}
