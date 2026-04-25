import { useEffect, useMemo, useRef, useState } from 'react';
import { Check, Copy, Gauge, KeyRound, Plus, ShieldCheck } from 'lucide-react';
import MetricCard from '../components/ui/MetricCard';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { useStatus } from '../hooks/useStatus';
import {
  createToken,
  deleteToken,
  fetchTokenKey,
  fetchTokens,
  updateToken,
  updateTokenStatus,
  type TokenInput,
  type TokenWorkspaceResponse,
} from '../lib/tokens';
import { useI18n } from '../i18n/I18nProvider';

type FilterMode = 'all' | 'active' | 'limited';

type FormState = {
  id?: number;
  name: string;
  remain_quota: string;
  expires_in_days: string;
  expires_at_input: string;
  expires_mode: 'never' | '7' | '30' | '90' | 'custom';
};

const defaultFormState: FormState = {
  name: '',
  remain_quota: '0',
  expires_in_days: '',
  expires_at_input: '',
  expires_mode: 'never',
};

function formatTimestamp(timestamp?: number, neverLabel?: string) {
  if (!timestamp || timestamp <= 0) return neverLabel || '--';
  return new Date(timestamp * 1000).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function formatDateTime(timestamp?: number, neverLabel?: string) {
  if (!timestamp || timestamp <= 0) return neverLabel || '--';
  return new Date(timestamp * 1000).toLocaleString('en-US', {
    month: '2-digit',
    day: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
}

function formatDateTimeInput(timestamp?: number) {
  if (!timestamp || timestamp <= 0) return '';
  const date = new Date(timestamp * 1000);
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`;
}

function parseDateTimeInput(value: string) {
  const timestamp = Date.parse(value);
  if (Number.isNaN(timestamp)) return -1;
  return Math.floor(timestamp / 1000);
}

function formatQuotaValue(
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

function formatQuotaInputValue(quota: number, quotaPerUnit?: number) {
  if (!quotaPerUnit || quotaPerUnit <= 0) {
    return String(Math.max(0, quota));
  }

  const usdValue = Math.max(0, quota) / quotaPerUnit;
  return usdValue.toFixed(4).replace(/\.?0+$/, '');
}

function parseQuotaInputValue(value: string, quotaPerUnit?: number) {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) return 0;

  if (!quotaPerUnit || quotaPerUnit <= 0) {
    return Math.round(normalized);
  }

  return Math.round(normalized * quotaPerUnit);
}

function buildTokenInput(form: FormState, quotaPerUnit?: number): TokenInput {
  let expiredTime = -1;

  if (form.expires_mode === 'custom') {
    expiredTime = parseDateTimeInput(form.expires_at_input);
  } else if (form.expires_mode !== 'never') {
    const expiresInDays = Math.max(0, Number(form.expires_mode));
    expiredTime =
      expiresInDays > 0
        ? Math.floor(Date.now() / 1000) + expiresInDays * 24 * 60 * 60
        : -1;
  }

  return {
    id: form.id,
    name: form.name.trim(),
    group: 'default',
    remain_quota: parseQuotaInputValue(form.remain_quota, quotaPerUnit),
    unlimited_quota: parseQuotaInputValue(form.remain_quota, quotaPerUnit) === 0,
    model_limits_enabled: false,
    expired_time: expiredTime,
  };
}

export default function Tokens() {
  const tableScrollRef = useRef<HTMLDivElement | null>(null);
  const status = useStatus();
  const { t } = useI18n();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const tokens = useAsyncData<TokenWorkspaceResponse>(() => fetchTokens(page, pageSize), [page, pageSize]);
  const [filter, setFilter] = useState<FilterMode>('all');
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [form, setForm] = useState<FormState>(defaultFormState);
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState('');
  const [copiedId, setCopiedId] = useState<number | null>(null);
  const [copyToast, setCopyToast] = useState('');
  const [busyId, setBusyId] = useState<number | null>(null);
  const [statusOverrides, setStatusOverrides] = useState<Record<number, number>>({});
  const [scrollState, setScrollState] = useState({
    left: 0,
    max: 0,
  });

  const tokenItems = tokens.data?.items || [];
  const totalItems = tokens.data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));

  const quotaPerUnit = status.data?.quota_per_unit;
  const quotaDisplayType = status.data?.quota_display_type;
  const usdExchangeRate = status.data?.usd_exchange_rate;
  const customCurrencySymbol = status.data?.custom_currency_symbol;
  const customCurrencyExchangeRate = status.data?.custom_currency_exchange_rate;

  const rows = useMemo(
    () =>
      tokenItems.map((token) => {
        const effectiveStatus = statusOverrides[token.id] ?? token.status ?? 1;

        return {
          id: token.id,
          name: token.name || `Token #${token.id}`,
          key: token.key || '',
          group: token.group || 'default',
          preview: Boolean(token.preview),
          active: effectiveStatus === 1,
          remainQuota: token.remain_quota || 0,
          usedQuota: token.used_quota || 0,
          totalQuota: Math.max(0, (token.used_quota || 0) + (token.remain_quota || 0)),
          todayQuota: token.today_quota || 0,
          totalUsageQuota: token.total_quota || Math.max(0, token.used_quota || 0),
          statusText: effectiveStatus === 1 ? t('tokensStatusActive') : t('tokensStatusInactive'),
          unlimited: Boolean(token.unlimited_quota),
          createdAt: token.created_time,
          createdAtText: formatDateTime(token.created_time),
          lastUsedAt: token.accessed_time,
          lastUsedAtText: formatDateTime(token.accessed_time, '--'),
          expiresAt: token.expired_time,
          expiresAtText: formatTimestamp(token.expired_time, t('tokensNever')),
          usedQuotaText: formatQuotaValue(
            Math.max(0, token.used_quota || 0),
            quotaPerUnit,
            quotaDisplayType,
            usdExchangeRate,
            customCurrencySymbol,
            customCurrencyExchangeRate,
          ),
          remainQuotaText: formatQuotaValue(
            Math.max(0, token.remain_quota || 0),
            quotaPerUnit,
            quotaDisplayType,
            usdExchangeRate,
            customCurrencySymbol,
            customCurrencyExchangeRate,
          ),
          totalQuotaText: formatQuotaValue(
            Math.max(0, (token.used_quota || 0) + (token.remain_quota || 0)),
            quotaPerUnit,
            quotaDisplayType,
            usdExchangeRate,
            customCurrencySymbol,
            customCurrencyExchangeRate,
          ),
          todayQuotaText: formatQuotaValue(
            Math.max(0, token.today_quota || 0),
            quotaPerUnit,
            quotaDisplayType,
            usdExchangeRate,
            customCurrencySymbol,
            customCurrencyExchangeRate,
          ),
          totalUsageQuotaText: formatQuotaValue(
            Math.max(0, token.total_quota || token.used_quota || 0),
            quotaPerUnit,
            quotaDisplayType,
            usdExchangeRate,
            customCurrencySymbol,
            customCurrencyExchangeRate,
          ),
          quotaLimitText: token.unlimited_quota
            ? t('tokensUsageUnlimited')
            : `${formatQuotaValue(
                Math.max(0, token.used_quota || 0),
                quotaPerUnit,
                quotaDisplayType,
                usdExchangeRate,
                customCurrencySymbol,
                customCurrencyExchangeRate,
              )} / ${formatQuotaValue(
                Math.max(0, (token.used_quota || 0) + (token.remain_quota || 0)),
                quotaPerUnit,
                quotaDisplayType,
                usdExchangeRate,
                customCurrencySymbol,
                customCurrencyExchangeRate,
              )}`,
        };
      }),
    [
      tokenItems,
      t,
      quotaPerUnit,
      quotaDisplayType,
      usdExchangeRate,
      customCurrencySymbol,
      customCurrencyExchangeRate,
      statusOverrides,
    ],
  );

  const filtered = rows.filter((token) => {
    if (filter === 'active') return token.active;
    if (filter === 'limited') return !token.unlimited;
    return true;
  });

  const stats = [
    {
      label: t('tokensMetricTotal'),
      value: String(totalItems),
      hint: '',
      icon: KeyRound,
    },
    {
      label: t('tokensMetricActive'),
      value: String(rows.filter((item) => item.active).length),
      hint: '',
      icon: ShieldCheck,
    },
    {
      label: t('tokensMetricUnlimited'),
      value: String(tokenItems.filter((item) => item.unlimited_quota).length),
      hint: '',
      icon: Gauge,
    },
  ];

  function openCreateForm() {
    setIsEditing(false);
    setForm(defaultFormState);
    setActionError('');
    setIsFormOpen(true);
  }

  function openEditForm(row: (typeof rows)[number]) {
    const expiresInDays =
      row.expiresAt && row.expiresAt > 0
        ? Math.max(1, Math.ceil((row.expiresAt - Math.floor(Date.now() / 1000)) / 86400))
        : '';

    setIsEditing(true);
    setForm({
      id: row.id,
      name: row.name,
      remain_quota: row.unlimited ? '0' : formatQuotaInputValue(Math.max(0, row.remainQuota), quotaPerUnit),
      expires_in_days: expiresInDays ? String(expiresInDays) : '',
      expires_at_input: row.expiresAt && row.expiresAt > 0 ? formatDateTimeInput(row.expiresAt) : '',
      expires_mode:
        expiresInDays === 7
          ? '7'
          : expiresInDays === 30
            ? '30'
            : expiresInDays === 90
              ? '90'
              : row.expiresAt && row.expiresAt > 0
                ? 'custom'
                : 'never',
    });
    setActionError('');
    setIsFormOpen(true);
  }

  async function handleSubmit() {
    if (!form.name.trim()) {
      setActionError(t('tokensActionError'));
      return;
    }

    setSubmitting(true);
    setActionError('');

    try {
      const payload = buildTokenInput(form, quotaPerUnit);
      if (isEditing) {
        await updateToken(payload);
      } else {
        await createToken(payload);
      }
      setIsFormOpen(false);
      setForm(defaultFormState);
      await tokens.reload();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : t('tokensActionError'));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(id: number) {
    if (!window.confirm(t('tokensDeleteConfirm'))) return;

    setBusyId(id);
    setActionError('');
    try {
      await deleteToken(id);
      if (tokenItems.length === 1 && page > 1) {
        setPage((current) => current - 1);
      } else {
        await tokens.reload();
      }
    } catch (error) {
      setActionError(error instanceof Error ? error.message : t('tokensActionError'));
    } finally {
      setBusyId(null);
    }
  }

  async function handleToggleStatus(id: number, active: boolean) {
    const nextStatus = active ? 2 : 1;
    setBusyId(id);
    setActionError('');
    setStatusOverrides((current) => ({ ...current, [id]: nextStatus }));
    try {
      await updateTokenStatus({
        id,
        status: nextStatus,
      });
    } catch (error) {
      setStatusOverrides((current) => {
        const rollback = { ...current };
        delete rollback[id];
        return rollback;
      });
      setActionError(error instanceof Error ? error.message : t('tokensActionError'));
    } finally {
      setBusyId(null);
    }
  }

  async function handleCopy(id: number) {
    const target = rows.find((row) => row.id === id);
    if (target?.preview) {
      setActionError('Copy is unavailable in preview mode');
      return;
    }

    setBusyId(id);
    setActionError('');
    try {
      const key = await fetchTokenKey(id);
      await navigator.clipboard.writeText(`sk-${key}`);
      setCopiedId(id);
      setCopyToast(t('tokensCopied'));
      window.setTimeout(() => setCopiedId((current) => (current === id ? null : current)), 1600);
      window.setTimeout(() => setCopyToast(''), 1600);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : t('tokensActionError'));
    } finally {
      setBusyId(null);
    }
  }

  function updateScrollState() {
    const node = tableScrollRef.current;
    if (!node) return;

    setScrollState({
      left: node.scrollLeft,
      max: Math.max(0, node.scrollWidth - node.clientWidth),
    });
  }

  useEffect(() => {
    updateScrollState();
    window.addEventListener('resize', updateScrollState);
    return () => window.removeEventListener('resize', updateScrollState);
  }, [rows.length]);

  useEffect(() => {
    setStatusOverrides({});
  }, [page, pageSize]);

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  return (
    <div className='space-y-5'>
      <section className='rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm'>
        <div className='space-y-4'>
          <div>
            <h1 className='whitespace-nowrap text-2xl font-semibold text-slate-950'>{t('tokensEyebrow')}</h1>
          </div>
          <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
            <div className='w-[320px] max-w-full overflow-x-auto'>
              <div className='grid min-w-[304px] grid-cols-3 rounded-xl border border-slate-200 bg-slate-50 p-1'>
              {(
                [
                  ['all', t('tokensFilterAll')],
                  ['active', t('tokensFilterActive')],
                  ['limited', t('tokensFilterLimited')],
                ] as Array<[FilterMode, string]>
              ).map(([value, label]) => (
                <button
                  key={value}
                  type='button'
                  onClick={() => setFilter(value)}
                  className={
                    filter === value
                      ? 'min-w-[96px] whitespace-nowrap rounded-lg bg-white px-4 py-2 text-sm font-medium text-slate-950 shadow-sm'
                      : 'min-w-[96px] whitespace-nowrap rounded-lg px-4 py-2 text-sm text-slate-500'
                  }
                >
                  {label}
                </button>
              ))}
              </div>
            </div>

            <button
              type='button'
              onClick={openCreateForm}
              className='primary-button !w-[132px] !justify-center whitespace-nowrap !px-4 !py-2.5'
            >
              <Plus className='h-4 w-4' />
              {t('tokensAdd')}
            </button>
          </div>
        </div>
      </section>

      {actionError ? (
        <div className='rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700'>
          {actionError}
        </div>
      ) : null}

      {copyToast ? (
        <div className='fixed left-1/2 top-24 z-50 min-w-[220px] -translate-x-1/2 rounded-2xl border border-emerald-200 bg-emerald-50 px-5 py-3 text-base font-semibold text-emerald-700 shadow-[0_18px_40px_-18px_rgba(16,185,129,0.45)] lg:top-28'>
          {copyToast}
        </div>
      ) : null}

      <section className='grid gap-4 md:grid-cols-3'>
        {stats.map((item) => (
          <MetricCard key={item.label} {...item} />
        ))}
      </section>

      <StatePanel
        loading={tokens.loading}
        error={tokens.error}
        empty={!tokens.loading && !tokens.error && totalItems === 0}
        title={t('tokensStatusTitle')}
        description={t('tokensStatusDescription')}
      />

      <section className='overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-sm'>
        <div
          ref={tableScrollRef}
          onScroll={updateScrollState}
          className='no-scrollbar overflow-x-auto'
        >
          <table className='table-fixed min-w-[1600px]'>
            <thead>
              <tr className='border-b border-slate-200 text-left text-xs uppercase tracking-[0.18em] text-slate-500'>
                <th className='sticky left-0 z-10 w-[192px] whitespace-nowrap border-r border-slate-200 bg-white px-5 py-4 font-medium shadow-[12px_0_24px_-18px_rgba(15,23,42,0.18)]'>
                  {t('tokensColumnToken')}
                </th>
                <th className='w-[280px] whitespace-nowrap px-5 py-4 font-medium'>{t('tokensColumnApiKey')}</th>
                <th className='w-[120px] whitespace-nowrap px-5 py-4 font-medium'>{t('tokensColumnStatus')}</th>
                <th className='w-[360px] whitespace-nowrap px-5 py-4 font-medium'>{t('tokensColumnUsage')}</th>
                <th className='w-[150px] whitespace-nowrap px-5 py-4 font-medium'>{t('tokensColumnExpires')}</th>
                <th className='w-[140px] whitespace-nowrap px-5 py-4 font-medium'>{t('tokensColumnGroup')}</th>
                <th className='w-[170px] whitespace-nowrap px-5 py-4 font-medium'>{t('tokensColumnLastUsed')}</th>
                <th className='w-[170px] whitespace-nowrap px-5 py-4 font-medium'>{t('tokensColumnCreated')}</th>
                <th className='sticky right-0 z-10 w-[260px] whitespace-nowrap border-l border-slate-200 bg-white px-5 py-4 font-medium shadow-[-12px_0_24px_-18px_rgba(15,23,42,0.22)]'>
                  {t('tokensColumnActions')}
                </th>
              </tr>
            </thead>
            <tbody>
              {filtered.length > 0 ? (
                filtered.map((token) => (
                  <tr key={token.id} className='h-[118px] border-b border-slate-100 text-sm text-slate-700'>
                    <td className='sticky left-0 z-10 w-[192px] border-r border-slate-100 bg-white px-5 py-4 align-middle shadow-[12px_0_24px_-18px_rgba(15,23,42,0.18)]'>
                      <div className='w-[152px] space-y-1 overflow-hidden'>
                        <p className='truncate font-medium text-slate-950'>{token.name}</p>
                      </div>
                    </td>
                    <td className='w-[280px] px-5 py-4 align-middle'>
                      <div className='flex items-center gap-3'>
                        <p className='w-[184px] truncate rounded-xl bg-slate-50 px-3 py-2 font-mono text-xs text-sky-700'>
                          {token.key || '--'}
                        </p>
                        <button
                          type='button'
                          onClick={() => handleCopy(token.id)}
                          disabled={busyId === token.id || token.preview}
                          className={
                            copiedId === token.id
                              ? 'inline-flex h-9 w-9 items-center justify-center rounded-xl border border-emerald-200 bg-emerald-50 text-emerald-700 transition-colors'
                              : 'inline-flex h-9 w-9 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-500 transition-colors hover:bg-slate-50 hover:text-slate-900'
                          }
                          aria-label={copiedId === token.id ? t('tokensCopied') : t('tokensCopy')}
                        >
                          {copiedId === token.id ? (
                            <Check className='h-4 w-4' />
                          ) : (
                            <Copy className='h-4 w-4' />
                          )}
                        </button>
                      </div>
                    </td>
                    <td className='px-5 py-4 align-middle'>
                      <span
                        className={
                          token.active
                            ? 'inline-flex w-[76px] justify-center rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700'
                            : 'inline-flex w-[76px] justify-center rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600'
                        }
                      >
                        {token.statusText}
                      </span>
                    </td>
                    <td className='w-[360px] px-5 py-4 align-middle'>
                      <div className='flex min-h-[72px] flex-col justify-center space-y-2'>
                        <div className='flex items-baseline gap-3 text-sm'>
                          <span className='w-[72px] whitespace-nowrap text-slate-500'>{t('tokensUsageToday')}</span>
                          <span className='font-semibold text-slate-950'>{token.todayQuotaText}</span>
                        </div>
                        <div className='flex items-baseline gap-3 text-sm'>
                          <span className='w-[72px] whitespace-nowrap text-slate-500'>{t('tokensUsageLast30d')}</span>
                          <span className='font-medium text-slate-700'>{token.totalUsageQuotaText}</span>
                        </div>
                        <div className='flex items-baseline gap-3 text-sm'>
                          <span className='w-[72px] whitespace-nowrap text-slate-500'>{t('tokensUsageQuota')}</span>
                          <span className='whitespace-nowrap font-medium text-slate-700'>
                            {token.quotaLimitText}
                          </span>
                        </div>
                        {!token.unlimited ? (
                          <div className='h-2 rounded-full bg-slate-100'>
                            <div
                              className='h-2 rounded-full bg-sky-500'
                              style={{
                                width: `${
                                  token.totalQuota > 0
                                    ? Math.min(100, (token.usedQuota / token.totalQuota) * 100)
                                    : 0
                                }%`,
                              }}
                            />
                          </div>
                        ) : (
                          <div className='h-2 rounded-full bg-transparent' />
                        )}
                      </div>
                    </td>
                    <td className='px-5 py-4 align-middle text-slate-500'>{token.expiresAtText}</td>
                    <td className='px-5 py-4 align-middle text-slate-600'>
                      <div className='inline-flex items-center gap-2 rounded-full bg-slate-100 px-2.5 py-1.5 text-xs font-medium text-slate-700'>
                        <span className='h-2 w-2 rounded-full bg-emerald-500' />
                        {token.group}
                      </div>
                    </td>
                    <td className='whitespace-nowrap px-5 py-4 align-middle text-slate-500'>{token.lastUsedAtText}</td>
                    <td className='whitespace-nowrap px-5 py-4 align-middle text-slate-500'>
                      {token.createdAtText}
                    </td>
                    <td className='sticky right-0 z-10 w-[260px] border-l border-slate-100 bg-white px-5 py-4 align-middle shadow-[-12px_0_24px_-18px_rgba(15,23,42,0.22)]'>
                      <div className='flex w-[220px] flex-nowrap gap-2'>
                        <button
                          type='button'
                          onClick={() => openEditForm(token)}
                          className='secondary-button !w-[72px] !justify-center !rounded-lg !px-3 !py-2'
                        >
                          {t('tokensEdit')}
                        </button>
                        <button
                          type='button'
                          onClick={() => handleToggleStatus(token.id, token.active)}
                          disabled={busyId === token.id}
                          className='secondary-button !w-[84px] !justify-center !rounded-lg !px-3 !py-2'
                        >
                          {token.active ? t('tokensDisable') : t('tokensEnable')}
                        </button>
                        <button
                          type='button'
                          onClick={() => handleDelete(token.id)}
                          disabled={busyId === token.id}
                          className='secondary-button !w-[72px] !justify-center !rounded-lg !border-rose-200 !px-3 !py-2 !text-rose-600 hover:!bg-rose-50'
                        >
                          {t('tokensDelete')}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={9} className='px-5 py-10 text-center text-sm text-slate-500'>
                    {t('tokensEmpty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <div className='border-t border-slate-200 bg-slate-50/70 px-4 py-3'>
          <input
            type='range'
            min={0}
            max={Math.max(1, scrollState.max)}
            value={Math.min(scrollState.left, Math.max(1, scrollState.max))}
            onChange={(event) => {
              const next = Number(event.target.value);
              if (tableScrollRef.current) {
                tableScrollRef.current.scrollLeft = next;
              }
              setScrollState((current) => ({ ...current, left: next }));
            }}
            className='token-scrollbar w-full'
          />
        </div>
        <div className='flex flex-col gap-3 border-t border-slate-200 bg-white px-4 py-4 xl:flex-row xl:items-center xl:justify-between'>
          <div className='min-w-[160px] whitespace-nowrap text-sm text-slate-500'>
            {t('tokensPaginationSummary')
              .replace('{start}', totalItems === 0 ? '0' : String((page - 1) * pageSize + 1))
              .replace('{end}', String(Math.min(page * pageSize, totalItems)))
              .replace('{total}', String(totalItems))}
          </div>
          <div className='flex flex-wrap items-center gap-2 xl:flex-nowrap'>
            <label className='whitespace-nowrap text-sm text-slate-500'>{t('tokensPaginationPerPage')}</label>
            <select
              value={pageSize}
              onChange={(event) => {
                setPageSize(Number(event.target.value));
                setPage(1);
              }}
              className='input-shell !h-10 !w-[88px] !rounded-xl !px-3 !py-2 text-sm'
            >
              {[20, 50, 100].map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
            <button
              type='button'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page <= 1}
              className='secondary-button !w-[68px] !justify-center !rounded-xl !px-3 !py-2 disabled:opacity-40'
            >
              {t('tokensPaginationPrev')}
            </button>
            <div className='flex items-center gap-2'>
              <select
                value={page}
                onChange={(event) => setPage(Number(event.target.value))}
                className='input-shell !h-10 !w-[72px] !rounded-xl !px-3 !py-2 text-sm'
              >
                {Array.from({ length: totalPages }, (_, index) => index + 1).map((pageNumber) => (
                  <option key={pageNumber} value={pageNumber}>
                    {pageNumber}
                  </option>
                ))}
              </select>
              <div className='min-w-[32px] whitespace-nowrap text-center text-sm font-medium text-slate-700'>
                / {totalPages}
              </div>
            </div>
            <button
              type='button'
              onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
              disabled={page >= totalPages}
              className='secondary-button !w-[68px] !justify-center !rounded-xl !px-3 !py-2 disabled:opacity-40'
            >
              {t('tokensPaginationNext')}
            </button>
          </div>
        </div>
      </section>

      {isFormOpen ? (
        <div className='fixed inset-0 z-50 grid place-items-center bg-slate-950/30 p-4'>
          <div className='w-full max-w-lg rounded-[28px] border border-slate-200 bg-white p-6 shadow-2xl'>
            <div className='flex items-start justify-between gap-4'>
              <div>
                <h2 className='text-xl font-semibold text-slate-950'>
                  {isEditing ? t('tokensFormEditTitle') : t('tokensFormCreateTitle')}
                </h2>
              </div>
              <button
                type='button'
                onClick={() => setIsFormOpen(false)}
                className='secondary-button !rounded-lg !px-3 !py-2'
              >
                {t('tokensCancel')}
              </button>
            </div>

            <div className='mt-6 grid gap-4 sm:grid-cols-2'>
              <div className='space-y-2 sm:col-span-2'>
                <label className='text-sm font-medium text-slate-700'>{t('tokensFormName')}</label>
                <input
                  value={form.name}
                  onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                  placeholder={t('tokensFormNamePlaceholder')}
                  className='input-shell'
                />
              </div>

              <div className='space-y-2'>
                <label className='block min-h-[40px] text-sm font-medium leading-5 text-slate-700'>
                  {t('tokensFormQuota')} ({t('tokensFormQuotaHint')})
                </label>
                <div className='relative'>
                  <span className='pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-sm text-slate-400'>
                    $
                  </span>
                  <input
                    type='number'
                    min='0'
                    step='0.0001'
                    value={form.remain_quota}
                    onChange={(event) =>
                      setForm((current) => ({ ...current, remain_quota: event.target.value }))
                    }
                    className='input-shell pl-8'
                  />
                  <span className='pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-sm text-slate-400'>
                    USD
                  </span>
                </div>
              </div>

              <div className='space-y-2 sm:col-span-2'>
                <label className='text-sm font-medium text-slate-700'>{t('tokensFormExpires')}</label>
                <div className='grid grid-cols-2 gap-2 lg:grid-cols-5'>
                  {(
                    [
                      ['never', t('tokensFormNever')],
                      ['7', t('tokensFormPreset7d')],
                      ['30', t('tokensFormPreset30d')],
                      ['90', t('tokensFormPreset90d')],
                      ['custom', t('tokensFormPresetCustom')],
                    ] as const
                  ).map(([value, label]) => (
                    <button
                      key={value}
                      type='button'
                      onClick={() =>
                        setForm((current) => ({
                          ...current,
                          expires_mode: value,
                          expires_in_days:
                            value === '7' || value === '30' || value === '90'
                              ? value
                              : value === 'never'
                                ? ''
                                : current.expires_in_days,
                          expires_at_input:
                            value === 'custom'
                              ? current.expires_at_input
                              : value === 'never'
                                ? ''
                                : formatDateTimeInput(
                                    Math.floor(Date.now() / 1000) +
                                      Number(value) * 24 * 60 * 60,
                                  ),
                        }))
                      }
                      className={
                        form.expires_mode === value
                          ? 'rounded-xl bg-emerald-50 px-4 py-2 text-sm font-medium text-emerald-700'
                          : 'rounded-xl bg-slate-100 px-4 py-2 text-sm font-medium text-slate-600'
                      }
                    >
                      <span className='whitespace-nowrap'>{label}</span>
                    </button>
                  ))}
                </div>

                {form.expires_mode !== 'never' ? (
                  <div className='rounded-2xl border border-slate-200 bg-slate-50/60 p-4'>
                    <div className='space-y-2'>
                      <label className='text-sm font-medium text-slate-700'>
                        {t('tokensFormExpiresAt')}
                      </label>
                      {form.expires_mode === 'custom' ? (
                        <input
                          type='datetime-local'
                          value={form.expires_at_input}
                          onChange={(event) =>
                            setForm((current) => ({
                              ...current,
                              expires_at_input: event.target.value,
                            }))
                          }
                          className='input-shell bg-white'
                        />
                      ) : (
                        <div className='input-shell bg-white text-slate-600'>
                          {formatDateTimeInput(
                            Math.floor(Date.now() / 1000) +
                              Number(form.expires_mode || 0) * 24 * 60 * 60,
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                ) : null}
              </div>
            </div>

            <div className='mt-6 flex justify-end gap-3'>
              <button
                type='button'
                onClick={() => setIsFormOpen(false)}
                className='secondary-button !rounded-lg !px-4 !py-2.5'
              >
                {t('tokensCancel')}
              </button>
              <button
                type='button'
                onClick={() => handleSubmit()}
                disabled={submitting}
                className='primary-button !px-4 !py-2.5 disabled:opacity-60'
              >
                {isEditing ? t('tokensSave') : t('tokensCreate')}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
