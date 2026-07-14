---
title: "Go Context 与并发：让取消信号穿过真实调用链"
description: "以 Go 1.23 为基准，梳理 context 的取消、超时、值传递和 goroutine 生命周期管理。"
slug: "go-context-concurrency"
date: 2026-07-13T10:10:00+08:00
lastmod: 2026-07-13T10:10:00+08:00
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

## Context 不是“全局取消开关”

Go 1.23 的 `context.Context` 是一条请求或任务的控制通道，携带截止时间、取消信号和少量请求范围值。它不是用来存放业务状态的 map，也不能替代返回值和错误。典型调用链应把 `ctx` 放在第一个参数，并让每一层把它继续传给数据库、HTTP 客户端或下游服务。

```go
package main

import (
	"context"
	"fmt"
	"time"
)

func work(ctx context.Context) error {
	select {
	case <-time.After(200 * time.Millisecond):
		fmt.Println("work done")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	fmt.Println(work(ctx))
}
```

`Done` 是只读 channel，关闭意味着所有下游都应尽快停止；`Err` 通常返回 `context.Canceled` 或 `context.DeadlineExceeded`。取消函数必须调用，即使你认为任务会自然结束，因为它负责释放定时器和父子 context 的关联。

## 超时必须覆盖整个操作

只给最外层 handler 设置超时并不够。如果内部函数启动 goroutine、等待 channel 或重试请求，却没有监听 `ctx.Done()`，超时只是让调用方先返回，后台工作仍然存在。超时设计要回答两个问题：哪个资源拥有取消权，以及每个阻塞点如何被唤醒。

```go
package main

import (
	"context"
	"fmt"
	"time"
)

func producer(ctx context.Context, out chan<- int) {
	defer close(out)
	for i := 0; i < 5; i++ {
		select {
		case out <- i:
		case <-ctx.Done():
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	values := make(chan int)
	go producer(ctx, values)
	for value := range values {
		fmt.Println(value)
	}
	fmt.Println("finished:", ctx.Err())
}
```

生产者在发送前监听取消，消费者通过 `range` 等待关闭。实际服务中还要给 HTTP 请求、数据库查询和重试 backoff 设置各自的上限；子操作的 timeout 可以短于父操作，但不应超过父 context 的 deadline。

## 并发结构要有拥有者

goroutine 不是免费的函数调用。启动它的代码应知道它何时结束、错误交给谁、channel 谁关闭，以及取消后是否还有资源需要回收。`errgroup` 能把“一个失败取消其余任务”和“等待全部任务”组合起来，但它仍要求每个任务尊重 context。

```go
package main

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"time"
)

func main() {
	g, ctx := errgroup.WithContext(context.Background())
	for _, name := range []string{"cache", "profile"} {
		name := name
		g.Go(func() error {
			select {
			case <-time.After(30 * time.Millisecond):
				fmt.Println(name, "ready")
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}
	if err := g.Wait(); err != nil { fmt.Println(err) }
}
```

这段程序需要 `go get golang.org/x/sync/errgroup`，属于可直接运行的完整示例。循环变量在 goroutine 中要绑定到局部变量，避免闭包读取变化中的变量；Go 1.23 的循环变量语义已改进，但显式绑定仍能让代码意图和旧版本兼容性更清楚。

## 失败案例：超时返回了，goroutine 还在写

一个导出任务使用无缓冲 channel 把结果发回 handler。handler 的 context 超时后直接返回，后台 goroutine 仍执行 `results <- row`，由于再也没有消费者，它永久阻塞。任务量上升后，goroutine 数和连接数一起增长，最终表现为内存上涨和连接池耗尽。

修复不是简单把 channel 改成大容量。发送方必须用 `select` 同时监听 `ctx.Done()`，任务拥有者还要在退出路径等待 goroutine，必要时关闭结果 channel。对于批处理，优先使用有界 worker pool，限制同时运行的任务数，并把取消、队列满和下游失败都建模为可观察的错误。

## Value 使用边界

`context.WithValue` 只适合请求范围的元数据，例如 trace ID、认证后的主体或日志字段。key 应使用私有类型，避免包之间碰撞；不要把数据库连接、业务对象或可变配置塞进去。读取值失败时要有明确的默认策略，不能因为类型断言失败而 panic。

Context 也不应作为可选参数的替代品。若函数可以同步完成，就返回结果和错误；若必须后台执行，就定义任务 ID、状态和显式取消接口。这样调用方才能知道“请求结束”是否代表“工作完成”。

## 区分请求任务与后台任务

请求触发的工作不一定都应继承请求 context。生成当前响应所必需的查询必须继承，客户端断开后应停止；需要保证最终执行的审计、消息投递或报表任务，则不能简单沿用会随请求取消的 context。正确做法是先把任务持久化或交给受控队列，再由独立 worker 使用自己的生命周期 context 执行。

也不要为了让后台任务继续运行而随手改成 `context.Background()`。这样会丢失 trace 关联、租户信息和截止策略。可以显式提取允许传播的元数据，构造新任务对象，并为 worker 设置最大执行时间。Go 1.21 引入的 `context.WithoutCancel` 能断开取消关系，但它也移除了 deadline 和 `Err`，使用前必须确认任务仍有新的退出边界。

## 错误原因与可观测性

只记录 `ctx.Err()` 往往不足以区分谁取消了任务。`context.WithCancelCause` 和 `context.Cause` 可以保留业务原因，例如上游熔断、服务关闭或首个并发分支失败。对外响应仍应转换成稳定错误码，内部日志和 trace 则记录 cause，避免把所有取消都归为客户端超时。

指标至少要区分 deadline exceeded、主动 canceled、正常完成和内部失败，并记录任务排队时间与实际执行时间。若大量任务在 deadline 到来前才开始，问题可能在队列或连接池，而不是具体 SQL。goroutine 数只能作为症状指标，结合活跃请求、阻塞 profile 和下游延迟才有诊断意义。

## 测试取消路径

并发测试不要依赖固定 `Sleep` 猜测调度顺序。通过 channel 通知“任务已进入阻塞点”，测试再调用 cancel，并用有限 timeout 等待退出。这样可以稳定覆盖取消前、处理中和完成后三条路径。测试还应断言资源释放，例如连接归还、结果 channel 关闭和 worker 计数回落。

竞态检测器能发现未同步访问，但不能证明没有 goroutine 泄漏，也不能证明业务取消及时。对长期运行服务，可在压力测试前后比较 goroutine profile，并给每个可阻塞操作设计明确的退出条件。

## 官方资料与检查方法

- [context package](https://pkg.go.dev/context)：Go 标准库中取消、deadline、`WithCancel` 和 `WithTimeout` 的契约。
- [Go Concurrency Patterns: Context](https://go.dev/blog/context)：官方博客对请求范围取消和调用链传播的说明。
- [Go Blog: Pipelines](https://go.dev/blog/pipelines)：channel、关闭和取消在流水线中的组合方式。

版本说明：本文按 Go 1.23 编写。验证并发代码时可使用 `go test -race ./...`，并配合 goroutine、请求耗时和取消计数指标；文章没有虚构任何测试输出，示例中的打印结果会随取消时序变化。
