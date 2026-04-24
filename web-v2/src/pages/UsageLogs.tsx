import { useMemo, useState } from 'react';
import { BarChart3, Coins, ReceiptText, TimerReset } from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { useI18n } from '../i18n/I18nProvider';
import { fetchUsageLogs } from '../lib/usageLogs';
import { useStatus } from '../hooks/useStatus';

function parseOther(other?: string) {
  if (!other) return {};
  try {
    return JSON.parse(other) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function formatLatency(milliseconds: number, t: (key: any) => string) {
  if (milliseconds < 1000) {
    return `${milliseconds}${t('usageTimeUnitMs')}`;
  }
  return `${(milliseconds / 1000).toFixed(2)}${t('usageTimeUnitS')}`;
}

function formatTimestamp(timestamp: number) {
  return new Date(timestamp * 1000).toLocaleString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
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
  const pageSize = 20;

  const usage = useAsyncData(
    () =>
      fetchUsageLogs({
        days,
        tokenName,
      }),
    [days, tokenName],
  );

  const quotaPerUnit = status.data?.quota_per_unit;
  const quotaDisplayType = status.data?.quota_display_type;
  const usdExchangeRate = status.data?.usd_exchange_rate;
  const customCurrencySymbol = status.data?.custom_currency_symbol;
  const customCurrencyExchangeRate = status.data?.custom_currency_exchange_rate;

  const normalizedRows = useMemo(() => {
    return (usage.data?.items || []).map((item) => {
      const other = parseOther(item.other);
      const endpoint =
        typeof other.request_path === 'string'
          ? other.request_path
          : t('usageUnknownEndpoint');
      const totalTokens = (item.prompt_tokens || 0) + (item.completion_tokens || 0);

      return {
        id: item.id,
        tokenName: item.token_name || 'default',
        modelName: item.model_name || 'unknown',
        endpoint,
        mode: item.is_stream ? t('usageModeStream') : t('usageModeStandard'),
        promptTokens: item.prompt_tokens || 0,
        completionTokens: item.completion_tokens || 0,
        totalTokens,
        quota: item.quota || 0,
        spend: formatQuota(
          item.quota || 0,
          quotaPerUnit,
          quotaDisplayType,
          usdExchangeRate,
          customCurrencySymbol,
          customCurrencyExchangeRate,
        ),
        latency: item.use_time || 0,
        latencyText: formatLatency(item.use_time || 0, t),
        time: formatTimestamp(item.created_at),
        requestId: item.request_id || '',
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

  const pagedRows = normalizedRows.slice((page - 1) * pageSize, page * pageSize);
  const totalRequests = usage.data?.total || 0;
  const totalTokens = normalizedRows.reduce((sum, row) => sum + row.totalTokens, 0);
  const averageLatency =
    normalizedRows.length > 0
      ? Math.round(normalizedRows.reduce((sum, row) => sum + row.latency, 0) / normalizedRows.length)
      : 0;
  const totalSpend = formatQuota(
    usage.data?.stat.quota || 0,
    quotaPerUnit,
    quotaDisplayType,
    usdExchangeRate,
    customCurrencySymbol,
    customCurrencyExchangeRate,
  );

  const tokenOptions = Array.from(
    new Set(normalizedRows.map((row) => row.tokenName).filter(Boolean)),
  );
  const totalPages = Math.max(1, Math.ceil(normalizedRows.length / pageSize));

  const stats = [
    {
      label: t('usageRequests'),
      value: totalRequests.toLocaleString(),
      hint: t('usageRequestsHint'),
      icon: BarChart3,
    },
    {
      label: t('usageTokens'),
      value:
        totalTokens >= 1000000
          ? `${(totalTokens / 1000000).toFixed(2)}M`
          : totalTokens.toLocaleString(),
      hint: t('usageTokensHint'),
      icon: Coins,
    },
    {
      label: t('usageSpend'),
      value: totalSpend,
      hint: t('usageSpendHint'),
      icon: ReceiptText,
    },
    {
      label: t('usageLatency'),
      value: formatLatency(averageLatency, t),
      hint: t('usageLatencyHint'),
      icon: TimerReset,
    },
  ];

  function handleExport() {
    const headers = [
      t('usageTableToken'),
      t('usageTableModel'),
      t('usageTableEndpoint'),
      t('usageTableMode'),
      t('usageTableTokens'),
      t('usageTableSpend'),
      t('usageTableLatency'),
      t('usageTableTime'),
      t('usageTableRequestId'),
    ];

    const lines = normalizedRows.map((row) =>
      [
        row.tokenName,
        row.modelName,
        row.endpoint,
        row.mode,
        row.totalTokens,
        row.spend,
        row.latencyText,
        row.time,
        row.requestId,
      ]
        .map((value) => `"${String(value).replaceAll('"', '""')}"`)
        .join(','),
    );

    const csv = [headers.join(','), ...lines].join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `usage-logs-${days}d.csv`;
    link.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className='space-y-8'>
      <section className='space-y-4'>
        <p className='eyebrow'>{t('usageEyebrow')}</p>
        <h1 className='page-title'>{t('usageTitle')}</h1>
        <p className='page-description'>{t('usageDescription')}</p>
      </section>

      <section className='rounded-[36px] border border-white/80 bg-[radial-gradient(circle_at_top,_rgba(45,212,191,0.14),_transparent_58%),linear-gradient(180deg,_rgba(255,255,255,0.92),_rgba(244,253,253,0.9))] p-6 shadow-[0_32px_100px_-52px_rgba(15,23,42,0.28)]'>
        <div className='grid gap-4 lg:grid-cols-4'>
          {stats.map((item) => (
            <MetricCard key={item.label} {...item} />
          ))}
        </div>
      </section>

      <section className='panel-card p-6'>
        <div className='grid gap-4 xl:grid-cols-[240px_220px_1fr_auto] xl:items-end'>
          <div className='space-y-2'>
            <label className='text-sm font-medium text-slate-700'>{t('usageFilterToken')}</label>
            <select
              value={tokenName}
              onChange={(event) => {
                setTokenName(event.target.value);
                setPage(1);
              }}
              className='input-shell'
            >
              <option value=''>{t('usageFilterTokenAll')}</option>
              {tokenOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </div>

          <div className='space-y-2'>
            <label className='text-sm font-medium text-slate-700'>{t('usageFilterRange')}</label>
            <select
              value={days}
              onChange={(event) => {
                setDays(Number(event.target.value) as 1 | 7 | 30);
                setPage(1);
              }}
              className='input-shell'
            >
              <option value={1}>{t('usageRange1d')}</option>
              <option value={7}>{t('usageRange7d')}</option>
              <option value={30}>{t('usageRange30d')}</option>
            </select>
          </div>

          <div />

          <div className='flex flex-wrap gap-3'>
            <button type='button' onClick={() => usage.reload()} className='secondary-button'>
              {t('usageRefresh')}
            </button>
            <button
              type='button'
              onClick={() => {
                setTokenName('');
                setDays(7);
                setPage(1);
              }}
              className='secondary-button'
            >
              {t('usageReset')}
            </button>
            <button type='button' onClick={handleExport} className='primary-button'>
              {t('usageExport')}
            </button>
          </div>
        </div>
      </section>

      <StatePanel
        loading={usage.loading}
        error={usage.error}
        empty={!usage.loading && !usage.error && normalizedRows.length === 0}
        title={t('usageLoadingTitle')}
        description={t('usageLoadingDescription')}
      />

      <section className='panel-card overflow-hidden'>
        <div className='overflow-x-auto'>
          <table className='min-w-full'>
            <thead>
              <tr className='border-b border-slate-200 bg-slate-50/80 text-left text-xs uppercase tracking-[0.2em] text-slate-500'>
                <th className='px-6 py-4 font-medium'>{t('usageTableToken')}</th>
                <th className='px-6 py-4 font-medium'>{t('usageTableModel')}</th>
                <th className='px-6 py-4 font-medium'>{t('usageTableEndpoint')}</th>
                <th className='px-6 py-4 font-medium'>{t('usageTableMode')}</th>
                <th className='px-6 py-4 font-medium'>{t('usageTableTokens')}</th>
                <th className='px-6 py-4 font-medium'>{t('usageTableSpend')}</th>
                <th className='px-6 py-4 font-medium'>{t('usageTableLatency')}</th>
                <th className='px-6 py-4 font-medium'>{t('usageTableTime')}</th>
              </tr>
            </thead>
            <tbody>
              {pagedRows.length > 0 ? (
                pagedRows.map((row) => (
                  <tr
                    key={`${row.id}-${row.requestId}`}
                    className='border-b border-slate-100 text-sm text-slate-700 transition-colors hover:bg-slate-50/80'
                  >
                    <td className='px-6 py-5 align-top'>
                      <div className='space-y-2'>
                        <p className='font-medium text-slate-950'>{row.tokenName}</p>
                        {row.requestId ? (
                          <p className='text-xs text-slate-500'>
                            {t('usageTableRequestId')}: {row.requestId}
                          </p>
                        ) : null}
                      </div>
                    </td>
                    <td className='px-6 py-5 align-top'>{row.modelName}</td>
                    <td className='px-6 py-5 align-top'>{row.endpoint}</td>
                    <td className='px-6 py-5 align-top'>
                      <span className='inline-flex rounded-full bg-sky-50 px-3 py-1 text-xs font-medium text-sky-700'>
                        {row.mode}
                      </span>
                    </td>
                    <td className='px-6 py-5 align-top'>
                      <div className='space-y-1'>
                        <p className='text-slate-900'>
                          {t('usageTokenInput')} {row.promptTokens.toLocaleString()} / {t('usageTokenOutput')}{' '}
                          {row.completionTokens.toLocaleString()}
                        </p>
                        <p className='text-sm font-semibold text-sky-600'>
                          {t('usageTokenTotal')} {row.totalTokens.toLocaleString()}
                        </p>
                      </div>
                    </td>
                    <td className='px-6 py-5 align-top'>
                      <span className='font-semibold text-emerald-600'>{row.spend}</span>
                    </td>
                    <td className='px-6 py-5 align-top'>{row.latencyText}</td>
                    <td className='px-6 py-5 align-top'>{row.time}</td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={8} className='px-6 py-10 text-center text-sm text-slate-500'>
                    {t('usageEmpty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        <div className='flex flex-col gap-4 border-t border-slate-200 px-6 py-5 lg:flex-row lg:items-center lg:justify-between'>
          <p className='text-sm text-slate-500'>
            {Math.min((page - 1) * pageSize + 1, normalizedRows.length)}-
            {Math.min(page * pageSize, normalizedRows.length)} / {normalizedRows.length}
          </p>
          <div className='flex items-center gap-2'>
            <button
              type='button'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page === 1}
              className='secondary-button disabled:cursor-not-allowed disabled:opacity-50'
            >
              Prev
            </button>
            <div className='rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700'>
              {page} / {totalPages}
            </div>
            <button
              type='button'
              onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
              disabled={page === totalPages}
              className='secondary-button disabled:cursor-not-allowed disabled:opacity-50'
            >
              Next
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
