---
title: "GORM 建模与迁移：让数据库演进可审计"
description: "以 GORM 1.30.0 为基准，讨论模型边界、索引设计、AutoMigrate 的适用范围与可回滚迁移。"
slug: "gorm-modeling-migrations"
date: 2026-07-13T10:40:00+08:00
lastmod: 2026-07-13T10:40:00+08:00
draft: false
comments: true
toc: true
readingTime: true
categories:
  - "technology"
tags:
  - "go"
  - "engineering-practice"
  - "architecture"
---

## 模型是边界，不是表结构复刻

GORM 1.30.0 的 struct tag 很方便，但模型设计首先要回答业务问题：哪些字段由服务拥有，哪些来自外部系统，哪些状态允许回退，哪些数据需要唯一约束。把数据库表一比一映射成公开请求 DTO，会把存储细节泄露到 API，也会让一次字段迁移变成全链路改动。

建议把请求对象、领域对象和持久化模型分开。持久化模型可以有 `CreatedAt`、软删除字段和数据库专用冗余列；请求对象只接受明确允许修改的字段。字段名、零值、默认值和 NULL 语义要在代码评审中一起确定，不能只看 Go 类型。

```go
package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID        uint   `gorm:"primaryKey"`
	Email     string `gorm:"size:255;not null;uniqueIndex"`
	Name      string `gorm:"size:80;not null"`
	CreatedAt int64
}

func main() {
	db, err := gorm.Open(sqlite.Open("file:demo?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { panic(err) }
	if err = db.AutoMigrate(&User{}); err != nil { panic(err) }
	user := User{Email: "a@example.com", Name: "A"}
	result := db.Create(&user)
	fmt.Println(result.Error, user.ID)
}
```

运行示例需要 `go get gorm.io/gorm@v1.30.0 gorm.io/driver/sqlite`。`uniqueIndex` 表达的是数据库约束，不是仅供 GORM 查询的提示；业务层仍应处理插入时的唯一冲突，因为并发下“先查询再插入”无法代替唯一索引。

## 类型、NULL 与默认值

Go 的零值和 SQL 的 NULL 不是一回事。`bool` 无法区分“未提供”和 `false`，`time.Time` 也无法自然表示数据库 NULL。对可选字段可使用指针或 `sql.Null*` 类型，但要统一序列化和业务判断；如果字段必须有值，数据库使用 `NOT NULL`，应用模型也不要让它长期处于不完整状态。

默认值同样要明确由谁负责。数据库默认值能覆盖多种写入路径，适合创建时间、状态等基础字段；应用默认值更容易被单元测试和领域规则复用。不要同时依赖两套不同默认逻辑，否则批量导入、原生 SQL 和 GORM 写入可能产生不同结果。

## AutoMigrate 的边界

`AutoMigrate` 适合开发环境初始化或简单追加表、列、索引，但不应被当作完整版本迁移系统。生产变更常包含重命名、拆分列、回填数据、双写窗口和删除旧字段，这些都需要显式的、有序的、可审计脚本。自动迁移也不会替你设计锁定时间、批量回填和回滚策略。

```go
package main

import (
	"log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Migration struct { ID int64 `gorm:"primaryKey"`; Name string `gorm:"uniqueIndex"` }

func main() {
	db, err := gorm.Open(sqlite.Open("migration.db"), &gorm.Config{})
	if err != nil { log.Fatal(err) }
	if err := db.AutoMigrate(&Migration{}); err != nil { log.Fatal(err) }
	var count int64
	db.Model(&Migration{}).Count(&count)
	log.Println("migration records:", count)
}
```

真实项目应把迁移版本记录放在数据库中，部署脚本按版本顺序执行，并在每一步记录耗时和影响行数。破坏性迁移可以分成 expand、migrate、contract：先添加兼容结构，再发布代码回填和切换，确认旧版本不再访问后最后删除旧结构。

## 失败案例：改名被当成新增列

团队把 `nickname` 改成 `display_name`，直接修改了 struct tag 并在发布时调用 `AutoMigrate`。数据库里保留了旧列，新列为空，读取逻辑因默认值看起来还能工作；一段时间后旧数据和新数据分裂，报表按旧列统计与 API 按新列展示不一致。

修复要先新增 `display_name`，再批量回填并验证非空与长度，代码进入兼容读阶段；随后改为写新列，必要时短期双写，最后再删除旧列。若数据量很大，回填需要分批、可暂停、可重试，并监控锁等待和复制延迟。字段重命名不是文本替换，而是数据迁移。

## 索引与查询形状一起设计

索引要服务于稳定查询形状。联合索引的列顺序要结合等值过滤、范围过滤和排序；唯一索引既是性能结构，也是并发正确性约束。GORM 的 `Preload` 能减少手写关联查询，但可能增加查询次数和返回数据量，列表接口应显式限制字段、分页和关联深度。

发布前使用真实数据分布检查执行计划，关注慢查询、回表、索引选择和迁移期间的锁。模型 tag 不能代替数据库审计：实际 schema、约束、字符集和线上版本都需要进入变更记录。

## 关联模型与删除语义

一对多、多对多和多态关联会迅速放大模型复杂度。是否使用级联删除必须根据数据生命周期决定：删除用户不一定意味着物理删除订单，删除文章也可能需要保留审计记录。外键约束能保护引用完整性，但上线前要评估历史脏数据和创建约束时的锁。不要只在 GORM tag 中声明关联，却不检查数据库是否真的创建成功。

软删除适合保留恢复窗口，却会让每个唯一约束和统计查询都更复杂。邮箱唯一是否包含已删除用户、恢复时发生冲突怎么办，都要在 schema 里回答。若合规要求真正删除个人数据，软删除标记并不能满足要求，还需要单独的擦除流程和审计证明。

## 发布迁移的验证闭环

迁移前记录目标 schema、预计影响行数、可接受锁时间和回滚条件；迁移中观察数据库连接、锁等待、复制延迟与应用错误率；迁移后同时验证结构和数据。只检查命令退出码不够，回填可能成功执行却漏掉部分租户或不满足业务约束。

每个版本应能回答“已执行到哪一步”。DDL、回填任务和应用发布使用独立版本标识，失败后可以从批次游标继续。回滚也不总是反向 DDL：数据已经按新逻辑写入后，直接删列会造成二次损失，通常应先停止写入、恢复兼容读，再决定如何转换数据。

## 官方资料与版本边界

- [GORM 官方文档](https://gorm.io/docs/)：GORM 1.30.0 的模型、查询和配置 API。
- [GORM Migration](https://gorm.io/docs/migration.html)：`AutoMigrate`、迁移接口与生产边界说明。
- [GORM Conventions](https://gorm.io/docs/conventions.html)：表名、主键、时间字段和命名约定。

版本说明：本文按 Go 1.23、GORM 1.30.0 编写。示例使用 SQLite 便于运行，生产数据库的锁、索引和 DDL 行为仍需在目标数据库版本上验证；本文没有虚构迁移耗时或测试输出。
