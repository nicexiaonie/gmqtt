## 1. Shared Subscription API

- [x] 1.1 增加 shared subscription topic 构造 helper，并校验 group name 和 topic filter
- [x] 1.2 增加 Client 级单个 shared subscribe API，订阅 `$share/{group}/{filter}` 并保存本地订阅状态
- [x] 1.3 增加 Client 级批量 shared subscribe API，将原始 filters 映射为 shared broker filters，并在失败时清理本地状态
- [x] 1.4 增加 Client 级 shared unsubscribe API，接收 group 和原始 filters，并取消对应 shared broker filters

## 2. Handler Dispatch

- [x] 2.1 增加 MQTT topic filter 匹配逻辑，支持精确 topic、`+` 和 `#`
- [x] 2.2 增加 shared subscription filter 匹配逻辑，匹配前剥离 `$share/{group}/` 前缀
- [x] 2.3 为非精确 handler 匹配维护订阅注册顺序
- [x] 2.4 更新消息分发逻辑，优先使用精确 topic handler，再按注册顺序匹配 wildcard 或 shared filter handler
- [x] 2.5 保持无订阅 filter 命中时调用 default handler 的现有行为

## 3. Tests and Verification

- [x] 3.1 增加 shared subscription topic 构造和非法输入校验的纯单元测试
- [x] 3.2 增加 MQTT wildcard 匹配、shared wildcard 匹配、注册顺序分发选择的纯单元测试
- [x] 3.3 增加 Client 级 shared subscribe/unsubscribe 本地状态行为测试，尽量不依赖 live broker
- [x] 3.4 在现有集成测试环境支持时，增加或更新 broker-backed shared subscription 行为测试
- [x] 3.5 运行相关 Go 测试套件，并修复本次变更引入的失败
