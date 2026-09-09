# Requirements Document

## Introduction

为已认证用户提供每日余额签到奖励。

## Glossary

- **签到日**：服务端配置时区中的日历日期。
- **签到奖励**：每个签到日为用户余额增加的 0.20 美元。

## Requirements

### Requirement 1

**User Story:** AS 已认证用户, I want 每日领取余额奖励, so that 我可以获得持续使用额度。

#### Acceptance Criteria

1. WHEN 用户提交每日签到请求，系统 SHALL 为当前签到日首次签到的用户增加 0.20 美元余额。
2. WHILE 用户已在当前签到日完成签到，系统 SHALL 返回已签到状态和当前余额。
3. WHEN 多个同日签到请求并发到达，系统 SHALL 仅发放一次签到奖励。
4. WHEN 用户打开每日签到页面，系统 SHALL 显示当前签到日的领取状态、奖励金额和当前余额。
