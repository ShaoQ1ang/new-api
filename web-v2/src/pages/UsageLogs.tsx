import { useMemo, useState } from 'react';
import { useAsyncData } from '../hooks/useAsyncData';
import { useI18n } from '../i18n/I18nProvider';
import { fetchUsageLogs } from '../lib/usageLogs';
import { useStatus } from '../hooks/useStatus';
import StatePanel from '../components/ui/StatePanel';

function parseOther(other?: string) {
  if (!other) return {};
  try {
    return JSON.parse(other) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function formatTime(timestamp: number) {
  return new Date(timestamp * 1000).toLocaleString('en-US', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
}

function formatLatency(milliseconds: number) {
  if (milliseconds < 1000) return `${milliseconds}ms`;
  return `${(milliseconds / 1000).toFixed(1)}s`;
}

function formatQuota(
  quota: number,
  quotaPerUnit?: number,
  displayType?: string,
  usdExchangeRate?: number,
  customCurrencySymbol?: string,
  customCurrencyExchangeRate?: number,
) {
  if (!quotaPerUnit || quotaPerUnit <= 0 || displayType === 'TOKENS') {
    return quota.toLocaleString();
  }

  const usdValue = quota / quotaPerUnit;

  if (displayType === 'CNY') {
    return `¥${(usdValue * (usdExchangeRate || 1)).toFixed(4)}`;
  }

  if (displayType === 'CUSTOM') {
    return `${customCurrencySymbol || '¤'}${(
      usdValue * (customCurrencyExchangeRate || 1)
    ).toFixed(4)}`;
  }

  return `$${usdValue.toFixed(4)}`;
}

export default function UsageLogs() {
  const { t } = useI18n();
  const status = useStatus();
  const [days, setDays] = useState<1 | 7 | 30>(7);
  const [tokenName, setTokenName] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 12;

  const usage = useAsyncData(() => fetchUsageLogs({ days, tokenName }), [days, tokenName]);

  const quotaPerUnit = status.data?.quota_per_unit;
  const quotaDisplayType = status.data?.quota_display_type;
  const usdExchangeRate = status.data?.usd_exchange_rate;
  const customCurrencySymbol = status.data?.custom_currency_symbol;
  const customCurrencyExchangeRate = status.data?.custom_currency_exchange_rate;

  const rows = useMemo(() => {
    return (usage.data?.items || []).map((item) => {
      const other = parseOther(item.other);
      return {
        id: item.id,
        tokenName: item.token_name || 'default',
        modelName: item.model_name || 'unknown',
        endpoint:
          typeof other.request_path === 'string'
            ? other.request_path
            : t('usageUnknownEndpoint'),
        mode: item.is_stream ? t('usageModeStream') : t('usageModeStandard'),
        totalTokens: (item.prompt_tokens || 0) + (item.completion_tokens || 0),
        spend: formatQuota(
          item.quota || 0,
          quotaPerUnit,
          quotaDisplayType,
          usdExchangeRate,
          customCurrencySymbol,
          customCurrencyExchangeRate,
        ),
        latency: formatLatency(item.use_time || 0),
        time: formatTime(item.created_at),
      };
    });
  }, [
    usage.data?.items,
    t,
    quotaPerUnit,
    quotaDisplayType,
    usdExchangeRate,
    customCurrencySymbol,
    customCurrencyExchangeRate,
  ]);

  const tokenOptions = Array.from(new Set(rows.map((row) => row.tokenName)));
  const pagedRows = rows.slice((page - 1) * pageSize, page * pageSize);
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));

  const summary = {
    requests: usage.data?.total || 0,
    tokens: rows.reduce((sum, row) => sum + row.totalTokens, 0),
    spend: formatQuota(
      usage.data?.stat.quota || 0,
      quotaPerUnit,
      quotaDisplayType,
      usdExchangeRate,
      customCurrencySymbol,
      customCurrencyExchangeRate,
    ),
  };

  return (
    <div className='space-y-5'>
      <section className='flex flex-col gap-4 rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm lg:flex-row lg:items-end lg:justify-between'>
        <div>
          <h1 className='text-2xl font-semibold text-slate-950'>{t('usageEyebrow')}</h1>
          <p className='mt-1 text-sm text-slate-500'>{t('usageDescription')}</p>
        </div>

        <div className='grid gap-3 sm:grid-cols-3'>
          <div className='rounded-2xl bg-slate-50 px-4 py-3'>
            <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>{t('usageRequests')}</p>
            <p className='mt-2 text-2xl font-semibold text-slate-950'>
              {summary.requests.toLocaleString()}
            </p>
          </div>
          <div className='rounded-2xl bg-slate-50 px-4 py-3'>
            <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>{t('usageTokens')}</p>
            <p className='mt-2 text-2xl font-semibold text-slate-950'>
              {summary.tokens >= 1000000
                ? `${(summary.tokens / 1000000).toFixed(2)}M`
                : summary.tokens.toLocaleString()}
            </p>
          </div>
          <div className='rounded-2xl bg-slate-50 px-4 py-3'>
            <p className='text-xs uppercase tracking-[0.18em] text-slate-500'>{t('usageSpend')}</p>
            <p className='mt-2 text-2xl font-semibold text-emerald-600'>{summary.spend}</p>
          </div>
        </div>
      </section>

      <section className='rounded-[28px] border border-slate-200 bg-white p-4 shadow-sm'>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
          <div className='flex flex-col gap-3 sm:flex-row'>
            <select
              value={tokenName}
              onChange={(event) => {
                setTokenName(event.target.value);
                setPage(1);
              }}
              className='min-w-[180px] rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700 outline-none'
            >
              <option value=''>{t('usageFilterTokenAll')}</option>
              {tokenOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>

            <div className='inline-flex rounded-xl border border-slate-200 bg-slate-50 p-1'>
              {(
                [
                  [1, t('usageRange1d')],
                  [7, t('usageRange7d')],
                  [30, t('usageRange30d')],
                ] as const
              ).map(([value, label]) => (
                <button
                  key={value}
                  type='button'
                  onClick={() => {
                    setDays(value);
                    setPage(1);
                  }}
                  className={
                    days === value
                      ? 'rounded-lg bg-white px-4 py-2 text-sm font-medium text-slate-950 shadow-sm'
                      : 'rounded-lg px-4 py-2 text-sm text-slate-500'
                  }
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div className='flex gap-2'>
            <button type='button' onClick={() => usage.reload()} className='secondary-button !rounded-xl !px-4 !py-2.5'>
              {t('usageRefresh')}
            </button>
            <button
              type='button'
              onClick={() => {
                setTokenName('');
                setDays(7);
                setPage(1);
              }}
              className='secondary-button !rounded-xl !px-4 !py-2.5'
            >
              {t('usageReset')}
            </button>
          </div>
        </div>
      </section>

      {usage.loading || usage.error ? (
        <StatePanel
          loading={usage.loading}
          error={usage.error}
          empty={false}
          title={t('usageLoadingTitle')}
          description={t('usageLoadingDescription')}
        />
      ) : null}

      <section className='overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-sm'>
        <div className='overflow-x-auto'>
          <table className='min-w-full'>
            <thead>
              <tr className='border-b border-slate-200 text-left text-xs uppercase tracking-[0.18em] text-slate-500'>
                <th className='px-5 py-4 font-medium'>{t('usageTableToken')}</th>
                <th className='px-5 py-4 font-medium'>{t('usageTableModel')}</th>
                <th className='px-5 py-4 font-medium'>{t('usageTableTokens')}</th>
                <th className='px-5 py-4 font-medium'>{t('usageTableSpend')}</th>
                <th className='px-5 py-4 font-medium'>{t('usageTableLatency')}</th>
                <th className='px-5 py-4 font-medium'>{t('usageTableTime')}</th>
              </tr>
            </thead>
            <tbody>
              {pagedRows.length > 0 ? (
                pagedRows.map((row) => (
                  <tr key={row.id} className='border-b border-slate-100 text-sm text-slate-700'>
                    <td className='px-5 py-4'>{row.tokenName}</td>
                    <td className='px-5 py-4'>
                      <div className='space-y-1'>
                        <p className='font-medium text-slate-950'>{row.modelName}</p>
                        <p className='text-xs text-slate-500'>{row.endpoint}</p>
                      </div>
                    </td>
                    <td className='px-5 py-4'>{row.totalTokens.toLocaleString()}</td>
                    <td className='px-5 py-4 font-medium text-emerald-600'>{row.spend}</td>
                    <td className='px-5 py-4'>{row.latency}</td>
                    <td className='px-5 py-4 text-slate-500'>{row.time}</td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={6} className='px-5 py-10 text-center text-sm text-slate-500'>
                    {t('usageEmpty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        <div className='flex flex-col gap-3 border-t border-slate-200 px-5 py-4 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-sm text-slate-500'>
            {Math.min((page - 1) * pageSize + 1, rows.length)}-
            {Math.min(page * pageSize, rows.length)} / {rows.length}
          </p>

          <div className='flex items-center gap-2'>
            <button
              type='button'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page === 1}
              className='rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm text-slate-600 disabled:opacity-40'
            >
              Prev
            </button>
            <div className='rounded-xl bg-slate-50 px-4 py-2 text-sm font-medium text-slate-700'>
              {page} / {totalPages}
            </div>
            <button
              type='button'
              onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
              disabled={page === totalPages}
              className='rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm text-slate-600 disabled:opacity-40'
            >
              Next
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
