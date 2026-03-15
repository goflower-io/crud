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

//go:embed "internal/templates/http.tmpl"
var httpTmpl []byte

//go:embed "internal/templates/view.tmpl"
var viewTmpl []byte

//go:embed "internal/templates/client.tmpl"
var clientGenericTmpl []byte

//go:embed "internal/templates/sql_crud.tmpl"
var genericTmpl []byte

var (
	database    string
	path        string
	service     bool
	httpHandler bool
	protopkg    string
	dialect     string
)

const defaultDir = "crud"

func init() {
	flag.StringVar(&database, "database", "", "-database  target database name")
	flag.StringVar(&path, "path", "", "-path  path to SQL file or directory (default: ./crud)")
	flag.BoolVar(
		&service,
		"service",
		false,
		"-service  generate gRPC proto, service implementation, HTTP handler, and templ views",
	)
	flag.BoolVar(
		&httpHandler,
		"http",
		false,
		"-http  generate HTTP handler (service/[name].http.go) and templ views (views/[name].templ)",
	)
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

	// positional subcommand: crud init
	if len(os.Args) == 2 && os.Args[1] == "init" {
		if err := os.Mkdir(defaultDir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		return
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

func tableFromSqlFile(filePath, db, relative, d string) *model.Table {
	switch d {
	case "mysql":
		return model.MysqlTable(db, filePath, relative, d)
	case "postgres":
		return model.PostgresTable(db, filePath, relative, d)
	case "sqlite3":
		return model.Sqlite3Table(db, filePath, relative, d)
	}
	return nil
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
				if obj := tableFromSqlFile(filepath.Join(path, v.Name()), database, relativePath, dialect); obj != nil {
					tableObjs = append(tableObjs, obj)
				}
			}
		}
	} else {
		if obj := tableFromSqlFile(path, database, relativePath, dialect); obj != nil {
			tableObjs = append(tableObjs, obj)
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
	dir := filepath.Join(defaultDir, tableObj.PackageName)
	os.Mkdir(dir, os.ModePerm)
	generateFile(filepath.Join(dir, tableObj.PackageName+".go"), string(genericTmpl), f, tableObj)
	if service {
		generateService(tableObj)
	}
	if httpHandler {
		generateHTTP(tableObj)
		generateView(tableObj)
	}
}

func generateService(tableObj *model.Table) {
	pkgName := tableObj.PackageName
	tableObj.Protopkg = protopkg
	os.Mkdir(filepath.Join("proto"), os.ModePerm)
	os.Mkdir(filepath.Join("service"), os.ModePerm)

	generateFile(filepath.Join("proto", pkgName+".api.proto"), string(protoTmpl), f, tableObj)

	// compile proto → Go + gRPC stubs
	cmd := exec.Command(
		"protoc",
		"-I.",
		"--go_out=.",
		"--go-grpc_out=.",
		filepath.Join("proto", pkgName+".api.proto"),
	)
	cmd.Dir = filepath.Join(model.GetCurrentPath())
	log.Println(cmd.Dir, "exec:", cmd.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Println(string(out), err)
	}

	pbfile := filepath.Join("api", pkgName+".api.pb.go")
	if ara, err := tag.ParseFile(pbfile, nil, nil); err != nil {
		log.Printf("err:%v", err)
	} else {
		tag.WriteFile(pbfile, ara, false)
	}

	generateFile(filepath.Join("service", pkgName+".service.go"), string(serviceTmpl), f, tableObj)
	if httpHandler {
		generateHTTP(tableObj)
		generateView(tableObj)
	}
}

// generateHTTP writes service/[name].http.go from http.tmpl.
func generateHTTP(tableObj *model.Table) {
	os.Mkdir(filepath.Join("service"), os.ModePerm)
	pkgName := tableObj.PackageName
	generateFile(filepath.Join("service", pkgName+".http.go"), string(httpTmpl), f, tableObj)
}

// generateView writes views/[name].templ from view.tmpl.
// Run `templ generate` afterwards to produce the compiled *_templ.go file.
func generateView(tableObj *model.Table) {
	os.Mkdir(filepath.Join("views"), os.ModePerm)
	pkgName := tableObj.PackageName
	generateFile(filepath.Join("views", pkgName+".templ"), string(viewTmpl), f, tableObj)
}

func generateFile(filename, tmpl string, f template.FuncMap, data interface{}) {
	tpl, err := template.New(filename).Funcs(f).Parse(string(tmpl))
	if err != nil {
		log.Fatalln(err)
	}
	bs := bytes.NewBuffer(nil)
	if err = tpl.Execute(bs, data); err != nil {
		log.Fatalln(err)
	}

	result := bs.Bytes()
	// Only run go/format on .go files; .templ files have their own formatter.
	if strings.HasSuffix(filename, ".go") {
		if result, err = format.Source(bs.Bytes()); err != nil {
			log.Fatal(err)
		}
	}
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()
	if _, err = file.Write(result); err != nil {
		log.Fatalln(err)
	}
}
