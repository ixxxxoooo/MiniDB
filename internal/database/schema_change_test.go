package database

import "testing"

func TestIsSchemaChangeSQL(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{name: "create table", sql: "CREATE TABLE users(id int)", want: true},
		{name: "alter table", sql: "ALTER TABLE users ADD COLUMN name varchar(255)", want: true},
		{name: "drop index", sql: "DROP INDEX idx_name ON users", want: true},
		{name: "rename table", sql: "RENAME TABLE users TO app_users", want: true},
		{name: "select", sql: "SELECT * FROM users", want: false},
		{name: "insert", sql: "INSERT INTO users(id) VALUES (1)", want: false},
		{name: "truncate", sql: "TRUNCATE TABLE users", want: false},
		{name: "commented create", sql: "-- note\nCREATE VIEW v_users AS SELECT * FROM users", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSchemaChangeSQL(tt.sql); got != tt.want {
				t.Fatalf("IsSchemaChangeSQL(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}
