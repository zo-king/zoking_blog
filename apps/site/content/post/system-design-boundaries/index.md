---
title: "系统设计笔记：先建立边界，再选择技术"
description: "面对复杂项目时，如何从职责、数据和演进路径开始做工程决策。"
slug: "system-design-boundaries"
date: 2026-07-05T20:15:00+08:00
lastmod: 2026-07-11T16:38:51+08:00
draft: false
comments: true
toc: true
readingTime: true
series:
  slug: "long-term-practice"
  name: "长期主义实践"
  order: 1
categories:
  - "technology"
tags:
  - "go"
  - "engineering-practice"
  - "architecture"
---

技术选型很重要，但它通常不是系统设计的第一步。先回答谁负责什么、数据以谁为准、失败后如何恢复，方案会清晰很多。

![抽象建筑结构](/img/showcase/architecture.jpg)

## 从稳定边界开始

将内容编辑、静态发布和读者访问拆成清晰职责后，每个模块都可以独立演进。数据库保存编辑事实，发布系统生成可回滚的静态版本，前台专注阅读体验。

## 为变化保留余地

成熟架构不是预测所有未来，而是让高概率变化发生时，不必推翻整个系统。接口、迁移、审计和回滚能力，往往比某个具体框架更值得优先设计。
