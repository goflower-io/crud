# crud — Code Generation ORM for Go

**Design your table, get your code.** crud generates type-safe, high-performance CRUD code from SQL DDL for MySQL, MariaDB, PostgreSQL, SQLite3, and MongoDB.

> Inspired by [facebook/ent](https://github.com/ent/ent)

[中文文档](README_zh.md) | [Examples](https://github.com/goflower-io/example) | [xsql](https://github.com/goflower-io/xsql) | [golib](https://github.com/goflower-io/golib)

---

## Why crud?

| Feature | Description |
|---|---|
| Table-first workflow | Write DDL → run `crud` → get production-ready Go code instantly |
| Zero reflection on hot paths | Querying all fields uses no reflection; performance matches hand-written SQL |
| IDE-friendly API | Full autocomplete, no magic strings in query conditions |
| Batteries included | Transactions, row-level locking (`FOR UPDATE`, `LOCK IN SHARE MODE`), upsert, batch insert |
| gRPC ready | One flag generates `.proto` files + service skeleton following Google API Design Guide |
| Read-write separation | Built-in master/slave routing with round-robin load balancing via [xsql](https://github.com/goflower-io/xsql) |

---

## Ecosystem

```
┌─────────────────────────────────────────────────────┐
│                      example                         │
│   (MySQL / PostgreSQL / SQLite3 full-stack demo)     │
└──────────┬──────────────────────────────────────────┘
           │ uses
    ┌──────▼──────┐   generates   ┌──────────┐
    │    crud     │──────────────▶│  your    │
    │  (codegen)  │               │  models  │
    └─────────────┘               └────┬─────┘
                                       │ runtime
    ┌─────────────┐   DB client  ┌─────▼──────┐
    │    xsql     │◀─────────────│  generated  │
    │ (SQL build) │              │    code     │
    └─────────────┘              └─────┬───────┘
                                       │ served by
    ┌─────────────┐                    │
    │    golib    │◀───────────────────┘
    │(HTTP/gRPC)  │
    └─────────────┘
```

---

## Quick Start

### 1. Install

```bash
go install github.com/goflower-io/crud@main
```

### 2. Write your SQL DDL

```sql
-- crud/sql/user.sql
CREATE TABLE `user` (
  `id`    int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `name`  varchar(100)     NOT NULL COMMENT '名称|text|validate:"max=100,min=10"',
  `age`   int(11)          NOT NULL DEFAULT '0'  COMMENT '年龄|number|validate:"max=140,min=18"',
  `sex`   int(11)          NOT NULL DEFAULT '2'  COMMENT '性别|select|validate:"oneof=0 1 2"|0:女 1:男 2:无',
  `ctime` datetime         NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `mtime` datetime         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `ix_name`  (`name`)  USING BTREE,
  KEY `ix_mtime` (`mtime`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 3. Generate code

```bash
# Initialize crud directory in your project root
crud init

# Place user.sql in crud/sql/, then generate CRUD model
crud

# Also generate gRPC proto + service skeleton
crud -service -protopkg mypkg
```

Generated layout:

```
myproject/
├── crud/
│   ├── aa_client.go          # DB client with read-write separation
│   ├── sql/user.sql
│   └── user/
│       └── user.go           # Generated model + all CRUD operations
├── proto/
│   └── user.api.proto        # gRPC service definition
├── api/
│   ├── user.api.pb.go
│   └── user.api_grpc.pb.go
└── service/
    └── user.service.go       # gRPC service skeleton (fill in validation)
```

---

## Generated Code Overview

`crud/user/user.go` contains the model, column constants, and typed field operators:

```go
type User struct {
    Id    int64     `json:"id"`
    Name  string    `json:"name"`
    Age   int64     `json:"age"`
    Sex   int64     `json:"sex"`
    Ctime time.Time `json:"ctime"`
    Mtime time.Time `json:"mtime"`
}

// Column name constants
const (
    Id    = "id"
    Name  = "name"
    Age   = "age"
    Sex   = "sex"
    Ctime = "ctime"
    Mtime = "mtime"
)

// Typed field operators — used to build WHERE conditions
const (
    IdOp    = xsql.FieldOp[int64]("id")
    NameOp  = xsql.StrFieldOp("name")
    AgeOp   = xsql.FieldOp[int64]("age")
    SexOp   = xsql.FieldOp[int64]("sex")
    CtimeOp = xsql.FieldOp[string]("ctime")
    MtimeOp = xsql.FieldOp[string]("mtime")
)
```

`crud/aa_client.go` wraps the DB with per-table sub-clients and read-write routing:

```go
// Read operations route to replicas; write operations go to master
client.User.Find()    // → slave
client.User.Create()  // → master
client.User.Update()  // → master
client.User.Delete()  // → master

client.Master.User.Find() // force master read (e.g. read-after-write)
```

---

## Initialize DB Client

```go
import (
    "github.com/goflower-io/example/crud"
    "github.com/goflower-io/xsql"
)

client, _ := crud.NewClient(&xsql.Config{
    DSN:          "root:123456@tcp(127.0.0.1:3306)/test?parseTime=true&loc=Local",
    ReadDSN:      []string{"root:123456@tcp(127.0.0.1:3306)/test?parseTime=true&loc=Local"},
    Active:       20,
    Idle:         20,
    IdleTimeout:  time.Hour * 24,
    QueryTimeout: time.Second * 10,
    ExecTimeout:  time.Second * 10,
}, true) // true = enable debug logging
```

---

## CRUD API

### Create

```go
// Single insert — a.Id is populated with the auto-increment ID after save
a := &user.User{
    Id:    0,
    Name:  "alice",
    Age:   18,
    Sex:   1,
    Ctime: time.Now(),
    Mtime: time.Now(),
}
_, err := client.User.Create().SetUser(a).Save(ctx)
fmt.Println(a.Id) // set by DB

// Batch insert
_, err = client.User.Create().SetUser(a, b, c).Save(ctx)

// Upsert (INSERT … ON DUPLICATE KEY UPDATE)
_, err = client.User.Create().SetUser(a).Upsert(ctx)
```

Or use the package-level functions directly (e.g. with a raw `*xsql.DB`):

```go
_, err := user.Create(db).SetUser(a).Save(ctx)
_, err  = user.Create(db).SetUser(a, b).Upsert(ctx)
```

### Query

```go
// Single record — automatically adds LIMIT 1
u, err := client.User.Find().Where(user.IdOp.EQ(1)).One(ctx)

// Force read from master (read-after-write consistency)
u, err = client.Master.User.Find().Where(user.IdOp.EQ(a.Id)).One(ctx)

// Multiple records
list, err := client.User.Find().Where(user.AgeOp.In(18, 20, 30)).All(ctx)

// Complex conditions + ordering + pagination
list, err = client.User.Find().
    Where(user.Or(
        user.IdOp.GT(100),
        user.AgeOp.In(18, 25),
    )).
    OrderDesc(user.Mtime).
    Offset(0).Limit(20).
    All(ctx)

// String operations
list, err = client.User.Find().Where(user.NameOp.Contains("ali")).All(ctx)
list, err = client.User.Find().Where(user.NameOp.HasPrefix("ali")).All(ctx)

// Select specific columns
list, err = client.User.Find().
    Select(user.Id, user.Name, user.Age).
    Where(user.AgeOp.GT(18)).
    All(ctx)

// Count
count, err := client.User.Find().Count().Where(user.IdOp.GT(0)).Int64(ctx)

// Single column list
names, err := client.User.Find().
    Select(user.Name).
    Where(user.IdOp.In(1, 2, 3)).
    Strings(ctx)

// GROUP BY / HAVING / custom result struct
type Stat struct {
    Name string `json:"name"`
    Cnt  int64  `json:"cnt"`
}
var result []*Stat
client.User.Find().
    Select(user.Name, xsql.As(xsql.Count("*"), "cnt")).
    ForceIndex("ix_name").
    GroupBy(user.Name).
    Having(xsql.GT("cnt", 1)).
    Slice(ctx, &result)
// SELECT `name`, COUNT(*) AS `cnt` FROM `user` FORCE INDEX (`ix_name`) GROUP BY `name` HAVING `cnt` > ?
```

### Update

```go
// Set fields
_, err := client.User.Update().
    SetName("bob").SetAge(25).SetSex(0).
    Where(user.IdOp.EQ(1)).
    Save(ctx)

// Increment / decrement
_, err = client.User.Update().
    AddAge(-1).
    Where(user.IdOp.EQ(1)).
    Save(ctx)
// UPDATE `user` SET `age` = COALESCE(`age`,0) + -1 WHERE `id` = 1
```

### Delete

```go
_, err := client.User.Delete().
    Where(user.IdOp.EQ(1)).
    Exec(ctx)
```

### Transactions

```go
tx, err := client.Begin(ctx)
if err != nil {
    return err
}
_, err = tx.User.Create().SetUser(u1).Save(ctx)
if err != nil {
    return tx.Rollback()
}
_, err = tx.User.Update().SetAge(100).Where(user.IdOp.EQ(u1.Id)).Save(ctx)
if err != nil {
    return tx.Rollback()
}
return tx.Commit()
```

### Debug logging

```go
// Option 1: enable debug on the whole client at init time
client, _ := crud.NewClient(config, true)

// Option 2: wrap a single operation
user.Create(xsql.Debug(db)).SetUser(u).Save(ctx)
// [xsql] INSERT INTO `user` (`name`, `age`, ...) VALUES (?, ?, ...) [alice 18 ...]
```

---

## gRPC Code Generation

### Prerequisites

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# Ensure /usr/local/include contains google/protobuf/empty.proto
```

### Generate

```bash
crud -service -protopkg mypkg
```

### Generated proto (`user.api.proto`)

```proto
syntax = "proto3";
option go_package = "/api";
import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

service UserService {
    rpc CreateUser(User)              returns (User);
    rpc DeleteUser(UserId)            returns (google.protobuf.Empty);
    rpc UpdateUser(UpdateUserReq)     returns (User);
    rpc GetUser(UserId)               returns (User);
    rpc ListUsers(ListUsersReq)       returns (ListUsersResp);
    rpc ListUsersMore(ListUsersMoreReq) returns (ListUsersMoreResp);
}

message UpdateUserReq {
    User            user  = 1;
    repeated UserField masks = 2;  // enum field mask
}

message ListUsersReq {
    int32                  page          = 1;
    int32                  page_size     = 2;
    repeated UserOrderBy   orderbys      = 3;
    repeated UserFilter    filters       = 4;
    repeated UserField     select_fields = 5;
}
```

### Generated service (`user.service.go`)

The skeleton implements all five gRPC methods. Key patterns:

```go
// CreateUser — insert then read-after-write from master
func (s *UserServiceImpl) CreateUser(ctx context.Context, req *api.User) (*api.User, error) {
    a := &user.User{
        Name:  req.GetName(),
        Age:   req.GetAge(),
        Sex:   req.GetSex(),
        Ctime: time.Now(),
        Mtime: time.Now(),
    }
    _, err := s.Client.User.Create().SetUser(a).Save(ctx)
    if err != nil {
        return nil, status.Error(codes.Internal, err.Error())
    }
    // read-after-write: force master
    a2, err := s.Client.Master.User.Find().Where(user.IdOp.EQ(a.Id)).One(ctx)
    if err != nil {
        return nil, status.Error(codes.Internal, err.Error())
    }
    return convertUser(a2), nil
}

// UpdateUser — enum field mask controls which fields are updated
func (s *UserServiceImpl) UpdateUser(ctx context.Context, req *api.UpdateUserReq) (*api.User, error) {
    if len(req.GetMasks()) == 0 {
        return nil, status.Error(codes.InvalidArgument, "empty filter condition")
    }
    update := s.Client.User.Update()
    for _, v := range req.GetMasks() {
        switch v {
        case api.UserField_User_name:
            update.SetName(req.GetUser().GetName())
        case api.UserField_User_age:
            update.SetAge(req.GetUser().GetAge())
        case api.UserField_User_sex:
            update.SetSex(req.GetUser().GetSex())
        }
    }
    _, err := update.Where(user.IdOp.EQ(req.GetUser().GetId())).Save(ctx)
    // ...
}

// ListUsers — dynamic filters + multi-column ordering + select fields
func (s *UserServiceImpl) ListUsers(ctx context.Context, req *api.ListUsersReq) (*api.ListUsersResp, error) {
    finder := s.Client.User.Find().
        Select(selectFields...).
        Offset(offset).Limit(size)

    for _, v := range req.GetOrderbys() {
        col := strings.TrimPrefix(v.GetField().String(), "User_")
        if v.GetDesc() {
            finder.OrderDesc(col)
        } else {
            finder.OrderAsc(col)
        }
    }
    for _, v := range req.GetFilters() {
        p, _ := xsql.GenP(strings.TrimPrefix(v.Field.String(), "User_"), v.Op, v.Val)
        ps = append(ps, p)
    }
    if len(ps) > 0 {
        finder.WhereP(xsql.And(ps...))
    }
    // ...
}
```

### Wire up with golib and test with grpcurl

```go
import "github.com/goflower-io/golib/net/app"

a := app.New(app.WithAddr("0.0.0.0", 8080))
a.RegisteGrpcService(&api.UserService_ServiceDesc, &service.UserServiceImpl{Client: client})
a.Run()
```

```bash
# Install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# List services (gRPC reflection is auto-registered by golib)
grpcurl -plaintext localhost:8080 list

# Describe UserService
grpcurl -plaintext localhost:8080 describe UserService

# Create a user
grpcurl -plaintext \
  -d '{"name":"alice","age":18,"sex":1}' \
  localhost:8080 UserService/CreateUser

# Get a user
grpcurl -plaintext \
  -d '{"id":1}' \
  localhost:8080 UserService/GetUser

# Update specific fields (masks use UserField enum: 2=name, 3=age, 4=sex)
grpcurl -plaintext \
  -d '{"user":{"id":1,"name":"bob","age":25,"sex":0},"masks":[2,3]}' \
  localhost:8080 UserService/UpdateUser

# List with pagination
grpcurl -plaintext \
  -d '{"page":1,"page_size":10}' \
  localhost:8080 UserService/ListUsers

# List with filter (age > 18), order by mtime desc, select id+name+age only
grpcurl -plaintext \
  -d '{
    "page": 1,
    "page_size": 10,
    "filters":  [{"field":3,"op":"GT","val":"18"}],
    "orderbys": [{"field":6,"desc":true}],
    "select_fields": [1,2,3]
  }' \
  localhost:8080 UserService/ListUsers

# Cursor-based pagination (no total count, better for large datasets)
grpcurl -plaintext \
  -d '{"page_size":5,"cursor":{"orderbys":[{"field":1,"desc":false}]}}' \
  localhost:8080 UserService/ListUsersMore

# Delete a user
grpcurl -plaintext \
  -d '{"id":1}' \
  localhost:8080 UserService/DeleteUser
```

**UserField enum values:**

| Value | Field |
|---|---|
| 1 | id |
| 2 | name |
| 3 | age |
| 4 | sex |
| 5 | ctime |
| 6 | mtime |

---

## CLI Reference

```
Usage of crud:
  -dialect string   database dialect: mysql | postgres | sqlite3  (default "mysql")
  -protopkg string  Go package name for the generated proto go_package option
  -service          also generate gRPC proto + service skeleton
```

---

## Best Practices

1. Use `NOT NULL DEFAULT 0` for numeric columns and `NOT NULL DEFAULT ''` for strings — avoids NULL edge cases in generated code.
2. Keep `.sql` files in version control alongside generated code — table schema changes become reviewable diffs.
3. Use `client.Master.User.Find()` for read-after-write scenarios (e.g. return the created record).
4. Use [golib](https://github.com/goflower-io/golib) `app.New()` to serve gRPC and HTTP on the same port with built-in recovery, structured logging, and Prometheus metrics.
