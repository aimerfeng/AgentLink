# Implementation Plan: AgentLink Platform

## Overview

本实现计划将 AgentLink 平台分为 10 个主要阶段，按照依赖关系和优先级排序。每个任务都关联到具体的需求，确保完整覆盖所有功能�?

**GitHub 仓库**: https://github.com/aimerfeng/AgentLink.git

**提交策略**:
- 功能完成 �?提交�?`main` 分支
- 遇到报错 �?创建 `error-fix/*` 分支，修复后合并

## Phase 0: 环境准备与仓库初始化

- [x] 0. 初始化 Git 仓库和环境
  - [x] 0.1 克隆并配置 Git 仓库
    - 克隆 https://github.com/aimerfeng/AgentLink.git
    - 配置 .gitignore 文件
    - 创建 develop 分支
    - _Requirements: 设计文档 Git Workflow_
  - [x] 0.2 创建环境配置文件
    - 创建 .env.example 模板文件
    - 配置所有必需的环境变量
    - 创建 .env.local (本地开发)
    - _Requirements: 设计文档 Configuration Management_
  - [x] 0.3 安装开发依赖
    - 安装 Go 1.22+
    - 安装 Node.js 20+
    - 安装 Docker 和 Docker Compose
    - 安装 golangci-lint
    - _Requirements: 设计文档 Technical Stack_
  - [x] 0.4 配置外部服务账号
    - 注册 Stripe 测试账号，获取 API Key
    - 注册 Coinbase Commerce 账号
    - 获取 OpenAI/Anthropic/Google AI API Key
    - 配置 AWS S3 或兼容存储(MinIO 本地开发)
    - 配置邮件服务 (SendGrid/Resend)
    - _Requirements: R6, R9, R19_
  - [x] 0.5 创建 README.md
    - 项目介绍
    - 快速开始指南
    - 环境配置说明
    - _Requirements: 项目文档_

## Phase 1: 项目初始化与基础架构

- [-] 1. 初始化项目结�?
  - [x] 1.1 创建 Go 后端项目结构
    - 初始�?Go module，创�?cmd/api、cmd/proxy、internal 目录结构
    - 配置 Makefile 和基础构建脚本
    - _Requirements: 设计文档 Project Structure_
  - [x] 1.2 创建 Next.js 前端项目结构
    - 使用 create-next-app 初始�?Next.js 14 项目
    - 配置 Tailwind CSS �?Shadcn UI
    - 创建 app router 目录结构
    - _Requirements: R11.2, 设计文档 Frontend Architecture_
  - [x] 1.3 配置 Docker 开发环�?
    - 创建 docker-compose.yml 包含 PostgreSQL、Redis、pgvector
    - 创建各服务的 Dockerfile
    - _Requirements: 设计文档 Deployment Architecture_

- [ ] 2. 数据库设计与迁移
  - [ ] 2.1 创建 PostgreSQL Schema
    - 实现 users、agents、api_keys、quotas 等核心表
    - 创建索引和约�?
    - _Requirements: R1, R2, R4, 设计文档 Data Models_
  - [ ] 2.2 配置 pgvector 扩展
    - 创建 knowledge_embeddings 表和向量索引
    - _Requirements: R9, 设计文档 Vector Database Schema_
  - [ ] 2.3 实现数据库迁移工�?
    - 使用 golang-migrate 管理迁移
    - _Requirements: 设计文档 Data Models_

- [ ] 3. 配置管理与基础设施
  - [ ] 3.1 实现配置加载模块
    - 从环境变量加载配�?
    - 实现配置验证
    - _Requirements: 设计文档 Configuration Management_
  - [ ] 3.2 实现 Redis 连接�?
    - 配置 Redis 客户�?
    - 实现连接健康检�?
    - _Requirements: R5.6, 设计文档 Redis 数据结构_
  - [ ] 3.3 实现日志和监控基础
    - 配置 zerolog 结构化日�?
    - 配置 Prometheus 指标收集
    - _Requirements: R18.6, 设计文档 Monitoring & Observability_

- [ ] 4. Checkpoint - 基础架构验证
  - 确保所有基础服务可以启动
  - 验证数据库连接和迁移
  - 确保 Docker 环境正常运行

## Phase 2: 用户认证系统

- [ ] 5. 实现认证服务
  - [ ] 5.1 实现用户注册功能
    - 创建 POST /api/v1/auth/register 接口
    - 实现邮箱验证逻辑
    - 使用 Argon2id 哈希密码
    - _Requirements: R1.1_
  - [ ] 5.2 编写用户注册属性测�?
    - **Property 35: Free Quota Initialization**
    - **Validates: Requirements 4.1**
  - [ ] 5.3 实现用户登录功能
    - 创建 POST /api/v1/auth/login 接口
    - 实现 JWT Token 生成
    - _Requirements: R1.3, R1.4_
  - [ ] 5.4 编写认证属性测�?
    - **Property 5: Authentication Correctness**
    - **Validates: Requirements 5.1, 5.2**
  - [ ] 5.5 实现 Token 刷新机制
    - 创建 POST /api/v1/auth/refresh 接口
    - 实现 Refresh Token 轮换
    - _Requirements: R1.3_

- [ ] 6. 实现钱包绑定功能
  - [ ] 6.1 创建钱包绑定接口
    - 创建 PUT /api/v1/creators/me/wallet 接口
    - 实现以太坊地址格式验证
    - _Requirements: R1.2_
  - [ ] 6.2 编写钱包地址验证属性测�?
    - **Property 10: Wallet Address Validation**
    - **Validates: Requirements 1.2**

- [ ] 7. 实现认证中间�?
  - [ ] 7.1 创建 JWT 验证中间�?
    - 验证 Authorization header
    - 解析并验�?JWT Token
    - _Requirements: R1.3_
  - [ ] 7.2 创建角色授权中间�?
    - 实现 creator/developer/admin 角色检�?
    - _Requirements: 设计文档 Security Design_

- [ ] 8. Checkpoint - 认证系统验证
  - 确保注册、登录、Token 刷新流程正常
  - 验证中间件正确拦截未授权请求

## Phase 3: Agent 系统�?Proxy Gateway

- [ ] 9. 实现 Agent 构建服务
  - [ ] 9.1 创建 Agent CRUD 接口
    - 实现 POST/GET/PUT /api/v1/agents 接口
    - 实现 Agent 配置验证
    - _Requirements: R2.1, R2.3_
  - [ ] 9.2 编写 Agent ID 唯一性属性测�?
    - **Property 2: ID Uniqueness**
    - **Validates: Requirements 2.3, 4.2**
  - [ ] 9.3 实现 Agent 定价配置
    - 验证价格范围
    - 存储定价配置
    - _Requirements: R2.4_
  - [ ] 9.4 编写价格验证属性测�?
    - **Property 11: Price Validation**
    - **Validates: Requirements 2.4**
  - [ ] 9.5 实现 System Prompt 加密存储
    - 使用 AES-256-GCM 加密
    - 安全存储加密密钥
    - _Requirements: R10.1_
  - [ ] 9.6 编写加密往返属性测�?
    - **Property 19: Encryption Round-Trip**
    - **Validates: Requirements 10.1**

- [ ] 10. 实现 Agent 发布管理
  - [ ] 10.1 创建发布/下架接口
    - 实现 POST /api/v1/agents/:id/publish
    - 实现 POST /api/v1/agents/:id/unpublish
    - _Requirements: R3.1, R3.3_
  - [ ] 10.2 编写发布状态转换属性测�?
    - **Property 8: Publish State Transition**
    - **Validates: Requirements 3.1**
  - [ ] 10.3 实现 Agent 版本管理
    - 更新时创建新版本
    - 保留历史版本
    - _Requirements: R3.2_
  - [ ] 10.4 编写版本保留属性测�?
    - **Property 9: Version Preservation**
    - **Validates: Requirements 3.2**

- [ ] 11. 实现 API Key 管理
  - [ ] 11.1 创建 API Key CRUD 接口
    - 实现 POST/GET/DELETE /api/v1/developers/keys
    - 生成安全�?API Key
    - _Requirements: R4.2, R4.3, R4.4_
  - [ ] 11.2 编写 API Key 撤销属性测�?
    - **Property 12: API Key Revocation Immediacy**
    - **Validates: Requirements 4.3**

- [ ] 12. 实现 Proxy Gateway 核心
  - [ ] 12.1 创建 Proxy Gateway 服务
    - 实现 POST /proxy/v1/agents/:agentId/chat
    - 实现 API Key 验证
    - _Requirements: R5.1_
  - [ ] 12.2 实现配额检查和扣减
    - Redis 配额计数�?
    - 原子扣减操作
    - _Requirements: R5.5_
  - [ ] 12.3 编写配额一致性属性测�?
    - **Property 3: Quota Consistency**
    - **Validates: Requirements 5.5**
  - [ ] 12.4 实现 System Prompt 注入
    - 解密并注�?System Prompt
    - 确保 Prompt 不暴露在响应�?
    - _Requirements: R5.3, R10.2_
  - [ ] 12.5 编写 Prompt 安全属性测�?
    - **Property 1: Prompt Security (Critical)**
    - **Validates: Requirements 5.3, 10.2**
  - [ ] 12.6 实现 SSE 流式响应
    - 转发上游 SSE 事件
    - 处理流式错误
    - _Requirements: R5.4_

- [ ] 13. 实现限速和熔断
  - [ ] 13.1 实现 Rate Limiting
    - Redis 滑动窗口限�?
    - 区分免费/付费用户限制
    - _Requirements: R5.6_
  - [ ] 13.2 编写限速属性测�?
    - **Property 6: Rate Limiting Enforcement**
    - **Validates: Requirements 5.6**
  - [ ] 13.3 实现熔断�?
    - 使用 sony/gobreaker 实现熔断
    - 配置熔断阈�?
    - _Requirements: R18.2_
  - [ ] 13.4 编写熔断器属性测�?
    - **Property 26: Circuit Breaker Behavior**
    - **Validates: Requirements 18.2**
  - [ ] 13.5 实现请求超时
    - 配置 30 秒默认超�?
    - 返回 504 错误
    - _Requirements: R18.4_
  - [ ] 13.6 编写超时属性测�?
    - **Property 27: Timeout Enforcement**
    - **Validates: Requirements 18.4**

- [ ] 14. 实现错误处理
  - [ ] 14.1 实现标准化错误响�?
    - 定义错误码体�?
    - 实现错误响应格式
    - _Requirements: R18.1_
  - [ ] 14.2 编写错误响应标准化属性测�?
    - **Property 33: Error Response Standardization**
    - **Validates: Requirements 18.1**
  - [ ] 14.3 实现 Correlation ID
    - 生成请求追踪 ID
    - 在响应中返回
    - _Requirements: R18.6_
  - [ ] 14.4 编写 Correlation ID 属性测�?
    - **Property 34: Correlation ID Presence**
    - **Validates: Requirements 18.6**
  - [ ] 14.5 实现失败调用不扣�?
    - 上游错误时不扣减配额
    - _Requirements: R18.3_
  - [ ] 14.6 编写失败调用属性测�?
    - **Property 4: Failed Calls Don't Cost Quota**
    - **Validates: Requirements 18.3**

- [ ] 15. 实现草稿 Agent 访问控制
  - [ ] 15.1 拦截草稿 Agent API 调用
    - 检�?Agent 状�?
    - 返回适当错误
    - _Requirements: R2.5_
  - [ ] 15.2 编写草稿 Agent 属性测�?
    - **Property 7: Draft Agent Inaccessibility**
    - **Validates: Requirements 2.5**

- [ ] 16. Checkpoint - Proxy Gateway 验证
  - 确保 API 调用流程完整
  - 验证配额扣减正确
  - 验证限速和熔断工作正常

## Phase 4: 支付与商业化

- [ ] 17. 实现 Stripe 支付集成
  - [ ] 17.1 创建 Stripe Checkout 接口
    - 实现 POST /api/v1/payments/checkout
    - 创建 Checkout Session
    - _Requirements: R6.1_
  - [ ] 17.2 实现 Stripe Webhook 处理
    - 处理 checkout.session.completed 事件
    - 更新支付状�?
    - _Requirements: R6.1_
  - [ ] 17.3 实现支付成功配额增加
    - 确认支付后增加配�?
    - _Requirements: R6.3_
  - [ ] 17.4 编写支付配额一致性属性测�?
    - **Property 13: Payment-Quota Consistency**
    - **Validates: Requirements 6.3**
  - [ ] 17.5 实现支付失败处理
    - 失败时不增加配额
    - 发送通知
    - _Requirements: R6.4_
  - [ ] 17.6 编写支付失败属性测�?
    - **Property 14: Failed Payment No-Credit**
    - **Validates: Requirements 6.4**

- [ ] 18. 实现 Coinbase 加密支付
  - [ ] 18.1 创建 Coinbase Charge 接口
    - 生成支付地址
    - _Requirements: R6.2_
  - [ ] 18.2 实现 Coinbase Webhook 处理
    - 监听支付确认
    - 更新配额
    - _Requirements: R6.2_

- [ ] 19. 实现试用机制
  - [ ] 19.1 创建试用配额管理
    - 每个 Agent 3 次免费试�?
    - Redis 存储试用次数
    - _Requirements: R22.1, R22.2_
  - [ ] 19.2 编写试用配额属性测�?
    - **Property 25: Trial Quota Enforcement**
    - **Validates: Requirements 22.1**
  - [ ] 19.3 实现试用禁用选项
    - 创作者可禁用试用
    - _Requirements: R22.4_

- [ ] 20. 实现创作者提�?
  - [ ] 20.1 创建提现接口
    - 验证最低提现金�?
    - _Requirements: R20.1_
  - [ ] 20.2 编写提现阈值属性测�?
    - **Property 17: Withdrawal Threshold Enforcement**
    - **Validates: Requirements 20.1**
  - [ ] 20.3 实现提现费用计算
    - 扣除平台费用
    - _Requirements: R20.3_
  - [ ] 20.4 编写提现费用属性测�?
    - **Property 16: Withdrawal Fee Calculation**
    - **Validates: Requirements 20.3**
  - [ ] 20.5 实现提现失败恢复
    - 失败时返还余�?
    - _Requirements: R20.5_
  - [ ] 20.6 编写提现失败恢复属性测�?
    - **Property 32: Withdrawal Failure Recovery**
    - **Validates: Requirements 20.5**

- [ ] 21. 实现结算系统
  - [ ] 21.1 创建结算计算服务
    - 计算创作者收�?
    - 扣除平台分成
    - _Requirements: R8.1_
  - [ ] 21.2 编写结算计算属性测�?
    - **Property 15: Settlement Calculation Accuracy**
    - **Validates: Requirements 8.1**
  - [ ] 21.3 实现定时结算任务
    - 每日/每周结算
    - _Requirements: R8.1_

- [ ] 22. Checkpoint - 支付系统验证
  - 确保支付流程完整
  - 验证配额增减正确
  - 验证提现和结算正�?

## Phase 5: 高级功能

- [ ] 23. 实现 RAG 知识库系�?
  - [ ] 23.1 实现文件上传接口
    - 支持 PDF/Markdown 上传
    - 存储�?S3
    - _Requirements: R9.1, R9.2_
  - [ ] 23.2 实现文档处理服务
    - PDF 文本提取
    - Markdown 解析
    - 文本分块
    - _Requirements: R9.1, R9.2_
  - [ ] 23.3 实现向量化服�?
    - 生成 Embedding
    - 存储�?pgvector
    - _Requirements: R9.3_
  - [ ] 23.4 实现 RAG 检�?
    - 相似度搜�?
    - 上下文注�?
    - _Requirements: R9.4_
  - [ ] 23.5 编写 RAG 检索属性测�?
    - **Property 18: RAG Retrieval Relevance**
    - **Validates: Requirements 9.4**

- [ ] 24. 实现区块链集�?
  - [ ] 24.1 实现 ERC-1155 Token 铸�?
    - Agent 发布时铸�?Token
    - _Requirements: R7.1_
  - [ ] 24.2 实现 Token 元数�?
    - 包含 AgentID、创作者地址、时间戳
    - _Requirements: R7.2_
  - [ ] 24.3 编写 Token 元数据属性测�?
    - **Property 30: Token Metadata Completeness**
    - **Validates: Requirements 7.2**
  - [ ] 24.4 实现所有权查询
    - 查询链上所有权证明
    - _Requirements: R7.3_
  - [ ] 24.5 实现链上结算
    - 自动转账到创作者钱�?
    - _Requirements: R8.2_

- [ ] 25. 实现�?AI 模型支持
  - [ ] 25.1 创建统一 AI 接口
    - 抽象不同提供�?API
    - _Requirements: R19.3_
  - [ ] 25.2 实现 OpenAI 适配�?
    - GPT-4, GPT-3.5 支持
    - _Requirements: R19.1_
  - [ ] 25.3 实现 Anthropic 适配�?
    - Claude 3 支持
    - _Requirements: R19.1_
  - [ ] 25.4 实现 Google 适配�?
    - Gemini 支持
    - _Requirements: R19.1_

- [ ] 26. 实现 Webhook 系统
  - [ ] 26.1 创建 Webhook 配置接口
    - URL 验证
    - 事件订阅
    - _Requirements: R16.1_
  - [ ] 26.2 实现 Webhook 发送服�?
    - 异步发�?
    - 签名验证
    - _Requirements: R16.2, R16.3_
  - [ ] 26.3 实现重试机制
    - 指数退避重�?
    - 最�?3 �?
    - _Requirements: R16.4_
  - [ ] 26.4 编写 Webhook 重试属性测�?
    - **Property 24: Webhook Retry Logic**
    - **Validates: Requirements 16.4**
  - [ ] 26.5 实现 Webhook 日志
    - 记录发送历�?
    - _Requirements: R16.5_

- [ ] 27. Checkpoint - 高级功能验证
  - 确保 RAG 检索正�?
  - 验证区块链集�?
  - 验证 Webhook 发�?

## Phase 6: 商城与搜�?

- [ ] 28. 实现 Agent 商城
  - [ ] 28.1 创建商城搜索接口
    - 关键词搜�?
    - 相关性排�?
    - _Requirements: R14.2_
  - [ ] 28.2 实现筛选功�?
    - 分类、价格、评分筛�?
    - _Requirements: R14.3_
  - [ ] 28.3 编写搜索筛选属性测�?
    - **Property 20: Search Filter Accuracy**
    - **Validates: Requirements 14.3**
  - [ ] 28.4 实现推荐和热�?Agent
    - 特色 Agent 展示
    - 趋势排行
    - _Requirements: R14.1_

- [ ] 29. 实现评价系统
  - [ ] 29.1 创建评价提交接口
    - 验证调用次数 >= 10
    - _Requirements: R15.1_
  - [ ] 29.2 编写评价资格属性测�?
    - **Property 22: Review Eligibility Enforcement**
    - **Validates: Requirements 15.1**
  - [ ] 29.3 实现重复评价防止
    - 每个用户每个 Agent 只能评价一�?
    - _Requirements: R15.5_
  - [ ] 29.4 编写重复评价属性测�?
    - **Property 23: Duplicate Review Prevention**
    - **Validates: Requirements 15.5**
  - [ ] 29.5 实现评分计算
    - 计算平均评分
    - _Requirements: R15.3_
  - [ ] 29.6 编写评分计算属性测�?
    - **Property 21: Rating Calculation Accuracy**
    - **Validates: Requirements 15.3**

- [ ] 30. 实现分析仪表�?
  - [ ] 30.1 创建调用统计接口
    - �?�?月调用量
    - _Requirements: R17.1_
  - [ ] 30.2 编写分析数据属性测�?
    - **Property 31: Analytics Data Accuracy**
    - **Validates: Requirements 17.1**
  - [ ] 30.3 实现地理分布统计
    - 按国�?地区统计
    - _Requirements: R17.2_
  - [ ] 30.4 实现性能指标
    - 响应时间、错误率
    - _Requirements: R17.3_
  - [ ] 30.5 实现报表导出
    - CSV 格式导出
    - _Requirements: R17.5_

- [ ] 31. Checkpoint - 商城功能验证
  - 确保搜索和筛选正�?
  - 验证评价系统
  - 验证分析数据准确

## Phase 7: 前端实现

- [ ] 32. 实现 Landing Page
  - [ ] 32.1 创建 Hero Section
    - 价值主张展�?
    - CTA 按钮
    - _Requirements: R11.1_
  - [ ] 32.2 实现响应式布局
    - 移动端适配
    - 平板设备优化
    - _Requirements: R11.5, R24.3_
  - [ ] 32.3 实现暗色模式
    - 主题切换
    - 系统偏好检�?
    - _Requirements: R29_
  - [ ] 32.4 实现浏览器兼容�?
    - 配置 browserslist
    - 添加不支持浏览器警告
    - _Requirements: R24.1, R24.5_

- [ ] 33. 实现认证页面
  - [ ] 33.1 创建登录页面
    - 表单验证
    - 错误提示
    - _Requirements: R1.3_
  - [ ] 33.2 创建注册页面
    - 创作�?开发者选择
    - _Requirements: R1.1_

- [ ] 34. 实现创作者仪表盘
  - [ ] 34.1 创建概览页面
    - 收益图表
    - 统计卡片
    - _Requirements: R12.1_
  - [ ] 34.2 创建 Agent 列表页面
    - Agent 卡片展示
    - 状态管�?
    - _Requirements: R3.4_
  - [ ] 34.3 创建 Agent 构建�?
    - Prompt 编辑�?
    - 参数配置
    - 实时预览
    - _Requirements: R12.2_
  - [ ] 34.4 创建知识库上传界�?
    - 文件上传
    - 处理进度
    - _Requirements: R12.3_
  - [ ] 34.5 创建收益管理页面
    - 收益计算�?
    - 提现功能
    - _Requirements: R12.4_

- [ ] 35. 实现开发者控制台
  - [ ] 35.1 创建 API Key 管理页面
    - Key 列表
    - 创建/删除
    - _Requirements: R13.2_
  - [ ] 35.2 创建使用统计页面
    - 使用量图�?
    - 配额显示
    - _Requirements: R13.1_
  - [ ] 35.3 创建代码片段组件
    - 多语言支持
    - 一键复�?
    - _Requirements: R13.3_
  - [ ] 35.4 创建 Playground
    - 交互式测�?
    - 请求/响应查看
    - _Requirements: R13.4_
  - [ ] 35.5 创建账单页面
    - 支付历史
    - 发票下载
    - _Requirements: R13.5_

- [ ] 36. 实现商城页面
  - [ ] 36.1 创建搜索页面
    - 搜索�?
    - 筛选器
    - 结果列表
    - _Requirements: R11.3_
  - [ ] 36.2 创建 Agent 详情页面
    - Agent 信息
    - 评价列表
    - 试用按钮
    - _Requirements: R14.4_
  - [ ] 36.3 创建分类导航
    - 分类列表
    - 分类页面
    - _Requirements: R14.5_

- [ ] 37. 实现管理后台
  - [ ] 37.1 创建管理仪表�?
    - 平台统计
    - _Requirements: R21.1_
  - [ ] 37.2 创建用户管理页面
    - 用户列表
    - 状态管�?
    - _Requirements: R21.2_
  - [ ] 37.3 创建内容审核页面
    - 审核队列
    - 审批/拒绝
    - _Requirements: R21.3_

- [ ] 38. Checkpoint - 前端功能验证
  - 确保所有页面正常渲�?
  - 验证表单提交
  - 验证 API 集成

## Phase 8: API 版本管理与文�?

- [ ] 39. 实现 API 版本管理
  - [ ] 39.1 实现版本路由
    - /api/v1/ 路由结构
    - _Requirements: R23.1_
  - [ ] 39.2 实现多版本支�?
    - 同时支持 v1、v2
    - _Requirements: R23.3_
  - [ ] 39.3 编写多版本属性测�?
    - **Property 28: Multi-Version API Support**
    - **Validates: Requirements 23.3**
  - [ ] 39.4 实现废弃警告
    - 响应头添加废弃警�?
    - _Requirements: R23.4_
  - [ ] 39.5 编写废弃警告属性测�?
    - **Property 29: Deprecation Warning Headers**
    - **Validates: Requirements 23.4**

- [ ] 40. 实现 API 文档
  - [ ] 40.1 配置 Swagger/OpenAPI
    - 生成 API 文档
    - _Requirements: R11.7_
  - [ ] 40.2 创建交互式文档页�?
    - 代码示例
    - 在线测试
    - _Requirements: R11.7_

## Phase 9: 安全与优�?

- [ ] 41. 实现安全加固
  - [ ] 41.1 配置 CORS
    - 限制允许的域�?
    - _Requirements: R10, 设计文档 Security Design_
  - [ ] 41.2 实现请求签名验证
    - Webhook 签名
    - _Requirements: R16.1_
  - [ ] 41.3 实现敏感数据脱敏
    - 日志脱敏
    - _Requirements: R10.3_

- [ ] 42. 实现性能优化
  - [ ] 42.1 配置 CDN
    - 静态资�?CDN
    - _Requirements: R25.7_
  - [ ] 42.2 实现缓存预热
    - 热门 Agent 缓存
    - _Requirements: 设计文档 Caching Strategy_
  - [ ] 42.3 优化数据库查�?
    - 添加必要索引
    - 查询优化
    - _Requirements: R25_

- [ ] 43. 实现 SEO 优化
  - [ ] 43.1 配置 Meta 标签
    - 标题、描述、关键词
    - _Requirements: R26.1_
  - [ ] 43.2 实现 Open Graph
    - 社交分享预览
    - _Requirements: R26.2_
  - [ ] 43.3 生成 Sitemap
    - 自动生成 sitemap.xml
    - _Requirements: R26.3_

- [ ] 44. 实现可访问�?
  - [ ] 44.1 添加 ARIA 标签
    - 屏幕阅读器支�?
    - _Requirements: R27.3_
  - [ ] 44.2 实现键盘导航
    - Tab 导航
    - _Requirements: R27.2_
  - [ ] 44.3 确保色彩对比�?
    - WCAG 2.1 AA 合规
    - _Requirements: R27.4_

- [ ] 44.5 实现国际化准�?
  - [ ] 44.5.1 配置 next-intl
    - 安装和配置国际化�?
    - _Requirements: R28.2_
  - [ ] 44.5.2 外部化所有字符串
    - 创建翻译文件结构
    - 提取硬编码字符串
    - _Requirements: R28.1_
  - [ ] 44.5.3 实现 RTL 布局支持
    - CSS 逻辑属�?
    - _Requirements: R28.3_

- [ ] 45. Checkpoint - 安全与优化验�?
  - 运行安全扫描
  - 性能测试
  - 可访问性审�?

## Phase 10: 部署与发�?

- [ ] 46. 配置 CI/CD
  - [ ] 46.1 创建 GitHub Actions 工作�?
    - 代码检�?
    - 测试运行
    - 构建镜像
    - _Requirements: 设计文档 Git Workflow & CI/CD_
  - [ ] 46.2 配置自动部署
    - Staging 自动部署
    - Production 手动触发
    - _Requirements: 设计文档 Git Workflow & CI/CD_

- [ ] 47. 配置生产环境
  - [ ] 47.1 配置 Kubernetes
    - Deployment 配置
    - Service 配置
    - Ingress 配置
    - _Requirements: 设计文档 Deployment Architecture_
  - [ ] 47.2 配置自动扩缩�?
    - HPA 配置
    - _Requirements: 设计文档 Deployment Architecture_
  - [ ] 47.3 配置监控告警
    - Prometheus 规则
    - Grafana 仪表�?
    - _Requirements: 设计文档 Monitoring & Observability_

- [ ] 48. 最终验�?
  - 端到端测�?
  - 负载测试
  - 安全审计
  - 文档审查

## Notes

- 所有任务均为必须任务，包括属性测试
- 每个 Checkpoint 任务用于验证阶段性成果
- 所有任务完成后提交到 main 分支
- 遇到报错时创建 error-fix/* 分支，修复后合并
- 属性测试使用 Go rapid 库和 TypeScript fast-check 库
- 每个任务完成后提交到 GitHub 仓库

## 环境配置清单

### 必需的环境变量

```bash
# 数据库
DATABASE_URL=postgres://user:pass@localhost:5432/agentlink

# Redis
REDIS_URL=redis://localhost:6379

# JWT 密钥
JWT_SECRET=your-256-bit-secret

# 加密密钥
ENCRYPTION_KEY=your-32-byte-key

# AI 提供�?
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_AI_API_KEY=xxx

# 支付
STRIPE_SECRET_KEY=sk_test_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx
COINBASE_API_KEY=xxx

# 区块�?(可选，MVP 阶段可跳�?
BLOCKCHAIN_RPC_URL=https://base-sepolia.g.alchemy.com/v2/xxx
BLOCKCHAIN_PRIVATE_KEY=xxx

# 存储
S3_BUCKET=agentlink-files
S3_ENDPOINT=http://localhost:9000  # MinIO 本地开�?
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx

# 邮件服务
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USER=apikey
SMTP_PASSWORD=xxx
FROM_EMAIL=noreply@agentlink.io
```

### 本地开发服�?

| 服务 | 端口 | 说明 |
|------|------|------|
| PostgreSQL | 5432 | 主数据库 |
| Redis | 6379 | 缓存和限�?|
| MinIO | 9000/9001 | S3 兼容存储 |
| Mailhog | 1025/8025 | 邮件测试 |
| API Server | 8080 | Go 后端 |
| Proxy Gateway | 8081 | AI 代理 |
| Frontend | 3000 | Next.js |

### 开发工�?

- Go 1.22+
- Node.js 20+
- Docker & Docker Compose
- golangci-lint
- pnpm (推荐) �?npm

## Git 提交流程

### 正常开发流�?

```bash
# 1. �?develop 创建功能分支
git checkout develop
git pull origin develop
git checkout -b feature/task-name

# 2. 开发并提交
git add .
git commit -m "feat(scope): description"

# 3. 推送并创建 PR
git push origin feature/task-name
# 创建 PR �?develop

# 4. 合并�?main (功能完成�?
git checkout main
git merge develop
git push origin main
```

### 报错修复流程

```bash
# 1. 创建报错修复分支
git checkout -b error-fix/error-description

# 2. 修复并提�?
git add .
git commit -m "error(scope): fix error description

原因分析:
- 具体原因

解决方案:
- 修复步骤"

# 3. 推送并创建 PR
git push origin error-fix/error-description
# 创建 PR �?develop，包含详细的错误分析
```


## 需求覆盖检查表

| 需�?| 描述 | 任务覆盖 | 状�?|
|------|------|---------|------|
| R1 | 创作者账户管�?| 5.1, 5.3, 5.5, 6.1, 7.1, 33.1, 33.2 | �?|
| R2 | Agent 构建与配�?| 9.1, 9.3, 9.5, 15.1 | �?|
| R3 | Agent 发布与管�?| 10.1, 10.3, 34.2 | �?|
| R4 | 开发者账户与 API Key | 5.1, 11.1 | �?|
| R5 | API 代理网关 | 12.1, 12.2, 12.4, 12.6, 13.1 | �?|
| R6 | 支付与额度购�?| 17.1-17.5, 18.1-18.2 | �?|
| R7 | 区块链所有权存证 | 24.1, 24.2, 24.4 | �?|
| R8 | 区块链收益结�?| 21.1, 21.3, 24.5 | �?|
| R9 | 知识�?RAG 集成 | 23.1-23.4 | �?|
| R10 | 安全与隐私保�?| 9.5, 12.4, 41.1, 41.3 | �?|
| R11 | 前端用户界面设计 | 32.1, 32.2, 36.1, 40.1-40.2 | �?|
| R12 | 创作者工作台界面 | 34.1-34.5 | �?|
| R13 | 开发者控制台界面 | 35.1-35.5 | �?|
| R14 | Agent 发现与搜�?| 28.1, 28.2, 28.4, 36.2, 36.3 | �?|
| R15 | 评价与反馈系�?| 29.1, 29.3, 29.5 | �?|
| R16 | Webhook 通知机制 | 26.1-26.5 | �?|
| R17 | 调用分析与洞�?| 30.1, 30.3-30.5 | �?|
| R18 | 错误处理与服务降�?| 13.3, 13.5, 14.1, 14.3, 14.5 | �?|
| R19 | �?AI 模型支持 | 25.1-25.4 | �?|
| R20 | 创作者提现与收益管理 | 20.1, 20.3, 20.5 | �?|
| R21 | 平台管理后台 | 37.1-37.3 | �?|
| R22 | Agent 试用机制 | 19.1, 19.3 | �?|
| R23 | API 版本管理 | 39.1, 39.2, 39.4 | �?|
| R24 | 跨浏览器与平台适配 | 32.2, 32.4 | �?|
| R25 | 性能优化 | 42.1-42.3 | �?|
| R26 | SEO 与社交分享优�?| 43.1-43.3 | �?|
| R27 | 可访问�?| 44.1-44.3 | �?|
| R28 | 国际化准�?| 44.5.1-44.5.3 | �?|
| R29 | 暗色模式支持 | 32.3 | �?|

**所�?29 个需求均已覆�?�?*
