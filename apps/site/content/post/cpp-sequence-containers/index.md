---
title: "C++20 序列容器工程实践：vector、deque 与 list 怎么选"
description: "从连续内存、迭代器失效和操作复杂度出发，建立 C++20 序列容器的选择方法。"
slug: "cpp-sequence-containers"
date: 2026-07-13T11:00:00+08:00
lastmod: 2026-07-13T11:00:00+08:00
draft: false
comments: true
toc: true
readingTime: true
categories:
  - "technology"
tags:
  - "engineering-practice"
  - "architecture"
---

## 先描述访问和修改形状

C++20 的序列容器选择，不能只背“vector 快、list 插入快”。需要先描述工作负载：是否频繁按下标访问，是否需要稳定地址，是否从两端进出，是否在中间插入，元素是否昂贵移动，以及数据规模是否足以让缓存局部性成为主导因素。多数业务集合从 `std::vector` 开始，因为连续内存让遍历、排序和批量处理更容易获得稳定性能。

`std::deque` 提供两端高效插入和随机访问，但元素不保证整体连续；`std::list` 节点分散，拥有稳定迭代器和常数时间的已知节点插入，却要承担指针、分配和缓存未命中的成本。容器的复杂度表只是上限描述，实际性能要结合元素大小和访问模式测量。

```cpp
#include <algorithm>
#include <iostream>
#include <vector>

int main() {
    std::vector<int> values;
    values.reserve(5);
    for (int value : {4, 1, 3, 2}) values.push_back(value);
    std::ranges::sort(values);
    for (int value : values) std::cout << value << ' ';
    std::cout << '\n';
}
```

这是可用 C++20 编译的完整程序：`c++ -std=c++20 -O2 main.cpp -o main`。`reserve` 只提前容量，不改变 `size`；不能因为容量足够就通过下标访问尚未构造的元素。

## vector 的容量与失效规则

`vector` 扩容时会分配新存储并移动或复制元素，旧的指针、引用和迭代器通常全部失效。即使没有扩容，在指定位置插入和删除也可能让该位置之后的引用失效。保存 `&values[0]` 跨越 `push_back` 是典型错误，尤其在异步回调中更难排查。

如果最终元素数量可预估，`reserve` 能减少重分配；如果需要释放多余容量，`shrink_to_fit` 只是非强制请求，不能当作确定的内存归还操作。元素本身是大对象时，考虑存放可移动的句柄或稳定对象，但要重新评估所有权和生命周期。

```cpp
#include <deque>
#include <iostream>
#include <iterator>
#include <list>
#include <vector>

int main() {
    std::deque<int> queue;
    queue.push_front(2);
    queue.push_back(3);
    std::list<int> linked{1, 3};
    auto where = std::next(linked.begin());
    linked.insert(where, 2);
    std::vector<int> values{1, 2, 3};
    values.erase(values.begin() + 1);
    std::cout << queue.front() << ' ' << linked.size() << ' ' << values.size() << '\n';
}
```

`list::insert` 需要一个有效位置迭代器，`vector::erase` 会移动后续元素。不要把“插入是 O(1)”脱离寻找插入位置的成本：如果每次都从头遍历到中间，整体仍可能是线性甚至更差。

## 连续性往往比理论复杂度重要

现代 CPU 会预取连续访问的数据，`vector` 的顺序遍历通常能胜过节点容器，即使两者都写成 O(n)。节点容器每个节点可能单独分配，访问路径包含指针跳转和更多缓存 miss。对日志、批处理、排序、序列化和 SIMD 友好的数据，连续布局通常是首选。

`deque` 在两端队列场景中更自然，避免 vector 头部插入造成整体移动；如果只需要先进先出且不要求随机访问，可以考虑 `deque` 而不是用 vector 自己维护环形下标。`list` 适合确实需要节点稳定性、频繁在已知位置拼接且元素移动代价高的场景，不能仅因为“中间插入”四个字就选择它。

## 失败案例：保存引用后继续 push_back

某缓存先取出 `auto& entry = items.front()`，随后为了补充数据连续 `push_back`。当 vector 发生扩容，`entry` 指向的旧内存已被释放；后续读取可能出现崩溃或静默错误。预留容量只能降低概率，不能修复“引用跨越可能失效操作”的设计。

修复方式是保存稳定的业务 ID，扩容后重新查找；或者让容器在完成构建后再暴露引用；或者选择满足稳定地址需求的对象所有权结构。代码评审中应把迭代器、指针和引用视作带有效期的借用，不要把它们当作永久句柄。

## 接口设计与异常安全

返回容器时优先让返回值表达所有权，避免暴露内部容器的可变引用。接受范围时可使用 `std::span` 表达借用的连续视图，但调用方必须保证底层存储在使用期间有效；对 `deque` 和 `list` 不能直接当作连续 span。需要通用遍历时使用 iterator/range 接口，而不是为了统一接口牺牲布局。

容器操作可能触发分配和元素移动，移动构造是否 `noexcept` 会影响实现选择和异常保证。对强异常安全要求的更新操作，先构造临时结果，再通过 `swap` 或移动提交；不要在异常中依赖部分完成的索引或裸指针。

## 元素类型会改变容器表现

容器选择必须连同元素类型一起评估。`vector<std::string>` 移动元素通常比复制便宜，但自定义类型若移动构造可能抛异常，扩容时实现为了强异常保证可能退回复制。给真正不会抛出的移动构造标记 `noexcept`，同时确保移动后对象仍处于可析构、可赋值的有效状态。

存放 `std::unique_ptr<T>` 可以保持 `T` 的地址稳定并降低移动成本，但增加一次间接访问和独立分配；`std::shared_ptr` 还引入引用计数与共享所有权。不要只为了“vector 扩容会动”就把所有对象改成智能指针。若对象很小且可移动，直接存值往往更简单、更紧凑。

## 删除、批处理与内存策略

从 vector 删除满足条件的元素，C++20 可使用 `std::erase_if`，它会压缩后续元素并改变相关引用。批量删除优于循环中每找到一个就 erase，因为后者可能反复移动尾部。若顺序不重要，可用尾元素覆盖待删位置后 `pop_back`，但接口必须明确顺序不再保持。

高频短生命周期容器可评估 `std::pmr` 内存资源，让一批节点或元素使用统一的分配策略。它适合经过 profile 证明的分配热点，不是默认优化。容器性能测试应包含构建、遍历、修改和销毁完整生命周期，并使用实际元素类型，避免只测 `int` 得出错误结论。

## 官方资料与选择流程

- [ISO C++ draft: vector](https://eel.is/c++draft/vector)：C++ 标准中 `vector` 的要求、容量和失效规则。
- [ISO C++ draft: deque](https://eel.is/c++draft/deque)：`deque` 的操作、迭代器和存储要求。
- [ISO C++ draft: list](https://eel.is/c++draft/list)：链表节点操作和迭代器语义。

版本说明：本文按 C++20 编写，编译示例使用 `-std=c++20`。复杂度和失效规则来自标准要求；具体性能要在目标编译器、标准库和数据规模上基准测试，不应把示例输出当作性能结论。
