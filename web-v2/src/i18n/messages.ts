export type Locale = 'en' | 'zh';

export const messages = {
  en: {
    navBrand: 'AI Gateway Control Plane',
    navDocs: 'Documentation',
    navSignIn: 'Sign in',
    navConsole: 'Open console',
    localeLabel: '中文',
    heroBadge: 'OpenAI-compatible AI gateway',
    heroTitle: 'Ship one AI integration. Route across many model providers.',
    heroDescription:
      'Bring GPT, Claude, Gemini, and more behind one operational gateway. Keep your backend contracts, upgrade the product surface, and migrate clients by changing the base URL.',
    heroPrimary: 'Start with the console',
    heroSecondary: 'Read the docs',
    heroVersion: 'Backend version',
    baseUrlLabel: 'Base URL for migration',
    baseUrlHint:
      'Keep the OpenAI SDK shape. Replace the base URL, issue a new key, and move traffic without a rewrite.',
    metricUptime: 'Gateway uptime target',
    metricLatency: 'Added latency',
    metricModels: 'Provider adapters',
    metricMigration: 'Migration path',
    metricsMigrationValue: '5 min',
    metricsMigrationLabel: 'Client migration',
    supportTitle: 'Works with the providers your customers already expect',
    supportSubtitle:
      'A single gateway surface for text, multimodal, and operational policy control.',
    supportPillOne: 'Multi-provider routing',
    supportPillTwo: 'Text and chat workloads',
    supportPillThree: 'Unified operational surface',
    supportPillFour: 'Policy and access controls',
    whyEyebrow: 'Why web-v2',
    whyTitle: 'Built as a product surface, not a patched admin panel',
    whyDescription:
      'This redesign focuses on operator clarity, strong hierarchy, and a customer-facing story that works for an English-first market.',
    featureOneTitle: 'One gateway, many upstreams',
    featureOneDescription:
      'Route OpenAI, Claude, Gemini, Azure, Bedrock, and more through one consistent operator experience.',
    featureTwoTitle: 'Faster operating rhythm',
    featureTwoDescription:
      'Give internal teams a cleaner control center for tokens, channels, quotas, and model routing.',
    featureThreeTitle: 'Governance without friction',
    featureThreeDescription:
      'Bring quotas, restrictions, and billing rules forward without changing backend logic.',
    integrationEyebrow: 'Integration',
    integrationTitle: 'Drop-in replacement for existing clients',
    integrationDescription:
      'Use your current SDKs and HTTP clients. Keep the same request shape and move traffic through a cleaner gateway surface.',
    integrationBulletOne: 'OpenAI-compatible request format',
    integrationBulletTwo: 'Unified routing for multiple providers',
    integrationBulletThree: 'Operational visibility across tokens and channels',
    integrationBulletFour: 'Safer rollout path for teams and customers',
    panelTitle: 'Operator preview',
    panelHealthy: 'Healthy',
    panelOneTitle: 'Gateway routing',
    panelOneDescription:
      'Stable routing across multiple upstreams with the product layer reworked for faster operations.',
    panelTwoTitle: 'Token issuance',
    panelTwoDescription:
      'Issue scoped credentials, separate environments, and present cleaner access surfaces to teams.',
    panelThreeTitle: 'Usage visibility',
    panelThreeDescription:
      'Summaries, quotas, and trends are moving into a more premium console experience.',
    ctaTitle: 'Ready to modernize the frontend layer?',
    ctaDescription:
      'Keep the backend. Upgrade the customer-facing story and the operator experience.',
    ctaPrimary: 'Enter web-v2',
    ctaSecondary: 'Sign in',
  },
  zh: {
    navBrand: 'AI 网关控制台',
    navDocs: '文档',
    navSignIn: '登录',
    navConsole: '打开控制台',
    localeLabel: 'EN',
    heroBadge: 'OpenAI 兼容 AI 网关',
    heroTitle: '一次接入 AI，统一路由多个模型供应商。',
    heroDescription:
      '把 GPT、Claude、Gemini 等模型收敛到一个运营网关后面。保留现有后端契约，升级产品界面，只需改 Base URL 即可迁移客户端。',
    heroPrimary: '进入控制台',
    heroSecondary: '查看文档',
    heroVersion: '后端版本',
    baseUrlLabel: '迁移用 Base URL',
    baseUrlHint:
      '保留 OpenAI SDK 调用方式。替换 Base URL、签发新密钥，即可在不重写代码的前提下迁移流量。',
    metricUptime: '网关可用性目标',
    metricLatency: '新增延迟',
    metricModels: '供应商适配器',
    metricMigration: '迁移路径',
    metricsMigrationValue: '5 分钟',
    metricsMigrationLabel: '客户端迁移',
    supportTitle: '兼容客户已经在用的主流模型供应商',
    supportSubtitle: '统一的文本、多模态与运营策略控制网关。',
    supportPillOne: '多供应商统一路由',
    supportPillTwo: '文本与对话工作负载',
    supportPillThree: '统一运营操作界面',
    supportPillFour: '策略与访问控制',
    whyEyebrow: '为什么是 web-v2',
    whyTitle: '这是一个产品界面，不是旧后台的修补版',
    whyDescription:
      '这次改版重点是更清晰的运营层级、更强的信息表达，以及更适合英文优先市场的产品叙事。',
    featureOneTitle: '一个网关，多个上游',
    featureOneDescription:
      '通过一致的运营体验统一接入 OpenAI、Claude、Gemini、Azure、Bedrock 等供应商。',
    featureTwoTitle: '更快的运营节奏',
    featureTwoDescription:
      '给内部团队一个更清晰的控制中心，统一处理令牌、渠道、额度和模型路由。',
    featureThreeTitle: '治理能力前移',
    featureThreeDescription:
      '在不改后端逻辑的前提下，把配额、限制和计费规则前移到更好的产品界面。',
    integrationEyebrow: '接入方式',
    integrationTitle: '可直接替换现有客户端',
    integrationDescription:
      '继续使用现有 SDK 和 HTTP 客户端。保持请求格式不变，通过更清晰的网关层迁移流量。',
    integrationBulletOne: 'OpenAI 兼容请求格式',
    integrationBulletTwo: '多个供应商的统一路由',
    integrationBulletThree: '跨令牌与渠道的运营可见性',
    integrationBulletFour: '适合团队和客户的更安全迁移路径',
    panelTitle: '运营预览',
    panelHealthy: '运行正常',
    panelOneTitle: '网关路由',
    panelOneDescription:
      '多上游稳定路由，同时把产品层改造成更适合高频运营的前端。',
    panelTwoTitle: '令牌签发',
    panelTwoDescription:
      '支持范围化凭证、环境隔离，并为团队提供更清晰的访问界面。',
    panelThreeTitle: '使用可视化',
    panelThreeDescription:
      '摘要、额度和趋势正在迁移到更高级的控制台体验中。',
    ctaTitle: '准备升级前端产品层了吗？',
    ctaDescription: '保留后端，实现更好的客户展示和运营体验。',
    ctaPrimary: '进入 web-v2',
    ctaSecondary: '登录',
  },
} as const;

export type MessageKey = keyof typeof messages.en;
