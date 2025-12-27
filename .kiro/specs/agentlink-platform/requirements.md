# Requirements Document

## Introduction

AgentLink 是一个 SaaS 平台，让 AI 创作者能够安全地将其 Prompt 和 AI Agent 变现，同时让开发者能够一键集成 AI 能力。平台通过 API 代理网关保护创作者的 Prompt 不被泄露，并使用区块链技术确保收益分成的透明度和资产所有权。

## Project Repository

- **GitHub**: https://github.com/aimerfeng/AgentLink.git
- **分支策略**: 遇到问题时创建分支，提交详细解决办法

## Technical Stack

| 层次 | 技术选型 | 理由 |
|------|----------|------|
| Frontend | Next.js 14 (App Router) + Tailwind CSS + Shadcn UI | SEO 友好，专业简洁的商城风格，响应式极佳 |
| Backend | Go (Gin) | 高并发处理 API 转发性能优越，部署轻量 |
| Database | PostgreSQL | 处理复杂的代理、订单、用户关联关系 |
| Caching | Redis | 实现 API 高效限速 (Rate Limiting) 和调用量缓存 |
| AI Integration | OpenAI API / LangChain | 快速集成多模型，管理 RAG 知识库 |
| Blockchain | Base (L2) / Polygon | 极低 Gas 费，用于处理创作者收益结算和资产存证 |

## Glossary

- **Agent**: 由创作者构建的 AI 助手，包含 System Prompt、参数配置和可选的知识库
- **Creator**: 在平台上创建和发布 Agent 的用户
- **Developer**: 通过 API 调用 Agent 的用户
- **Proxy_Gateway**: API 代理网关，负责身份验证、Prompt 注入和流式转发
- **AgentID**: 每个 Agent 的唯一标识符
- **API_Key**: 用户调用 API 时的身份凭证
- **Quota**: 用户购买的 API 调用额度
- **RAG**: 检索增强生成，用于知识库集成
- **Settlement_Contract**: 处理收益分成的智能合约

## Requirements

### Requirement 1: 创作者账户管理

**User Story:** As a Creator, I want to register and manage my account, so that I can create and monetize my AI Agents.

#### Acceptance Criteria

1. WHEN a creator registers with email and password, THE System SHALL create a new creator account and send verification email
2. WHEN a creator binds a wallet address, THE System SHALL validate the address format and store it for settlement
3. WHEN a creator logs in with valid credentials, THE System SHALL issue a session token and redirect to dashboard
4. IF a creator provides invalid credentials, THEN THE System SHALL return an authentication error without revealing which field is incorrect

### Requirement 2: Agent 构建与配置

**User Story:** As a Creator, I want to build and configure AI Agents with custom prompts and knowledge bases, so that I can offer unique AI capabilities.

#### Acceptance Criteria

1. WHEN a creator submits Agent configuration (System Prompt, temperature, max tokens), THE Agent_Builder SHALL validate parameters and create a new Agent record
2. WHEN a creator uploads knowledge base files (PDF/Markdown), THE System SHALL process and vectorize the content for RAG retrieval
3. WHEN an Agent is created, THE System SHALL generate a unique AgentID and proxy API endpoint
4. WHEN a creator sets pricing (USD per call), THE System SHALL validate the price range and store the pricing configuration
5. WHILE an Agent is in draft status, THE System SHALL prevent API calls to that Agent

### Requirement 3: Agent 发布与管理

**User Story:** As a Creator, I want to publish and manage my Agents, so that developers can discover and use them.

#### Acceptance Criteria

1. WHEN a creator publishes an Agent, THE System SHALL change status to active and make it available for API calls
2. WHEN a creator updates an Agent's configuration, THE System SHALL version the changes and maintain backward compatibility
3. WHEN a creator unpublishes an Agent, THE System SHALL reject new API calls while honoring in-flight requests
4. WHEN viewing the dashboard, THE Creator_Dashboard SHALL display call statistics, revenue, and active user count

### Requirement 4: 开发者账户与 API Key 管理

**User Story:** As a Developer, I want to manage my API keys and quotas, so that I can integrate AI Agents into my applications.

#### Acceptance Criteria

1. WHEN a developer registers, THE System SHALL create a developer account with initial free quota
2. WHEN a developer creates an API key, THE System SHALL generate a unique key with configurable permissions
3. WHEN a developer revokes an API key, THE System SHALL immediately invalidate the key and reject subsequent requests
4. THE System SHALL support multiple API keys per developer account

### Requirement 5: API 代理网关

**User Story:** As a Developer, I want to call Agent APIs securely, so that I can integrate AI capabilities without exposing the underlying prompts.

#### Acceptance Criteria

1. WHEN a request arrives with X-AgentLink-Key header, THE Proxy_Gateway SHALL validate the API key and check quota availability
2. IF the API key is invalid or quota is exhausted, THEN THE Proxy_Gateway SHALL return appropriate error response (401/429)
3. WHEN forwarding to upstream AI provider, THE Proxy_Gateway SHALL inject the creator's hidden System Prompt without exposing it in the response
4. WHEN the upstream supports streaming, THE Proxy_Gateway SHALL forward Server-Sent Events (SSE) to enable typewriter effect
5. WHEN a call completes successfully, THE Proxy_Gateway SHALL decrement quota counter and create consumption log
6. THE Proxy_Gateway SHALL enforce rate limits: 10 calls/minute for free users, 1000 calls/minute for paid users

### Requirement 6: 支付与额度购买

**User Story:** As a Developer, I want to purchase API call quotas, so that I can continue using the Agents I need.

#### Acceptance Criteria

1. WHEN a developer initiates fiat payment via Stripe, THE Payment_System SHALL create a checkout session and redirect to payment page
2. WHEN a developer initiates crypto payment via Coinbase Commerce, THE Payment_System SHALL generate payment address and monitor for confirmation
3. WHEN payment is confirmed, THE System SHALL credit the corresponding quota to the developer's account
4. WHEN a payment fails, THE System SHALL notify the developer and not credit any quota
5. THE System SHALL maintain complete payment and quota transaction history

### Requirement 7: 区块链所有权存证

**User Story:** As a Creator, I want my Agent ownership recorded on blockchain, so that I have verifiable proof of creation.

#### Acceptance Criteria

1. WHEN an Agent is first published, THE System SHALL mint an ERC-1155 token representing ownership on Base/Polygon
2. THE Token_Metadata SHALL include AgentID, creator address, and creation timestamp
3. WHEN queried, THE System SHALL return the on-chain ownership proof for any Agent

### Requirement 8: 区块链收益结算

**User Story:** As a Creator, I want transparent and automatic revenue settlement, so that I can trust the platform's fairness.

#### Acceptance Criteria

1. WHEN settlement period triggers (daily/weekly or by call threshold), THE Settlement_Contract SHALL calculate creator's share based on consumption logs
2. THE Settlement_Contract SHALL automatically transfer earnings to creator's bound wallet address
3. WHEN a creator queries settlement history, THE System SHALL return on-chain transaction records
4. THE System SHALL provide public audit interface showing total calls and distributions per Agent

### Requirement 9: 知识库 RAG 集成

**User Story:** As a Creator, I want to enhance my Agent with private knowledge bases, so that it can provide domain-specific answers.

#### Acceptance Criteria

1. WHEN a creator uploads PDF files, THE RAG_System SHALL extract text and split into chunks
2. WHEN a creator uploads Markdown files, THE RAG_System SHALL parse and preserve structure
3. THE RAG_System SHALL generate embeddings and store in vector database
4. WHEN an API call is made to an Agent with knowledge base, THE Proxy_Gateway SHALL retrieve relevant context and include it in the prompt
5. IF knowledge base processing fails, THEN THE System SHALL notify creator with error details

### Requirement 10: 安全与隐私保护

**User Story:** As a Platform Operator, I want to ensure security and privacy, so that creators' prompts are protected and users' data is safe.

#### Acceptance Criteria

1. THE System SHALL encrypt all stored System Prompts at rest using AES-256
2. THE Proxy_Gateway SHALL never include the original System Prompt in API responses
3. THE System SHALL log all API calls with sanitized request/response data (excluding sensitive content)
4. WHEN a security incident is detected, THE System SHALL alert administrators and optionally suspend affected accounts
5. THE System SHALL comply with data retention policies and support user data deletion requests


### Requirement 11: 前端用户界面设计

**User Story:** As a User, I want a professional and visually appealing interface, so that I can easily navigate and use the platform.

#### Acceptance Criteria

1. THE Landing_Page SHALL display a modern hero section with clear value proposition and call-to-action buttons
2. THE UI_System SHALL implement a consistent design system using Shadcn UI components with custom theming
3. WHEN displaying Agent marketplace, THE System SHALL show Agent cards with preview, pricing, rating, and creator info
4. THE Dashboard SHALL provide clean data visualization for statistics (charts, metrics cards)
5. WHEN on mobile devices, THE System SHALL render fully responsive layouts with touch-friendly interactions
6. THE System SHALL implement smooth page transitions and loading states for professional feel
7. WHEN displaying API documentation, THE System SHALL provide interactive code examples with syntax highlighting

### Requirement 12: 创作者工作台界面

**User Story:** As a Creator, I want an intuitive dashboard to manage my Agents, so that I can efficiently build and monitor my AI products.

#### Acceptance Criteria

1. THE Creator_Dashboard SHALL display revenue overview with daily/weekly/monthly charts
2. WHEN building an Agent, THE Agent_Builder SHALL provide a live preview panel showing prompt behavior
3. THE Knowledge_Upload_UI SHALL show upload progress and processing status with visual feedback
4. WHEN configuring pricing, THE System SHALL display estimated earnings calculator
5. THE Dashboard SHALL provide quick-access cards for recent Agents and pending actions

### Requirement 13: 开发者控制台界面

**User Story:** As a Developer, I want a clean developer console, so that I can easily manage API keys and monitor usage.

#### Acceptance Criteria

1. THE Developer_Console SHALL display API usage graphs and quota remaining prominently
2. WHEN viewing API keys, THE System SHALL show key status, creation date, and last used timestamp
3. THE System SHALL provide one-click code snippets for popular languages (JavaScript, Python, Go, cURL)
4. WHEN testing an Agent, THE Playground SHALL provide an interactive chat interface with request/response inspector
5. THE Billing_Page SHALL display clear pricing tiers and payment history with invoice download


### Requirement 14: Agent 发现与搜索

**User Story:** As a Developer, I want to discover and search for Agents, so that I can find the right AI capabilities for my needs.

#### Acceptance Criteria

1. WHEN a developer visits the marketplace, THE System SHALL display featured and trending Agents
2. WHEN a developer searches by keyword, THE Search_System SHALL return relevant Agents ranked by relevance score
3. THE System SHALL support filtering by category, price range, rating, and creator
4. WHEN displaying search results, THE System SHALL show Agent preview, pricing, call count, and average rating
5. THE System SHALL provide category navigation (Writing, Coding, Analysis, Creative, etc.)

### Requirement 15: 评价与反馈系统

**User Story:** As a Developer, I want to rate and review Agents, so that I can help others make informed decisions.

#### Acceptance Criteria

1. WHEN a developer has used an Agent at least 10 times, THE System SHALL allow them to submit a rating (1-5 stars) and review
2. WHEN a review is submitted, THE System SHALL display it on the Agent detail page after moderation
3. THE System SHALL calculate and display average rating and total review count for each Agent
4. WHEN a creator receives a review, THE System SHALL notify them via email and dashboard
5. THE System SHALL prevent duplicate reviews from the same developer for the same Agent

### Requirement 16: Webhook 通知机制

**User Story:** As a Developer, I want to receive webhook notifications, so that I can automate my workflows based on platform events.

#### Acceptance Criteria

1. WHEN a developer configures a webhook endpoint, THE System SHALL validate the URL and store the configuration
2. WHEN quota falls below threshold, THE System SHALL send webhook notification to configured endpoints
3. WHEN an API call completes, THE System SHALL optionally send webhook with call metadata (if enabled)
4. IF webhook delivery fails, THEN THE System SHALL retry with exponential backoff up to 3 times
5. THE System SHALL provide webhook delivery logs for debugging

### Requirement 17: 调用分析与洞察

**User Story:** As a Creator, I want to analyze how developers use my Agents, so that I can improve and optimize them.

#### Acceptance Criteria

1. THE Analytics_Dashboard SHALL display daily/weekly/monthly call volume trends
2. THE System SHALL show geographic distribution of API calls (by country/region)
3. THE System SHALL display average response time and error rate metrics
4. WHEN viewing analytics, THE Creator SHALL see top use cases based on input patterns (anonymized)
5. THE System SHALL provide exportable reports in CSV format


### Requirement 18: 错误处理与服务降级

**User Story:** As a Platform Operator, I want robust error handling and graceful degradation, so that the platform remains reliable under adverse conditions.

#### Acceptance Criteria

1. IF upstream AI provider returns an error, THEN THE Proxy_Gateway SHALL return a standardized error response with error code and message
2. IF upstream AI provider is unavailable, THEN THE System SHALL implement circuit breaker pattern and return 503 with retry-after header
3. WHEN an API call fails due to upstream issues, THE System SHALL NOT decrement the developer's quota
4. THE System SHALL implement request timeout (30 seconds default) and return 504 if exceeded
5. WHEN errors exceed threshold, THE System SHALL alert administrators via configured channels (email, Slack, PagerDuty)
6. THE System SHALL maintain error logs with correlation IDs for debugging

### Requirement 19: 多 AI 模型支持

**User Story:** As a Creator, I want to choose from multiple AI models, so that I can select the best model for my Agent's use case.

#### Acceptance Criteria

1. THE System SHALL support multiple AI providers (OpenAI, Anthropic Claude, Google Gemini, etc.)
2. WHEN a creator configures an Agent, THE Agent_Builder SHALL allow selection of AI model and provider
3. THE System SHALL abstract provider-specific APIs behind a unified interface
4. WHEN a provider is added or updated, THE System SHALL require only configuration changes without code modifications
5. THE System SHALL display model capabilities, pricing, and rate limits for creator reference

### Requirement 20: 创作者提现与收益管理

**User Story:** As a Creator, I want to withdraw my earnings, so that I can monetize my work effectively.

#### Acceptance Criteria

1. WHEN a creator requests withdrawal, THE System SHALL validate minimum threshold (e.g., $50 USD) is met
2. THE System SHALL support withdrawal to bound wallet address (crypto) or bank account (fiat via Stripe Connect)
3. WHEN withdrawal is processed, THE System SHALL deduct platform fee (e.g., 20%) and transfer remaining amount
4. THE System SHALL display pending, processing, and completed withdrawal history
5. IF withdrawal fails, THEN THE System SHALL notify creator and return funds to available balance

### Requirement 21: 平台管理后台

**User Story:** As a Platform Administrator, I want a management dashboard, so that I can monitor and manage the platform effectively.

#### Acceptance Criteria

1. THE Admin_Dashboard SHALL display platform-wide metrics (total users, Agents, API calls, revenue)
2. WHEN an administrator searches for users, THE System SHALL return user details with account status and activity
3. THE Admin_Dashboard SHALL provide content moderation queue for flagged Agents and reviews
4. WHEN an administrator suspends an account, THE System SHALL immediately disable all associated API keys and Agents
5. THE System SHALL provide system health monitoring (API latency, error rates, queue depths)
6. THE Admin_Dashboard SHALL display financial reports (revenue, payouts, pending settlements)

### Requirement 22: Agent 试用机制

**User Story:** As a Developer, I want to try Agents before purchasing, so that I can evaluate if they meet my needs.

#### Acceptance Criteria

1. THE System SHALL provide 3 free trial calls per Agent per developer account
2. WHEN a developer uses trial calls, THE System SHALL clearly indicate remaining trial quota
3. WHEN trial quota is exhausted, THE System SHALL prompt developer to purchase quota
4. THE Creator SHALL have option to disable trial for their Agents
5. THE System SHALL track trial-to-paid conversion metrics for creators

### Requirement 23: API 版本管理

**User Story:** As a Developer, I want stable API versions, so that my integrations don't break unexpectedly.

#### Acceptance Criteria

1. THE API SHALL include version in URL path (e.g., /v1/agents/{id}/chat)
2. WHEN a breaking change is introduced, THE System SHALL release a new API version
3. THE System SHALL support at least 2 major versions concurrently with 6-month deprecation notice
4. WHEN calling deprecated endpoints, THE System SHALL return deprecation warning in response headers
5. THE API_Documentation SHALL clearly indicate version differences and migration guides


### Requirement 24: 跨浏览器与平台适配

**User Story:** As a User, I want the platform to work seamlessly across different browsers and devices, so that I can access it from anywhere.

#### Acceptance Criteria

1. THE System SHALL support modern browsers: Chrome (latest 2 versions), Firefox (latest 2 versions), Safari (latest 2 versions), Edge (latest 2 versions)
2. THE System SHALL NOT support Internet Explorer
3. WHEN accessed on tablet devices, THE System SHALL render optimized layouts with appropriate touch targets
4. THE System SHALL implement progressive enhancement for older browser versions
5. THE System SHALL display browser compatibility warning for unsupported browsers

### Requirement 25: 性能优化

**User Story:** As a User, I want fast page loads and responsive interactions, so that I can work efficiently on the platform.

#### Acceptance Criteria

1. THE Landing_Page SHALL achieve Lighthouse performance score of 90+ on desktop
2. THE System SHALL implement code splitting and lazy loading for non-critical components
3. THE System SHALL use Next.js Image optimization for all images
4. WHEN loading dashboard data, THE System SHALL display skeleton loaders within 100ms
5. THE System SHALL implement service worker for offline-capable static assets
6. THE API_Gateway SHALL respond within 200ms for non-AI endpoints (p95)
7. THE System SHALL use CDN for static assets delivery

### Requirement 26: SEO 与社交分享优化

**User Story:** As a Platform Operator, I want the platform to be discoverable and shareable, so that we can attract organic traffic.

#### Acceptance Criteria

1. THE System SHALL implement proper meta tags (title, description, keywords) for all public pages
2. WHEN sharing an Agent page, THE System SHALL provide Open Graph and Twitter Card meta tags with preview image
3. THE System SHALL generate sitemap.xml and robots.txt automatically
4. THE Agent_Detail_Page SHALL use semantic HTML and structured data (JSON-LD) for search engines
5. THE System SHALL implement canonical URLs to prevent duplicate content issues
6. THE Landing_Page SHALL be server-side rendered for optimal SEO

### Requirement 27: 可访问性 (Accessibility)

**User Story:** As a User with disabilities, I want the platform to be accessible, so that I can use it effectively.

#### Acceptance Criteria

1. THE System SHALL achieve WCAG 2.1 Level AA compliance
2. THE System SHALL support keyboard navigation for all interactive elements
3. THE System SHALL provide proper ARIA labels for screen readers
4. THE System SHALL maintain minimum color contrast ratio of 4.5:1 for text
5. WHEN displaying form errors, THE System SHALL announce them to screen readers
6. THE System SHALL support browser zoom up to 200% without horizontal scrolling

### Requirement 28: 国际化准备 (i18n Ready)

**User Story:** As a Platform Operator, I want the codebase to be i18n-ready, so that we can easily add language support in the future.

#### Acceptance Criteria

1. THE System SHALL externalize all user-facing strings into translation files
2. THE System SHALL use next-intl or similar library for internationalization infrastructure
3. THE System SHALL support RTL (right-to-left) layout structure in CSS
4. THE System SHALL store user language preference in account settings
5. THE System SHALL initially support English (en) as the default language
6. WHEN adding new UI text, THE Developer SHALL add it to translation files rather than hardcoding

### Requirement 29: 暗色模式支持

**User Story:** As a User, I want to use dark mode, so that I can reduce eye strain and save battery on OLED screens.

#### Acceptance Criteria

1. THE System SHALL support light and dark color themes
2. THE System SHALL detect and respect system color scheme preference by default
3. WHEN a user toggles theme, THE System SHALL persist the preference in local storage
4. THE System SHALL implement smooth theme transition animations
5. THE System SHALL ensure all components (including charts and code blocks) adapt to the selected theme
