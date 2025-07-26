package main

import (
	"bytes"
	_ "embed"
	"flag"
	"go/format"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goflower-io/crud/internal/model"
	"github.com/goflower-io/crud/internal/tag"
)

//go:embed "internal/templates/proto.tmpl"
var protoTmpl []byte

//go:embed "internal/templates/service.tmpl"
var serviceTmpl []byte

//go:embed "internal/templates/client.tmpl"
var clientGenericTmpl []byte

//go:embed "internal/templates/sql_crud.tmpl"
var genericTmpl []byte

//go:embed "internal/templates/http.tmpl"
var httpTmpl []byte

//go:embed "internal/templates/view.tmpl"
var viewTmpl []byte

var (
	database string
	path     string
	service  bool
	http     bool
	protopkg string
	dialect  string
)

// var fields string
const defaultDir = "crud"

func init() {
	flag.BoolVar(
		&service,
		"service",
		false,
		"-service  generate GRPC proto message and service implementation",
	)
	flag.BoolVar(&http, "http", false, "-http  generate http handler and templ view")
	flag.StringVar(&protopkg, "protopkg", "", "-protopkg  proto package field value")
	flag.StringVar(
		&dialect,
		"dialect",
		"mysql",
		"-dialect only support mysql postgres sqlite3, default mysql ",
	)
}

func main() {
	flag.Parse()

	// subcommand
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "init":
			// create crud dir
			if err := os.Mkdir(defaultDir, os.ModePerm); err != nil {
				log.Fatal(err)
			}
			return
		}
	}
	if len(os.Args) == 1 {
		info, err := os.Stat(defaultDir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Fatal("crud dir is not exist please exec: crud init")
				return
			}
			log.Fatal(err)
			return
		}
		if info.IsDir() {
			path = defaultDir
		}
	}

	if path == "" {
		path = defaultDir
	}
	tableObjs, isDir := tableFromSql(path)
	for _, v := range tableObjs {
		generateFiles(v)
	}
	if isDir && path == defaultDir {
		generateFile(
			filepath.Join(defaultDir, "aa_client.go"),
			string(clientGenericTmpl),
			f,
			tableObjs,
		)
	}
}

func tableFromSql(path string) (tableObjs []*model.Table, isDir bool) {
	relativePath := model.GetRelativePath()
	info, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}
	if info.IsDir() {
		isDir = true
		fs, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, v := range fs {
			if !v.IsDir() && strings.HasSuffix(strings.ToLower(v.Name()), ".sql") {
				switch dialect {
				case "mysql":
					obj := model.MysqlTable(
						database,
						filepath.Join(path, v.Name()),
						relativePath,
						dialect,
					)
					if obj != nil {
						tableObjs = append(tableObjs, obj)
					}
				case "postgres":
					obj := model.PostgresTable(
						database,
						filepath.Join(path, v.Name()),
						relativePath,
						dialect,
					)
					if obj != nil {
						tableObjs = append(tableObjs, obj)
					}
				case "sqlite3":
					obj := model.Sqlite3Table(
						database,
						filepath.Join(path, v.Name()),
						relativePath,
						dialect,
					)
					if obj != nil {
						tableObjs = append(tableObjs, obj)
					}

				}
			}
		}
	} else {
		switch dialect {
		case "mysql":
			obj := model.MysqlTable(database, path, relativePath, dialect)
			if obj != nil {
				tableObjs = append(tableObjs, obj)
			}
		case "postgres":
			obj := model.PostgresTable(database, path, relativePath, dialect)
			if obj != nil {
				tableObjs = append(tableObjs, obj)
			}
		case "sqlite3":
			obj := model.Sqlite3Table(database, path, relativePath, dialect)
			if obj != nil {
				tableObjs = append(tableObjs, obj)
			}

		}
	}
	return tableObjs, isDir
}

var f = template.FuncMap{
	"sqltool":                        model.SQLTool,
	"isnumber":                       model.IsNumber,
	"Incr":                           model.Incr,
	"GoTypeToTypeScriptDefaultValue": model.GoTypeToTypeScriptDefaultValue,
	"GoTypeToWhereFunc":              model.GoTypeToWhereFunc,
}

func generateFiles(tableObj *model.Table) {
	// 创建目录
	dir := filepath.Join(defaultDir, tableObj.PackageName)
	os.Mkdir(dir, os.ModePerm)
	generateFile(filepath.Join(dir, tableObj.PackageName+".go"), string(genericTmpl), f, tableObj)
	if service {
		generateService(tableObj)
	}
}

func generateService(tableObj *model.Table) {
	pkgName := tableObj.PackageName
	tableObj.Protopkg = protopkg
	os.Mkdir(filepath.Join("proto"), os.ModePerm)
	os.Mkdir(filepath.Join("service"), os.ModePerm)

	generateFile(filepath.Join("proto", pkgName+".api.proto"), string(protoTmpl), f, tableObj)

	// proto-go  grpc
	var cmd *exec.Cmd
	cmd = exec.Command(
		"protoc",
		"-I.",
		"--go_out=.",
		"--go-grpc_out=.",
		filepath.Join("proto", pkgName+".api.proto"),
	)

	cmd.Dir = filepath.Join(model.GetCurrentPath())
	log.Println(cmd.Dir, "exec:", cmd.String())
	s, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(s), err)
	}
	pbfile := filepath.Join("api", pkgName+".api.pb.go")
	ara, err := tag.ParseFile(pbfile, nil, nil)
	if err != nil {
		log.Printf("err:%v", err)
	} else {
		tag.WriteFile(pbfile, ara, false)
	}

	generateFile(filepath.Join("service", pkgName+".service.go"), string(serviceTmpl), f, tableObj)
	if http {
		generateFile(filepath.Join("service", pkgName+".http.go"), string(httpTmpl), f, tableObj)
		os.Mkdir(filepath.Join("views"), os.ModePerm)
		generateFile(filepath.Join("views", pkgName+".templ"), string(viewTmpl), f, tableObj)
	}
}

func generateFile(filename, tmpl string, f template.FuncMap, data interface{}) {
	tpl, err := template.New(filename).Funcs(f).Parse(string(tmpl))
	if err != nil {
		log.Fatalln(err)
	}
	bs := bytes.NewBuffer(nil)
	err = tpl.Execute(bs, data)
	if err != nil {
		log.Fatalln(err)
	}

	result := bs.Bytes()
	if strings.HasSuffix(filename, ".go") {
		result, err = format.Source(bs.Bytes())
		if err != nil {
			log.Fatal(err)
		}
	}
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalln(err)
	}
	file.Write(result)
	file.Close()
}
