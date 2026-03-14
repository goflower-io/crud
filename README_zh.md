# crud — Go 语言 ORM 代码生成工具

**设计好表结构，剩下的交给 crud。** 从 SQL DDL 自动生成类型安全、高性能的 CRUD 代码，支持 MySQL、MariaDB、PostgreSQL、SQLite3 和 MongoDB。

> 灵感来自 [facebook/ent](https://github.com/ent/ent)

[English](README.md) | [示例代码](https://github.com/goflower-io/example) | [xsql](https://github.com/goflower-io/xsql) | [golib](https://github.com/goflower-io/golib)

---

## 为什么选择 crud？

| 特性 | 说明 |
|---|---|
| 先建表后写代码 | 写好 DDL，执行 `crud`，立刻获得可投入生产的 Go 代码 |
| 热路径零反射 | 查询全部字段时不使用反射，性能与手写 SQL 相当 |
| IDE 友好的 API | 完整自动补全，查询条件无需硬编码字符串 |
| 开箱即用 | 事务、行级锁（`FOR UPDATE`、`LOCK IN SHARE MODE`）、Upsert、批量插入 |
| gRPC 就绪 | 一个参数即可生成遵循 Google API 设计规范的 `.proto` 文件和服务骨架 |
| 读写分离 | 通过 [xsql](https://github.com/goflower-io/xsql) 内置主从路由，读请求轮询负载均衡 |

---

## 生态系统

```
┌─────────────────────────────────────────────────────┐
│                      example                         │
│   （MySQL / PostgreSQL / SQLite3 全栈示例）            │
└──────────┬──────────────────────────────────────────┘
           │ 使用
    ┌──────▼──────┐   生成代码   ┌──────────┐
    │    crud     │────────────▶│  你的     │
    │  （代码生成） │             │  模型代码  │
    └─────────────┘             └────┬─────┘
                                     │ 运行时
    ┌─────────────┐  DB 客户端  ┌────▼──────┐
    │    xsql     │◀────────────│  生成的    │
    │（SQL 构建器）│             │   代码     │
    └─────────────┘             └────┬───────┘
                                     │ 由...提供服务
    ┌─────────────┐                  │
    │    golib    │◀─────────────────┘
    │（HTTP/gRPC）│
    └─────────────┘
```

---

## 快速开始

### 1. 安装

```bash
go install github.com/goflower-io/crud@main
```

### 2. 编写 SQL DDL

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

### 3. 生成代码

```bash
# 在项目根目录初始化 crud 目录
crud init

# 将 user.sql 放入 crud/sql/ 后生成 CRUD Model
crud

# 同时生成 gRPC proto 文件和服务骨架
crud -service -protopkg mypkg
```

生成结果：

```
myproject/
├── crud/
│   ├── aa_client.go          # 带读写分离的 DB 客户端
│   ├── sql/user.sql
│   └── user/
│       └── user.go           # 生成的 Model + 全部 CRUD 操作
├── proto/
│   └── user.api.proto        # gRPC 服务定义
├── api/
│   ├── user.api.pb.go
│   └── user.api_grpc.pb.go
└── service/
    └── user.service.go       # gRPC 服务骨架（补充参数校验即可）
```

---

## 生成代码速览

`crud/user/user.go` 包含 Model 结构体、字段常量和类型化字段操作符：

```go
type User struct {
    Id    int64     `json:"id"`
    Name  string    `json:"name"`
    Age   int64     `json:"age"`
    Sex   int64     `json:"sex"`
    Ctime time.Time `json:"ctime"`
    Mtime time.Time `json:"mtime"`
}

// 字段名常量
const (
    Id    = "id"
    Name  = "name"
    Age   = "age"
    Sex   = "sex"
    Ctime = "ctime"
    Mtime = "mtime"
)

// 类型化字段操作符 — 用于构建 WHERE 条件
const (
    IdOp    = xsql.FieldOp[int64]("id")
    NameOp  = xsql.StrFieldOp("name")
    AgeOp   = xsql.FieldOp[int64]("age")
    SexOp   = xsql.FieldOp[int64]("sex")
    CtimeOp = xsql.FieldOp[string]("ctime")
    MtimeOp = xsql.FieldOp[string]("mtime")
)
```

`crud/aa_client.go` 封装了按表划分的子客户端和读写路由：

```go
// 读操作路由到从库；写操作路由到主库
client.User.Find()    // → 从库
client.User.Create()  // → 主库
client.User.Update()  // → 主库
client.User.Delete()  // → 主库

client.Master.User.Find() // 强制读主库（如写后读场景）
```

---

## 初始化 DB 客户端

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
}, true) // true = 开启调试日志
```

---

## CRUD 接口

### Create（新增）

```go
// 单条插入 — 保存后 a.Id 自动被赋值为数据库自增 ID
a := &user.User{
    Id:    0,
    Name:  "alice",
    Age:   18,
    Sex:   1,
    Ctime: time.Now(),
    Mtime: time.Now(),
}
_, err := client.User.Create().SetUser(a).Save(ctx)
fmt.Println(a.Id) // 已被赋值

// 批量插入
_, err = client.User.Create().SetUser(a, b, c).Save(ctx)

// Upsert（INSERT … ON DUPLICATE KEY UPDATE）
_, err = client.User.Create().SetUser(a).Upsert(ctx)
```

也可直接使用包级函数（如配合原始 `*xsql.DB` 使用）：

```go
_, err := user.Create(db).SetUser(a).Save(ctx)
_, err  = user.Create(db).SetUser(a, b).Upsert(ctx)
```

### Query（查询）

```go
// 查询单条 — 自动添加 LIMIT 1
u, err := client.User.Find().Where(user.IdOp.EQ(1)).One(ctx)

// 写后读：强制从主库读取
u, err = client.Master.User.Find().Where(user.IdOp.EQ(a.Id)).One(ctx)

// 查询多条
list, err := client.User.Find().Where(user.AgeOp.In(18, 20, 30)).All(ctx)

// 复合条件 + 排序 + 分页
list, err = client.User.Find().
    Where(user.Or(
        user.IdOp.GT(100),
        user.AgeOp.In(18, 25),
    )).
    OrderDesc(user.Mtime).
    Offset(0).Limit(20).
    All(ctx)

// 字符串模糊查询
list, err = client.User.Find().Where(user.NameOp.Contains("ali")).All(ctx)
list, err = client.User.Find().Where(user.NameOp.HasPrefix("ali")).All(ctx)

// 指定查询列
list, err = client.User.Find().
    Select(user.Id, user.Name, user.Age).
    Where(user.AgeOp.GT(18)).
    All(ctx)

// 计数
count, err := client.User.Find().Count().Where(user.IdOp.GT(0)).Int64(ctx)

// 查询单列列表
names, err := client.User.Find().
    Select(user.Name).
    Where(user.IdOp.In(1, 2, 3)).
    Strings(ctx)

// GROUP BY / HAVING / 自定义结果结构体
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

### Update（更新）

```go
// 设置字段
_, err := client.User.Update().
    SetName("bob").SetAge(25).SetSex(0).
    Where(user.IdOp.EQ(1)).
    Save(ctx)

// 数值增减
_, err = client.User.Update().
    AddAge(-1).
    Where(user.IdOp.EQ(1)).
    Save(ctx)
// UPDATE `user` SET `age` = COALESCE(`age`,0) + -1 WHERE `id` = 1
```

### Delete（删除）

```go
_, err := client.User.Delete().
    Where(user.IdOp.EQ(1)).
    Exec(ctx)
// 线上账号若无删除权限，可改用 Update 实现软删除
```

### 事务

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

### 调试日志

```go
// 方式一：初始化时全局开启（第二个参数传 true）
client, _ := crud.NewClient(config, true)

// 方式二：单次操作包装
user.Create(xsql.Debug(db)).SetUser(u).Save(ctx)
// [xsql] INSERT INTO `user` (`name`, `age`, ...) VALUES (?, ?, ...) [alice 18 ...]
```

---

## gRPC 代码生成

### 前置依赖

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# 确保 /usr/local/include 中存在 google/protobuf/empty.proto
```

### 生成

```bash
crud -service -protopkg mypkg
```

### 生成的 proto（`user.api.proto`）

```proto
syntax = "proto3";
option go_package = "/api";
import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

service UserService {
    rpc CreateUser(User)                returns (User);
    rpc DeleteUser(UserId)              returns (google.protobuf.Empty);
    rpc UpdateUser(UpdateUserReq)       returns (User);
    rpc GetUser(UserId)                 returns (User);
    rpc ListUsers(ListUsersReq)         returns (ListUsersResp);
    rpc ListUsersMore(ListUsersMoreReq) returns (ListUsersMoreResp);
}

message UpdateUserReq {
    User           user  = 1;
    repeated UserField masks = 2; // 枚举字段掩码
}

message ListUsersReq {
    int32                page          = 1;
    int32                page_size     = 2;
    repeated UserOrderBy orderbys      = 3;
    repeated UserFilter  filters       = 4;
    repeated UserField   select_fields = 5;
}
```

### 生成的服务骨架（`user.service.go`）关键模式

```go
// CreateUser — 插入后从主库读取（写后读一致性）
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
    // 写后读：强制从主库查询
    a2, err := s.Client.Master.User.Find().Where(user.IdOp.EQ(a.Id)).One(ctx)
    if err != nil {
        return nil, status.Error(codes.Internal, err.Error())
    }
    return convertUser(a2), nil
}

// UpdateUser — 枚举字段掩码精确控制更新字段
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

// ListUsers — 动态过滤 + 多列排序 + 指定返回字段
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

### 与 golib 集成并用 grpcurl 测试

```go
import "github.com/goflower-io/golib/net/app"

a := app.New(app.WithAddr("0.0.0.0", 8080))
a.RegisteGrpcService(&api.UserService_ServiceDesc, &service.UserServiceImpl{Client: client})
a.Run()
```

```bash
# 安装 grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# 列出所有服务（golib 自动注册 gRPC 反射）
grpcurl -plaintext localhost:8080 list

# 查看 UserService 接口描述
grpcurl -plaintext localhost:8080 describe UserService

# 创建用户
grpcurl -plaintext \
  -d '{"name":"alice","age":18,"sex":1}' \
  localhost:8080 UserService/CreateUser

# 按 ID 查询用户
grpcurl -plaintext \
  -d '{"id":1}' \
  localhost:8080 UserService/GetUser

# 更新指定字段（masks 使用 UserField 枚举值：2=name, 3=age, 4=sex）
grpcurl -plaintext \
  -d '{"user":{"id":1,"name":"bob","age":25,"sex":0},"masks":[2,3]}' \
  localhost:8080 UserService/UpdateUser

# 分页查询用户列表
grpcurl -plaintext \
  -d '{"page":1,"page_size":10}' \
  localhost:8080 UserService/ListUsers

# 带过滤（age > 18）、排序（mtime 倒序）、指定返回字段（id+name+age）
grpcurl -plaintext \
  -d '{
    "page": 1,
    "page_size": 10,
    "filters":       [{"field":3,"op":"GT","val":"18"}],
    "orderbys":      [{"field":6,"desc":true}],
    "select_fields": [1,2,3]
  }' \
  localhost:8080 UserService/ListUsers

# 游标分页（适合大数据集，无需 COUNT(*)）
grpcurl -plaintext \
  -d '{"page_size":5,"cursor":{"orderbys":[{"field":1,"desc":false}]}}' \
  localhost:8080 UserService/ListUsersMore

# 删除用户
grpcurl -plaintext \
  -d '{"id":1}' \
  localhost:8080 UserService/DeleteUser
```

**UserField 枚举值对照表：**

| 枚举值 | 字段 |
|---|---|
| 1 | id |
| 2 | name |
| 3 | age |
| 4 | sex |
| 5 | ctime |
| 6 | mtime |

---

## CLI 参数

```
crud 参数说明：
  -dialect string   数据库方言：mysql | postgres | sqlite3  （默认 "mysql"）
  -protopkg string  生成 proto 文件的 go_package 包名
  -service          同时生成 gRPC proto 文件和服务骨架
```

---

## 最佳实践

1. 数值类型字段使用 `NOT NULL DEFAULT 0`，字符串类型使用 `NOT NULL DEFAULT ''`——避免 NULL 值引起的意外行为。
2. 将 `.sql` 文件和生成代码一起纳入版本控制——表结构变更可在 Code Review 中可见。
3. 写后读场景使用 `client.Master.User.Find()` 强制从主库读取，保证一致性。
4. 使用 [golib](https://github.com/goflower-io/golib) `app.New()` 在同一端口提供 gRPC 和 HTTP 服务，内置 Panic 恢复、结构化日志和 Prometheus 指标。
