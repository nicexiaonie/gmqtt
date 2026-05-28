## Why

MQTT shared subscription 是多个客户端以同一个消费组共同消费同一 topic filter 的标准能力。当前库只提供普通订阅 API，调用方需要手动拼接 `$share/{group}/{filter}`，并且 wildcard 或 shared filter 收到真实消息 topic 时可能无法路由到正确 handler。

## What Changes

- 增加面向 shared subscription 的一等 Client API，调用方传入 group 和 topic filter，由库构造 MQTT 标准订阅 filter。
- 增加批量 shared subscription API，支持同一个 group 下多个 topic filter 使用不同 QoS。
- 增加 shared unsubscribe API，调用方使用 group 和原始 topic filter 取消订阅。
- 修正 handler 分发逻辑，使普通 wildcard filter 和 shared wildcard filter 都能匹配真实消息 topic。
- 增加 shared subscription 输入校验，非法 group 或空 filter 在调用 broker 前失败。

## Capabilities

### New Capabilities
- `shared-subscription`: 定义 shared subscription API、输入校验、取消订阅，以及普通 wildcard/shared wildcard filter 的 handler 分发行为。

### Modified Capabilities

## Impact

- 影响 `Client` 的订阅 API 和消息分发逻辑。
- 可能需要调整订阅状态结构，以同时支持精确查找和按注册顺序匹配 wildcard/shared filter。
- 增加 shared topic 构造、输入校验、filter 匹配、shared subscribe/unsubscribe 行为的测试。
- 不引入新的运行时依赖。
