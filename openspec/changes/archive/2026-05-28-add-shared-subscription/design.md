## Context

当前 Client API 支持普通单 topic 订阅和批量订阅。订阅状态以订阅字符串作为 key 保存，但收到消息时使用真实消息 topic 查找 handler。这个模型对精确 topic 可用，但对 wildcard filter 不可靠；shared subscription 也有同样问题，因为 broker 投递消息时传入的是原始发布 topic，而不是 `$share/{group}/...` 订阅 filter。

shared subscription 应建模为 MQTT broker 侧消费组语义，而不是本地 topic 分组。库的职责是提供安全清晰的 API、构造标准 shared subscription filter、校验输入，并把收到的真实 topic 路由到正确 handler。

## Goals / Non-Goals

**Goals:**
- 提供单个 shared subscription 的 Client API。
- 提供同一 group 下多个 topic filter 的批量 shared subscription API。
- 提供 shared unsubscribe API。
- 保持现有普通订阅 API 兼容。
- 支持精确 topic、普通 wildcard filter、shared wildcard filter 的 handler 分发。
- 在调用 broker 前校验 shared subscription 的 group 和 topic filter 输入。

**Non-Goals:**
- 不实现客户端侧负载均衡；shared subscription 的消息分配由 broker 负责。
- 不引入本地 topic group 注册中心。
- 不修改 `MessageHandler` 签名。
- 不改变发布语义。

## Decisions

1. 增加显式 shared subscription API，而不是要求调用方手动传入 `$share/...` 字符串。

   原因：库可以统一校验 group、构造标准订阅 filter，并让 API 意图更清楚。只提供字符串 helper 不能解决 handler 分发问题，也会把正确性负担留给每个调用方。

2. 订阅状态仍按 broker-facing filter 保存，同时为非精确匹配维护注册顺序。

   原因：unsubscribe 和订阅查询应继续反映实际发给 broker 的 filter；但消息分发需要根据真实 topic 匹配 wildcard/shared filter。精确 topic 可以继续快速查找，wildcard/shared filter 需要稳定遍历顺序，因为同一真实 topic 可能匹配多个 filter。依赖 map 遍历会导致 handler 分发不确定。

3. shared filter 使用 broker-facing filter 存储，但匹配时使用逻辑 filter。

   原因：`$share/group/a/+/c` 是发给 broker 的订阅 filter，但收到 `a/x/c` 时应按 `a/+/c` 匹配。handler 仍接收真实消息 topic。把 `$share` 前缀暴露给 handler 不符合 MQTT 投递语义。

4. 输入校验只覆盖结构正确性。

   原因：库应拒绝空 group、空 filter，以及包含 `/`、`+`、`#` 的 group，因为这些会破坏 `$share/{group}/{filter}` 结构。更严格的 broker 特定 topic 策略不在本次范围内。

## Risks / Trade-offs

- wildcard 匹配错误可能把消息路由到错误 handler → 增加精确 topic、单层 wildcard、多层 wildcard、shared wildcard 的单测。
- 多个 filter 可能匹配同一真实 topic → 明确规则为精确 topic 优先，其余 wildcard/shared filter 按注册顺序选择第一个匹配项。
- shared subscription 依赖 broker 支持 → broker 不支持时保持订阅错误原样返回。
- 批量 shared subscribe 失败可能留下本地状态 → 复用现有批量订阅失败清理思路，删除本次失败尝试创建的本地订阅项。
