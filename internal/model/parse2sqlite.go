package model

import (
	"log"
	"os"
	"strings"

	"github.com/rqlite/sql"
)

func Sqlite3Table(db, path, relative string, dialect string) *Table {
	sqlstr, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	annotations := GetColumnAnnotations(string(sqlstr))
	p := sql.NewParser(strings.NewReader(string(sqlstr)))
	st, err := p.ParseStatement()
	if err != nil {
		log.Fatalln(err)
	}
	ct, ok := st.(*sql.CreateTableStatement)
	if !ok {
		log.Fatalln("not a CreateTableStatement")
	}
	tableName := ct.Name.Name
	gotableName := GoCamelCase(ct.Name.Name)
	mytable := &Table{
		Database:    db,
		TableName:   tableName,
		GoTableName: gotableName,
		PackageName: strings.ToLower(gotableName),
		Dialect:     dialect,
	}
	columns, err := Sqlite3Column(ct, annotations)
	if err != nil {
		log.Fatal(err)
	}
	if len(columns) <= 0 {
		log.Fatal("schema or table not exist")
	}
	mytable.Fields = columns
	for _, v := range columns {
		if v.IsPrimaryKey {
			mytable.PrimaryKey = v
		}
		if v.GoColumnType == "time.Time" {
			mytable.ImportTime = true
		}
		v.ProtoType = GoTypeToProtoType(v.GoColumnType)
	}

	mytable.GenerateWhereCol = mytable.Fields
	mytable.RelativePath = relative
	return mytable
}

func Sqlite3Column(ddl *sql.CreateTableStatement, annotations map[string]*ColumnAnnotation) ([]*Column, error) {
	res := []*Column{}
	for k, v := range ddl.Columns {

		var notNull bool
		var autoIncrement bool
		var primaryKey bool
		var comment string
		for _, v2 := range v.Constraints {
			if _, ok := v2.(*sql.PrimaryKeyConstraint); ok {
				primaryKey = true
			}
			if _, ok := v2.(*sql.NotNullConstraint); ok {
				notNull = true
			}

		}
		columnType := v.Type.String()
		if primaryKey && strings.Contains(columnType, "integer") {
			autoIncrement = true
		}
		c := &Column{
			OrdinalPosition:           k,
			ColumnName:                v.Name.Name,
			DataType:                  columnType,
			ColumnType:                "",
			ColumnComment:             comment,
			NotNull:                   notNull,
			IsPrimaryKey:              primaryKey,
			IsAutoIncrment:            autoIncrement,
			IsDefaultCurrentTimestamp: false,
			GoColumnName:              "",
			GoColumnType:              "",
			BigType:                   0,
			GoConditionType:           "",
			HTMLInputType:             "text",
		}

		if anno, ok := annotations[c.ColumnName]; ok {
			c.GoTags = anno.GoTags
			if anno.HTMLInputType != "" {
				c.HTMLInputType = anno.HTMLInputType
			}
			c.EnumValues = anno.SelectEnum
		}

		c.GoColumnName = GoCamelCase(c.ColumnName)
		c.GoColumnType, c.BigType = Sqlite3ToGoFieldType(c.DataType)
		if strings.Contains(c.GoColumnType, "int") {
			c.GoColumnType = "int64"
		}
		c.GoConditionType = c.GoColumnType
		if c.BigType == bigtypeCompareTime {
			c.GoConditionType = "string"
		}
		res = append(res, c)

	}
	var primaryKey string
	for _, v := range ddl.Constraints {
		if x, ok := v.(*sql.PrimaryKeyConstraint); ok {
			if len(x.Columns) > 1 {
				log.Fatal("primary key is not single column")
			}
			primaryKey = x.Columns[0].Name
		}
	}
	for _, v := range res {
		if v.ColumnName == primaryKey {
			v.IsPrimaryKey = true
			if strings.Contains(v.DataType, "integer") {
				v.IsAutoIncrment = true
			}
		}
		v.ProtoType = GoTypeToProtoType(v.GoColumnType)
	}
	return res, nil
}

func Sqlite3ToGoFieldType(dt string) (string, int) {
	var typ string
	var gtp int
	switch strings.ToLower(dt) {
	case "text":
		typ = "string"
		gtp = bigtypeCompareString
	case "integer":
		typ = "int64"
		gtp = bigtypeCompare
	case "real":
		typ = "float32"
		gtp = bigtypeCompare
	case "blob":
		typ = "[]byte"
	default:
		typ = "UNKNOWN"
	}
	return typ, gtp
}
