package model

import (
	"fmt"
	"log"
	"strings"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	_ "github.com/pingcap/parser/test_driver"
	"github.com/rqlite/sql"
)

func TestGetColumnComment(t *testing.T) {
	text := `
	--id:'id|text'
	--name:'姓名|text|validate:"gt 1"'
	`
	lt := GetColumnAnnotations(text)
	fmt.Println(lt)
}

func TestGoModFilePath(t *testing.T) {
	got := GoModFilePath()
	fmt.Println(got)
}

const user_table_sqlite = `
CREATE TABLE "user" (
	"id" integer NOT NULL,
	"name" text NOT NULL,
	"age" integer NOT NULL,
	"ctime" integer NOT NULL,
	"mtime" integer NOT NULL,
	PRIMARY KEY ("id")
  );
`

const user_table_pg = `
CREATE TABLE "public"."user" (
    "id" serial NOT NULL PRIMARY KEY,
    "name" varchar(255) NOT NULL,
    "age" int4 NOT NULL,
    "ctime" timestamp(6) NOT NULL DEFAULT now(),
    "mtime" timestamp(6) NOT NULL DEFAULT now()
)
`

const user_table_mysql = `
CREATE TABLE user (
    id int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id字段',
    name varchar(100) NOT NULL COMMENT '名称',
    age int(11) NOT NULL DEFAULT '0' COMMENT '年龄',
    ctime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    mtime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY ix_name (name) USING BTREE,
    KEY ix_mtime (mtime) USING BTREE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4
`

func TestSqliteParse(t *testing.T) {
	p := sql.NewParser(strings.NewReader(user_table_sqlite))
	st, err := p.ParseStatement()
	if err != nil {
		panic(err)
	}
	ct := st.(*sql.CreateTableStatement)
	fmt.Println(ct.Name.Name, ct.String(), ct.Columns)
	fmt.Println(ct.Constraints)
	for _, v := range ct.Columns {
		fmt.Println(v.Constraints)
	}
}

func TestPgParse(t *testing.T) {
	x, err := pg_query.Parse(user_table_pg)
	if err != nil {
		panic(err)
	}
	fmt.Println(x.GetStmts()[0].GetStmt())
}

func TestMysqlParse(t *testing.T) {
	p, _, err := parser.New().Parse(user_table_mysql, "", "")
	if err != nil {
		log.Fatal(err)
	}
	tableStmt, ok := p[0].(*ast.CreateTableStmt)
	fmt.Println(tableStmt, ok)
}
