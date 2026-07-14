---
title: "GORM 事务与锁：把并发更新写成可证明的流程"
description: "基于 GORM 1.30.0，说明事务边界、行锁、乐观锁和错误回滚在订单类流程中的组合方式。"
slug: "gorm-transactions-locking"
date: 2026-07-13T10:50:00+08:00
lastmod: 2026-07-13T10:50:00+08:00
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

## 先定义不变量和事务范围

事务不是“把几行代码包起来”的装饰，而是让一组数据库状态变化满足原子性。以扣减库存为例，不变量可能是库存不能小于零、订单创建必须对应一次扣减、重复请求不能重复扣减。只有先写出不变量，才能判断哪些读和写必须在同一个事务中。

事务范围通常覆盖一个业务用例，不应跨越网络调用、用户输入等待或长时间计算。支付、消息投递等外部动作应通过 outbox、状态机或补偿流程与本地事务衔接，而不是把远程调用塞进数据库锁持有期间。

```go
package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Account struct { ID uint `gorm:"primaryKey"`; Balance int64 }

func transfer(db *gorm.DB, from, to uint, amount int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var src Account
		if err := tx.First(&src, from).Error; err != nil { return err }
		if src.Balance < amount { return fmt.Errorf("insufficient balance") }
		if err := tx.Model(&Account{}).Where("id = ?", from).Update("balance", gorm.Expr("balance - ?", amount)).Error; err != nil { return err }
		if err := tx.Model(&Account{}).Where("id = ?", to).Update("balance", gorm.Expr("balance + ?", amount)).Error; err != nil { return err }
		return nil
	})
}

func main() {
	db, err := gorm.Open(sqlite.Open("file:tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { panic(err) }
	if err = db.AutoMigrate(&Account{}); err != nil { panic(err) }
	db.Create(&Account{Balance: 100}); db.Create(&Account{Balance: 0})
	fmt.Println(transfer(db, 1, 2, 40))
}
```

`db.Transaction` 在回调返回错误时回滚，返回 nil 时提交；不要在回调内部吞掉关键错误。示例中的余额检查仍只是业务判断，生产中还需根据数据库类型设计并发控制，否则两个事务可能同时读到同一余额。

## 悲观锁要配合确定顺序

需要锁住已读取的行时，可使用 GORM 的 clause：

```go
package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Stock struct { ID uint `gorm:"primaryKey"`; Available int64 }

func reserve(tx *gorm.DB, id uint, n int64) error {
	var stock Stock
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&stock, id).Error; err != nil { return err }
	if stock.Available < n { return gorm.ErrInvalidData }
	return tx.Model(&stock).Update("available", gorm.Expr("available - ?", n)).Error
}

func main() {
	db, err := gorm.Open(sqlite.Open("file:lock?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { panic(err) }
	if err = db.AutoMigrate(&Stock{}); err != nil { panic(err) }
	db.Create(&Stock{Available: 10})
	err = db.Transaction(func(tx *gorm.DB) error { return reserve(tx, 1, 3) })
	var stock Stock
	db.First(&stock, 1)
	fmt.Println(err, stock.Available)
}
```

这段函数应在事务中调用。`FOR UPDATE` 等语义由目标数据库方言决定，SQLite 不提供与 MySQL、PostgreSQL 相同的行锁行为，因此不能用内存 SQLite 的演示结果推断线上并发行为。多个资源需要加锁时按固定 ID 顺序获取，减少死锁；设置合理的锁等待超时，并对可重试的死锁错误做有限重试。

## 乐观锁适合短更新

不希望长时间持有锁时，可在表中增加 `version`，更新时带上旧版本条件：`UPDATE ... SET value=?, version=version+1 WHERE id=? AND version=?`。受影响行数为零表示发生冲突，服务返回冲突让调用方重新读取。GORM 生态中也有 optimisticlock 插件，但无论是否使用插件，必须检查 RowsAffected，不能只看 Error。

数据库原子表达式通常优于“先查到内存、再写回”的两步更新。库存扣减可以写成 `WHERE available >= ?` 的单条 UPDATE，然后检查影响行数；这让“不足时不更新”由数据库在并发条件下保证。

## 失败案例：事务开了，但关键写入用了 db

一个订单函数通过 `db.Transaction(func(tx *gorm.DB) error { ... })` 开启事务，却在某个 repository 中继续使用全局 `db.Create(&Log{})`。订单主表写入失败后被回滚，审计日志却已经提交，系统出现“没有订单但有成功日志”的事实矛盾。

修复是把 `tx` 作为依赖传入所有需要参与事务的 repository，或让 repository 接收统一的 `*gorm.DB` 执行器。代码评审应搜索事务回调内部是否出现全局连接、异步 goroutine 或网络调用。异步任务不能安全地引用事务中的未提交数据，应该在提交后由 outbox 触发。

## 隔离级别、死锁与重试

默认隔离级别取决于数据库。脏读、不可重复读、幻读和写偏差的风险不同，不能只用“加事务”概括。关键流程要明确读的是当前版本、锁定版本还是快照，并通过目标数据库的并发测试验证。死锁不是数据库坏了，而是不同事务以不同顺序持锁的结果；固定顺序、缩短事务和合理索引通常比无限重试更重要。

重试必须有上限、退避和幂等键。事务回调可能被重新执行，不能在其中发送不可撤销的邮件或扣款请求。错误日志应带业务键、事务耗时、RowsAffected 和数据库错误码，避免只打印一行“transaction failed”。

## 幂等键与唯一约束

客户端超时后通常会重试，而第一次事务可能已经提交。订单创建、充值和消息消费应接收稳定幂等键，并在数据库建立唯一约束。处理流程先尝试创建幂等记录；遇到唯一冲突时读取已完成结果，不能只在内存 map 中记忆，因为进程重启和多副本会绕过它。

幂等不等于所有错误都返回成功。若同一个 key 携带了不同金额或不同用户，应拒绝并记录冲突；只有参数摘要一致时才能复用旧结果。幂等记录与业务写入最好处于同一事务，否则可能出现记录已存在但订单未创建的中间状态。

## Context 与连接池边界

事务查询应使用 `db.WithContext(ctx).Transaction(...)`，让请求取消能够中断等待和数据库操作。但取消到来时，提交是否已经发生取决于具体时序，调用方不能看到 timeout 就断言“数据库没有写入”。对外部可重试接口仍要依赖幂等键确认最终结果。

长事务会长期占用连接并扩大锁范围。连接池最大连接数、锁等待超时和请求并发需要一起设计：如果每个请求开启事务后又等待另一个数据库连接，很容易形成池内自我阻塞。指标应区分等待连接、执行 SQL、等待锁和提交四段耗时。

## 如何做并发验证

不要只用单线程单元测试验证余额或库存。集成测试应在目标数据库容器中启动多个 worker，以同一个业务键并发执行，最终检查不变量、成功数量和错误类型。测试数据每次独立创建，避免不同用例共享行锁和幂等记录。

并发测试可以证明特定场景下的实现行为，却不能覆盖所有调度。关键约束仍应尽量落到数据库的唯一键、检查约束和条件 UPDATE 上，使错误操作无法提交；测试负责验证应用能正确解释冲突并释放事务资源。

## 官方资料与版本边界

- [GORM Transactions](https://gorm.io/docs/transactions.html)：事务、嵌套事务和回滚 API。
- [GORM Advanced Query](https://gorm.io/docs/advanced_query.html)：锁定子句和高级查询写法。
- [GORM Update](https://gorm.io/docs/update.html)：条件更新、表达式和 RowsAffected 语义。

版本说明：本文按 Go 1.23、GORM 1.30.0 编写。锁的具体行为必须以 MySQL、PostgreSQL 或实际使用的数据库文档为准；示例只展示 API 形状，不宣称任何未执行的并发测试结果。
