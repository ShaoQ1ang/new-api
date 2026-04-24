import { useMemo, useState } from 'react';
import { Gauge, KeyRound, LockKeyhole, ShieldCheck } from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchTokens } from '../lib/tokens';
import { useI18n } from '../i18n/I18nProvider';

type FilterMode = 'all' | 'active' | 'limited' | 'restricted';

function formatTimestamp(timestamp?: number, neverLabel?: string) {
  if (!timestamp || timestamp <= 0) return neverLabel || '--';
  return new Date(timestamp * 1000).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

export default function Tokens() {
  const tokens = useAsyncData(fetchTokens, []);
  const { t } = useI18n();
  const [filter, setFilter] = useState<FilterMode>('all');

  const rows = useMemo(
    () =>
      (tokens.data || []).map((token) => ({
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
        expiresAt: formatTimestamp(token.expired_time, t('tokensNever')),
      })),
    [tokens.data, t],
  );

  const filtered = rows.filter((token) => {
    if (filter === 'active') return token.active;
    if (filter === 'limited') return !token.unlimited;
    if (filter === 'restricted') return token.restricted;
    return true;
  });

  const stats = [
    {
      label: t('tokensMetricTotal'),
      value: String(rows.length),
      hint: t('tokensMetricTotalHint'),
      icon: KeyRound,
    },
    {
      label: t('tokensMetricActive'),
      value: String(rows.filter((item) => item.active).length),
      hint: t('tokensMetricActiveHint'),
      icon: ShieldCheck,
    },
    {
      label: t('tokensMetricUnlimited'),
      value: String(rows.filter((item) => item.unlimited).length),
      hint: t('tokensMetricUnlimitedHint'),
      icon: Gauge,
    },
    {
      label: t('tokensMetricRestricted'),
      value: String(rows.filter((item) => item.restricted).length),
      hint: t('tokensMetricRestrictedHint'),
      icon: LockKeyhole,
    },
  ];

  return (
    <div className='space-y-5'>
      <section className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
        <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <h1 className='text-2xl font-semibold text-slate-950'>{t('tokensEyebrow')}</h1>
            <p className='mt-1 text-sm text-slate-500'>{t('tokensDescription')}</p>
          </div>
          <div className='inline-flex rounded-xl border border-slate-200 bg-slate-50 p-1'>
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
                    ? 'rounded-lg bg-white px-4 py-2 text-sm font-medium text-slate-950 shadow-sm'
                    : 'rounded-lg px-4 py-2 text-sm text-slate-500'
                }
              >
                {label}
              </button>
            ))}
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
        empty={!tokens.loading && !tokens.error && rows.length === 0}
        title={t('tokensStatusTitle')}
        description={t('tokensStatusDescription')}
      />

      <section className='overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-sm'>
        <div className='overflow-x-auto'>
          <table className='min-w-full'>
            <thead>
              <tr className='border-b border-slate-200 text-left text-xs uppercase tracking-[0.18em] text-slate-500'>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnToken')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnGroup')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnStatus')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnQuota')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnAccess')}</th>
                <th className='px-5 py-4 font-medium'>{t('tokensColumnExpires')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length > 0 ? (
                filtered.map((token) => (
                  <tr key={token.id} className='border-b border-slate-100 text-sm text-slate-700'>
                    <td className='px-5 py-4'>
                      <div className='space-y-1'>
                        <p className='font-medium text-slate-950'>{token.name}</p>
                        {token.key ? <p className='font-mono text-xs text-slate-500'>{token.key}</p> : null}
                      </div>
                    </td>
                    <td className='px-5 py-4'>{token.group}</td>
                    <td className='px-5 py-4'>
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
                    <td className='px-5 py-4'>{token.quotaText}</td>
                    <td className='px-5 py-4'>{token.accessMode}</td>
                    <td className='px-5 py-4 text-slate-500'>{token.expiresAt}</td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={6} className='px-5 py-10 text-center text-sm text-slate-500'>
                    {t('tokensEmpty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
