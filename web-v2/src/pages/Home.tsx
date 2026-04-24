import {
  ArrowRight,
  Bot,
  Gauge,
  Globe2,
  Languages,
  Layers3,
  ShieldCheck,
  Sparkles,
  Workflow,
} from 'lucide-react';
import { useStatus } from '../hooks/useStatus';
import { useI18n } from '../i18n/I18nProvider';
import type { MessageKey } from '../i18n/messages';

const metrics: Array<{ value: string; labelKey: MessageKey }> = [
  { value: '99.99%', labelKey: 'metricUptime' },
  { value: '< 50ms', labelKey: 'metricLatency' },
  { value: '40+', labelKey: 'metricModels' },
  { value: '5 min', labelKey: 'metricMigration' },
];

const featureCards: Array<{
  icon: typeof Layers3;
  titleKey: MessageKey;
  descriptionKey: MessageKey;
}> = [
  {
    icon: Layers3,
    titleKey: 'featureOneTitle',
    descriptionKey: 'featureOneDescription',
  },
  {
    icon: Gauge,
    titleKey: 'featureTwoTitle',
    descriptionKey: 'featureTwoDescription',
  },
  {
    icon: ShieldCheck,
    titleKey: 'featureThreeTitle',
    descriptionKey: 'featureThreeDescription',
  },
];

const supportPills: Array<{ labelKey: MessageKey; icon: typeof Globe2 }> = [
  { labelKey: 'supportPillOne', icon: Globe2 },
  { labelKey: 'supportPillTwo', icon: Bot },
  { labelKey: 'supportPillThree', icon: Layers3 },
  { labelKey: 'supportPillFour', icon: ShieldCheck },
];

export default function Home() {
  const status = useStatus();
  const { locale, setLocale, t } = useI18n();

  const docsLink = status.data?.docs_link;
  const version = status.data?.version;
  const systemName = status.data?.system_name || 'new-api';
  const baseUrl = status.data?.server_address || 'https://your-gateway.example.com';

  return (
    <div className='min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top,_rgba(13,148,136,0.2),_transparent_34%),linear-gradient(180deg,_#f4fbfa_0%,_#ffffff_44%,_#f5f7fb_100%)] text-slate-950 selection:bg-teal-200/70'>
      <div className='pointer-events-none absolute inset-0 z-0 overflow-hidden'>
        <div className='absolute left-[-8%] top-[-12%] h-[32rem] w-[32rem] rounded-full bg-teal-300/20 blur-[120px]' />
        <div className='absolute right-[-10%] top-[12%] h-[28rem] w-[28rem] rounded-full bg-cyan-200/30 blur-[120px]' />
        <div className='absolute bottom-[-12%] left-[28%] h-[24rem] w-[24rem] rounded-full bg-emerald-200/30 blur-[120px]' />
      </div>

      <header className='relative z-10 mx-auto flex max-w-7xl items-center justify-between px-6 py-6 lg:px-10'>
        <div className='flex items-center gap-3'>
          <div className='flex h-11 w-11 items-center justify-center rounded-2xl bg-slate-950 text-white shadow-[0_16px_36px_-22px_rgba(15,23,42,0.7)]'>
            <Workflow className='h-5 w-5' />
          </div>
          <div>
            <p className='text-xs font-bold uppercase tracking-[0.24em] text-slate-500'>
              QuantumNous
            </p>
            <p className='text-lg font-semibold text-slate-950'>
              {systemName} · {t('navBrand')}
            </p>
          </div>
        </div>

        <nav className='hidden items-center gap-3 md:flex'>
          <button
            type='button'
            onClick={() => setLocale(locale === 'en' ? 'zh' : 'en')}
            className='inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition-colors hover:bg-slate-50'
          >
            <Languages className='h-4 w-4' />
            {t('localeLabel')}
          </button>
          {docsLink ? (
            <a
              href={docsLink}
              target='_blank'
              rel='noreferrer'
              className='text-sm font-medium text-slate-600 transition-colors hover:text-slate-950'
            >
              {t('navDocs')}
            </a>
          ) : null}
          <a
            href='/login'
            className='text-sm font-medium text-slate-600 transition-colors hover:text-slate-950'
          >
            {t('navSignIn')}
          </a>
          <a
            href='/console'
            className='inline-flex items-center justify-center rounded-full bg-slate-950 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-slate-800'
          >
            {t('navConsole')}
          </a>
        </nav>
      </header>

      <main className='relative z-10 mx-auto max-w-7xl px-6 pb-24 pt-8 lg:px-10 lg:pb-28 lg:pt-12'>
        <section className='grid gap-12 lg:grid-cols-[1.05fr_0.95fr] lg:items-center'>
          <div className='space-y-8'>
            <div className='inline-flex items-center gap-2 rounded-full border border-teal-300 bg-white/90 px-4 py-2 text-sm font-medium text-teal-700 shadow-sm backdrop-blur'>
              <Sparkles className='h-4 w-4' />
              {t('heroBadge')}
            </div>

            <div className='space-y-6'>
              <h1 className='max-w-4xl text-5xl font-semibold tracking-[-0.05em] text-slate-950 sm:text-6xl lg:text-7xl lg:leading-[1.02]'>
                {t('heroTitle')}
              </h1>
              <p className='max-w-2xl text-lg leading-8 text-slate-600 lg:text-xl'>
                {t('heroDescription')}
              </p>
              {version ? (
                <p className='text-sm text-slate-500'>
                  {t('heroVersion')}: {version}
                </p>
              ) : null}
            </div>

            <div className='flex flex-wrap items-center gap-4'>
              <a
                href='/console'
                className='inline-flex items-center justify-center rounded-full bg-slate-950 px-6 py-3.5 text-base font-semibold text-white shadow-[0_20px_40px_-24px_rgba(15,23,42,0.7)] transition-all hover:-translate-y-0.5 hover:bg-slate-800'
              >
                {t('heroPrimary')}
                <ArrowRight className='ml-2 h-5 w-5' />
              </a>
              {docsLink ? (
                <a
                  href={docsLink}
                  target='_blank'
                  rel='noreferrer'
                  className='inline-flex items-center justify-center rounded-full border border-slate-200 bg-white px-6 py-3.5 text-base font-semibold text-slate-700 shadow-sm transition-all hover:bg-slate-50'
                >
                  {t('heroSecondary')}
                </a>
              ) : null}
            </div>

            <div className='rounded-[32px] border border-white/80 bg-white/82 p-5 shadow-[0_32px_80px_-44px_rgba(15,23,42,0.35)] backdrop-blur'>
              <p className='text-xs font-medium uppercase tracking-[0.24em] text-slate-500'>
                {t('baseUrlLabel')}
              </p>
              <div className='mt-4 rounded-[24px] border border-slate-200 bg-slate-950 px-5 py-4 font-mono text-sm text-teal-300'>
                {baseUrl}
              </div>
              <p className='mt-3 max-w-2xl text-sm leading-7 text-slate-600'>
                {t('baseUrlHint')}
              </p>
            </div>

            <div className='grid grid-cols-2 gap-4 sm:grid-cols-4'>
              {metrics.map((metric) => (
                <div
                  key={metric.labelKey}
                  className='rounded-[24px] border border-white/80 bg-white/78 px-5 py-5 shadow-[0_20px_60px_-38px_rgba(15,23,42,0.35)] backdrop-blur'
                >
                  <p className='text-3xl font-semibold text-slate-950'>{metric.value}</p>
                  <p className='mt-2 text-sm leading-6 text-slate-600'>
                    {t(metric.labelKey)}
                  </p>
                </div>
              ))}
            </div>
          </div>

          <div className='relative'>
            <div className='absolute inset-0 -z-10 rounded-[40px] bg-gradient-to-tr from-teal-300/30 to-cyan-200/30 blur-3xl' />
            <div className='rounded-[36px] border border-white/80 bg-[linear-gradient(180deg,_rgba(255,255,255,0.95),_rgba(247,250,252,0.98))] p-6 shadow-[0_36px_100px_-54px_rgba(15,23,42,0.45)] backdrop-blur-xl lg:p-8'>
              <div className='mb-6 flex items-center justify-between border-b border-slate-200 pb-4'>
                <div className='flex items-center gap-2'>
                  <div className='flex gap-1.5'>
                    <div className='h-3 w-3 rounded-full bg-red-400/80' />
                    <div className='h-3 w-3 rounded-full bg-amber-400/80' />
                    <div className='h-3 w-3 rounded-full bg-emerald-400/80' />
                  </div>
                  <p className='ml-2 text-sm font-mono text-slate-500'>
                    gateway-telemetry.ts
                  </p>
                </div>
                <span className='inline-flex items-center gap-1.5 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700'>
                  <span className='relative flex h-2 w-2'>
                    <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75' />
                <span className='relative inline-flex h-2 w-2 rounded-full bg-emerald-500' />
                  </span>
                  {t('panelHealthy')}
                </span>
              </div>

              <div className='space-y-4'>
                {([
                  { titleKey: 'panelOneTitle', descriptionKey: 'panelOneDescription' },
                  { titleKey: 'panelTwoTitle', descriptionKey: 'panelTwoDescription' },
                  { titleKey: 'panelThreeTitle', descriptionKey: 'panelThreeDescription' },
                ] as Array<{
                  titleKey: MessageKey;
                  descriptionKey: MessageKey;
                }>).map((item) => (
                  <div
                    key={item.titleKey}
                    className='group rounded-[24px] border border-slate-200 bg-white p-5 transition-all hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-sm'
                  >
                    <div className='flex items-center justify-between'>
                      <p className='font-semibold text-slate-900'>{t(item.titleKey)}</p>
                      <ArrowRight className='h-4 w-4 text-slate-400 transition-transform group-hover:translate-x-1 group-hover:text-teal-600' />
                    </div>
                    <p className='mt-2 text-sm leading-7 text-slate-600'>
                      {t(item.descriptionKey)}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className='mt-28 space-y-12'>
          <div className='max-w-3xl space-y-4'>
            <p className='text-xs font-medium uppercase tracking-[0.24em] text-teal-700'>
              {t('supportTitle')}
            </p>
            <p className='text-lg leading-8 text-slate-600'>
              {t('supportSubtitle')}
            </p>
          </div>
          <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
            {supportPills.map((item) => (
              <div
                key={item.labelKey}
                className='flex items-center gap-3 rounded-[24px] border border-white/80 bg-white/82 px-5 py-4 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.35)]'
              >
                <div className='flex h-11 w-11 items-center justify-center rounded-2xl bg-teal-50 text-teal-700 ring-1 ring-teal-100'>
                  <item.icon className='h-5 w-5' />
                </div>
                <p className='text-sm font-medium text-slate-700'>{t(item.labelKey)}</p>
              </div>
            ))}
          </div>
        </section>

        <section className='mt-28 space-y-12'>
          <div className='max-w-3xl space-y-4'>
            <p className='text-xs font-medium uppercase tracking-[0.24em] text-teal-700'>
              {t('whyEyebrow')}
            </p>
            <h2 className='text-3xl font-semibold tracking-tight text-slate-950 lg:text-4xl'>
              {t('whyTitle')}
            </h2>
            <p className='text-lg leading-8 text-slate-600'>{t('whyDescription')}</p>
          </div>

          <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-3'>
            {featureCards.map((card) => (
              <article
                key={card.titleKey}
                className='rounded-[28px] border border-white/80 bg-white/82 p-8 shadow-[0_24px_80px_-42px_rgba(15,23,42,0.35)] transition-transform hover:-translate-y-1'
              >
                <div className='inline-flex items-center justify-center rounded-2xl bg-teal-50 p-3 ring-1 ring-teal-100'>
                  <card.icon className='h-6 w-6 text-teal-700' />
                </div>
                <h3 className='mt-6 text-xl font-semibold text-slate-950'>
                  {t(card.titleKey)}
                </h3>
                <p className='mt-3 text-sm leading-7 text-slate-600'>
                  {t(card.descriptionKey)}
                </p>
              </article>
            ))}
          </div>
        </section>

        <section className='mt-28 grid gap-10 rounded-[36px] border border-white/80 bg-white/82 p-8 shadow-[0_32px_100px_-50px_rgba(15,23,42,0.38)] lg:grid-cols-[0.95fr_1.05fr] lg:p-10'>
          <div className='space-y-5'>
            <p className='text-xs font-medium uppercase tracking-[0.24em] text-teal-700'>
              {t('integrationEyebrow')}
            </p>
            <h2 className='text-3xl font-semibold tracking-tight text-slate-950 lg:text-4xl'>
              {t('integrationTitle')}
            </h2>
            <p className='text-lg leading-8 text-slate-600'>
              {t('integrationDescription')}
            </p>
            <div className='space-y-3 text-sm text-slate-700'>
              {[
                'integrationBulletOne',
                'integrationBulletTwo',
                'integrationBulletThree',
                'integrationBulletFour',
              ].map((key) => (
                <div key={key} className='flex items-start gap-3'>
                  <Bot className='mt-0.5 h-4 w-4 text-teal-700' />
                  <span>{t(key as MessageKey)}</span>
                </div>
              ))}
            </div>
          </div>

          <div className='rounded-[28px] bg-slate-950 p-6 text-sm text-slate-200 shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]'>
            <div className='flex items-center justify-between border-b border-white/10 pb-4'>
              <p className='font-mono text-slate-400'>example.py</p>
              <span className='rounded-full bg-white/10 px-3 py-1 text-xs text-slate-300'>
                OpenAI compatible
              </span>
            </div>
            <pre className='mt-5 overflow-x-auto whitespace-pre-wrap font-mono leading-7 text-teal-300'>{`from openai import OpenAI

client = OpenAI(
    base_url="${baseUrl}/v1",
    api_key="your-new-api-key"
)

response = client.chat.completions.create(
    model="gpt-4.1",
    messages=[{"role": "user", "content": "Hello"}]
)`}</pre>
          </div>
        </section>

        <section className='mt-28 rounded-[36px] border border-slate-200 bg-slate-950 px-8 py-10 text-white shadow-[0_32px_100px_-54px_rgba(15,23,42,0.55)] lg:px-10'>
          <div className='flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between'>
            <div className='max-w-3xl space-y-4'>
              <p className='text-xs font-medium uppercase tracking-[0.24em] text-teal-300'>
                web-v2
              </p>
              <h2 className='text-3xl font-semibold tracking-tight lg:text-4xl'>
                {t('ctaTitle')}
              </h2>
              <p className='text-lg leading-8 text-slate-300'>{t('ctaDescription')}</p>
            </div>
            <div className='flex flex-wrap gap-3'>
              <a
                href='/console'
                className='inline-flex items-center justify-center rounded-full bg-white px-6 py-3 text-sm font-semibold text-slate-950 transition-colors hover:bg-slate-100'
              >
                {t('ctaPrimary')}
              </a>
              <a
                href='/login'
                className='inline-flex items-center justify-center rounded-full border border-white/20 bg-white/10 px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-white/15'
              >
                {t('ctaSecondary')}
              </a>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}
