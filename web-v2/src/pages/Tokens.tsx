import { useMemo, useState } from 'react';
import { Gauge, KeyRound, LockKeyhole, ShieldCheck } from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchTokens } from '../lib/tokens';
import { useI18n } from '../i18n/I18nProvider';

type FilterMode = 'all' | 'active' | 'limited' | 'restricted';

function formatTimestamp(timestamp?: number, neverLabel?: string) {
  if (!timestamp || timestamp <= 0) {
    return neverLabel || '--';
  }
  return new Date(timestamp * 1000).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export default function Tokens() {
  const tokens = useAsyncData(fetchTokens, []);
  const { t } = useI18n();
  const [filter, setFilter] = useState<FilterMode>('all');

  const normalizedTokens = useMemo(() => {
    return (tokens.data || []).map((token) => ({
      id: token.id,
      name: token.name || `Token #${token.id}`,
      key: token.key || '',
      group: token.group || 'default',
      active: token.status === 1,
      statusText: token.status === 1 ? t('tokensStatusActive') : t('tokensStatusInactive'),
      quotaText: token.unlimited_quota
        ? t('tokensUnlimited')
        : `${(token.remain_quota || 0).toLocaleString()} ${t('tokensQuotaRemaining')}`,
      unlimited: Boolean(token.unlimited_quota),
      restricted: Boolean(token.model_limits_enabled),
      accessMode: token.model_limits_enabled ? t('tokensRestricted') : t('tokensStandard'),
      createdAt: formatTimestamp(token.created_time),
      accessedAt: formatTimestamp(token.accessed_time),
      expiresAt: formatTimestamp(token.expired_time, t('tokensNever')),
      usedQuota: token.used_quota || 0,
    }));
  }, [tokens.data, t]);

  const filteredTokens = normalizedTokens.filter((token) => {
    if (filter === 'active') return token.active;
    if (filter === 'limited') return !token.unlimited;
    if (filter === 'restricted') return token.restricted;
    return true;
  });

  const activeCount = normalizedTokens.filter((token) => token.active).length;
  const unlimitedCount = normalizedTokens.filter((token) => token.unlimited).length;
  const restrictedCount = normalizedTokens.filter((token) => token.restricted).length;

  const stats = [
    {
      label: t('tokensMetricTotal'),
      value: String(normalizedTokens.length),
      hint: t('tokensMetricTotalHint'),
      icon: KeyRound,
    },
    {
      label: t('tokensMetricActive'),
      value: String(activeCount),
      hint: t('tokensMetricActiveHint'),
      icon: ShieldCheck,
    },
    {
      label: t('tokensMetricUnlimited'),
      value: String(unlimitedCount),
      hint: t('tokensMetricUnlimitedHint'),
      icon: Gauge,
    },
    {
      label: t('tokensMetricRestricted'),
      value: String(restrictedCount),
      hint: t('tokensMetricRestrictedHint'),
      icon: LockKeyhole,
    },
  ];

  return (
    <div className='space-y-8'>
      <section className='hero-console'>
        <div className='max-w-3xl space-y-5'>
          <p className='eyebrow'>{t('tokensEyebrow')}</p>
          <h1 className='page-hero-title'>{t('tokensTitle')}</h1>
          <p className='page-hero-description'>{t('tokensDescription')}</p>
        </div>

        <div className='hero-console-panel'>
          <p className='text-xs uppercase tracking-[0.24em] text-slate-500'>
            {t('tokensFilterTitle')}
          </p>
          <div className='mt-5 flex flex-wrap gap-3'>
            {(
              [
                ['all', t('tokensFilterAll')],
                ['active', t('tokensFilterActive')],
                ['limited', t('tokensFilterLimited')],
                ['restricted', t('tokensFilterRestricted')],
              ] as Array<[FilterMode, string]>
            ).map(([value, label]) => (
              <button
                key={value}
                type='button'
                onClick={() => setFilter(value)}
                className={
                  filter === value
                    ? 'inline-flex rounded-full bg-slate-950 px-4 py-2 text-sm font-medium text-white'
                    : 'inline-flex rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-50'
                }
              >
                {label}
              </button>
            ))}
          </div>
          <div className='mt-6 grid gap-3 text-sm text-slate-600'>
            <div className='rounded-2xl border border-slate-200 bg-white px-4 py-3'>
              {activeCount} / {normalizedTokens.length} {t('tokensFilterActive')}
            </div>
            <div className='rounded-2xl border border-slate-200 bg-white px-4 py-3'>
              {restrictedCount} / {normalizedTokens.length} {t('tokensFilterRestricted')}
            </div>
          </div>
        </div>
      </section>

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {stats.map((item) => (
          <MetricCard key={item.label} {...item} />
        ))}
      </section>

      <StatePanel
        loading={tokens.loading}
        error={tokens.error}
        empty={!tokens.loading && !tokens.error && normalizedTokens.length === 0}
        title={t('tokensStatusTitle')}
        description={t('tokensStatusDescription')}
      />

      <section className='grid gap-4 lg:grid-cols-3'>
        {[
          {
            title: t('tokensHighlightOne'),
            description: t('tokensHighlightOneDescription'),
          },
          {
            title: t('tokensHighlightTwo'),
            description: t('tokensHighlightTwoDescription'),
          },
          {
            title: t('tokensHighlightThree'),
            description: t('tokensHighlightThreeDescription'),
          },
        ].map((item) => (
          <article key={item.title} className='panel-card p-6'>
            <p className='text-lg font-semibold text-slate-950'>{item.title}</p>
            <p className='mt-3 text-sm leading-7 text-slate-600'>{item.description}</p>
          </article>
        ))}
      </section>

      <section className='panel-card overflow-hidden'>
        <div className='border-b border-slate-200 px-6 py-5'>
          <p className='eyebrow'>{t('tokensEyebrow')}</p>
          <h2 className='mt-3 text-3xl font-semibold tracking-tight text-slate-950'>
            {t('tokensTableTitle')}
          </h2>
          <p className='mt-3 max-w-3xl text-base leading-7 text-slate-600'>
            {t('tokensTableDescription')}
          </p>
        </div>

        {filteredTokens.length > 0 ? (
          <div className='overflow-x-auto'>
            <table className='min-w-full'>
              <thead>
                <tr className='border-b border-slate-200 bg-slate-50/80 text-left text-xs uppercase tracking-[0.2em] text-slate-500'>
                  <th className='px-6 py-4 font-medium'>{t('tokensColumnToken')}</th>
                  <th className='px-6 py-4 font-medium'>{t('tokensColumnGroup')}</th>
                  <th className='px-6 py-4 font-medium'>{t('tokensColumnStatus')}</th>
                  <th className='px-6 py-4 font-medium'>{t('tokensColumnQuota')}</th>
                  <th className='px-6 py-4 font-medium'>{t('tokensColumnAccess')}</th>
                  <th className='px-6 py-4 font-medium'>{t('tokensColumnCreated')}</th>
                  <th className='px-6 py-4 font-medium'>{t('tokensColumnExpires')}</th>
                </tr>
              </thead>
              <tbody>
                {filteredTokens.map((token) => (
                  <tr
                    key={token.id}
                    className='border-b border-slate-100 text-sm text-slate-700 transition-colors hover:bg-slate-50/80'
                  >
                    <td className='px-6 py-5'>
                      <div className='space-y-2'>
                        <p className='font-medium text-slate-950'>{token.name}</p>
                        {token.key ? (
                          <p className='font-mono text-xs text-slate-500'>{token.key}</p>
                        ) : null}
                        <p className='text-xs text-slate-500'>
                          {t('tokensUsedAt')}: {token.accessedAt}
                        </p>
                      </div>
                    </td>
                    <td className='px-6 py-5'>{token.group}</td>
                    <td className='px-6 py-5'>
                      <span
                        className={
                          token.active
                            ? 'inline-flex rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700'
                            : 'inline-flex rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600'
                        }
                      >
                        {token.statusText}
                      </span>
                    </td>
                    <td className='px-6 py-5'>
                      <div className='space-y-1'>
                        <p>{token.quotaText}</p>
                        <p className='text-xs text-slate-500'>
                          Used: {token.usedQuota.toLocaleString()}
                        </p>
                      </div>
                    </td>
                    <td className='px-6 py-5'>
                      <span className='inline-flex rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-600'>
                        {token.accessMode}
                      </span>
                    </td>
                    <td className='px-6 py-5'>{token.createdAt}</td>
                    <td className='px-6 py-5'>{token.expiresAt}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className='px-6 py-10 text-sm text-slate-500'>{t('tokensEmpty')}</div>
        )}
      </section>
    </div>
  );
}
