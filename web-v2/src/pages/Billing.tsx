import { useEffect, useMemo, useState } from 'react';
import { ArrowUpRight, CreditCard, Loader2, Receipt, Wallet } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { useI18n } from '../i18n/I18nProvider';
import { useAsyncData } from '../hooks/useAsyncData';
import { createEpayTopUp, createStripeTopUp, fetchBillingInfo, fetchTopUpHistory, type BillingChannel, type TopUpRecord } from '../lib/billing';

function formatPaginationSummary(template: string, start: number, end: number, total: number) {
  return template
    .replace('{start}', String(start))
    .replace('{end}', String(end))
    .replace('{total}', String(total));
}

function formatTimestamp(timestamp: number) {
  if (!timestamp) return '--';
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

function getStatusTone(status: string) {
  if (status === 'success') return 'bg-emerald-50 text-emerald-700';
  if (status === 'pending') return 'bg-amber-50 text-amber-700';
  if (status === 'failed' || status === 'expired') return 'bg-rose-50 text-rose-700';
  return 'bg-slate-100 text-slate-700';
}

function getStatusLabel(status: string, t: (key: string) => string) {
  if (status === 'success') return t('billingStatusSuccess');
  if (status === 'pending') return t('billingStatusPending');
  if (status === 'failed') return t('billingStatusFailed');
  if (status === 'expired') return t('billingStatusExpired');
  return status || '--';
}

function getPaymentMethodLabel(paymentMethod: string, t: (key: string) => string) {
  if (paymentMethod === 'stripe') return 'Stripe';
  if (paymentMethod === 'creem') return 'Creem';
  if (paymentMethod === 'waffo') return 'Waffo';
  if (paymentMethod === 'waffo_pancake') return 'Waffo Pancake';
  return paymentMethod || t('billingUnknownMethod');
}

function getChannelDisplayName(channel: BillingChannel, locale: 'en' | 'zh') {
  const rawName = (channel.name || '').trim();
  const normalizedType = channel.type.toLowerCase();
  const normalizedName = rawName.toLowerCase();

  if (normalizedType === 'stripe') return 'Stripe';
  if (normalizedType === 'creem') return 'Creem';

  if (normalizedType === 'alipay' || rawName === '支付宝') {
    return locale === 'zh' ? '支付宝' : 'Alipay';
  }

  if (normalizedType === 'wxpay' || normalizedType === 'wechat' || rawName === '微信') {
    return locale === 'zh' ? '微信' : 'WeChat Pay';
  }

  if (normalizedName.startsWith('custom')) {
    return locale === 'zh' ? rawName.replace(/^custom/i, '自定义') : rawName;
  }

  return rawName || channel.type;
}

export default function Billing() {
  const { locale, t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const translate = (key: string) => t(key as never);
  const [amount, setAmount] = useState('20');
  const [selectedChannel, setSelectedChannel] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [checkoutLoading, setCheckoutLoading] = useState(false);
  const [actionError, setActionError] = useState('');

  const billingInfo = useAsyncData(fetchBillingInfo, []);
  const history = useAsyncData(() => fetchTopUpHistory(page, pageSize), [page, pageSize]);

  const channels = billingInfo.data?.channels || [];
  const presetAmounts = billingInfo.data?.amount_options?.length ? billingInfo.data.amount_options : [10, 20, 50, 100];
  const activeChannel = channels.find((channel) => channel.type === selectedChannel) || channels[0];
  const minTopUp = Math.max(1, Number(activeChannel?.minTopUp || 1));

  useEffect(() => {
    if (!channels.length) {
      if (selectedChannel) setSelectedChannel('');
      return;
    }

    const stripe = channels.find((channel) => channel.type === 'stripe');
    const nextSelected = stripe?.type || channels[0]?.type || '';
    const stillExists = channels.some((channel) => channel.type === selectedChannel);
    if (!stillExists) {
      setSelectedChannel(nextSelected);
    }
  }, [channels, selectedChannel]);

  useEffect(() => {
    if (Number(amount) < minTopUp) {
      setAmount(String(minTopUp));
    }
  }, [minTopUp]);

  const rows = history.data?.items || [];
  const total = history.data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const paginationSummary = formatPaginationSummary(
    translate('billingPaginationSummary'),
    total === 0 ? 0 : (page - 1) * pageSize + 1,
    Math.min(page * pageSize, total),
    total,
  );

  const metrics = useMemo(() => {
    const successCount = rows.filter((item) => item.status === 'success').length;
    const recentSpend = rows
      .filter((item) => item.status === 'success')
      .reduce((sum, item) => sum + Number(item.money || 0), 0);

    return {
      totalOrders: total,
      successCount,
      recentSpend,
    };
  }, [rows, total]);

  useEffect(() => {
    if (searchParams.get('show_history') !== 'true') return;

    setPage(1);
    history.reload().catch(() => undefined);
    const nextParams = new URLSearchParams(searchParams);
    nextParams.delete('show_history');
    setSearchParams(nextParams, { replace: true });
  }, [history, searchParams, setSearchParams]);

  function shouldUseSameTabPaymentRedirect(userAgent = navigator?.userAgent) {
    return /Android|iPhone|iPad|iPod|Mobile|Windows Phone|HarmonyOS/i.test(typeof userAgent === 'string' ? userAgent : '');
  }

  function redirectToPaymentUrl(url: string, userAgent = navigator?.userAgent) {
    if (!url || typeof window === 'undefined') return;

    if (shouldUseSameTabPaymentRedirect(userAgent)) {
      window.location.assign(url);
      return;
    }

    window.open(url, '_blank');
  }

  function submitPaymentForm(url: string, params: Record<string, string>) {
    const form = document.createElement('form');
    form.action = url;
    form.method = 'POST';
    form.target = '_blank';

    Object.entries(params).forEach(([key, value]) => {
      const input = document.createElement('input');
      input.type = 'hidden';
      input.name = key;
      input.value = value;
      form.appendChild(input);
    });

    document.body.appendChild(form);
    form.submit();
    document.body.removeChild(form);
  }

  async function handleCheckout() {
    const numericAmount = Number(amount);
    if (!Number.isFinite(numericAmount) || numericAmount < minTopUp) {
      setActionError(t('billingMinTopUpError').replace('{min}', String(minTopUp)));
      return;
    }
    if (!activeChannel) {
      setActionError(t('billingNoChannel'));
      return;
    }

    setCheckoutLoading(true);
    setActionError('');
    try {
      if (activeChannel.type === 'stripe') {
        const origin = window.location.origin;
        const payLink = await createStripeTopUp(numericAmount, {
          successUrl: `${origin}/console/billing?show_history=true`,
          cancelUrl: `${origin}/console/billing`,
        });
        redirectToPaymentUrl(payLink);
      } else {
        const payment = await createEpayTopUp(numericAmount, activeChannel.type);
        submitPaymentForm(payment.url, payment.params);
      }
      history.reload().catch(() => undefined);
    } catch (error: any) {
      setActionError(error?.message || t('billingCheckoutFailed'));
    } finally {
      setCheckoutLoading(false);
    }
  }

  return (
    <div className='space-y-5'>
      <section className='grid gap-3 md:grid-cols-3'>
        <article className='min-h-[108px] rounded-[22px] border border-slate-200 bg-white px-4 py-4 shadow-sm'>
          <div className='flex items-center justify-between'>
            <div>
              <p className='min-h-[14px] whitespace-nowrap text-[11px] uppercase tracking-[0.18em] text-slate-400'>{t('billingMetricOrders')}</p>
              <p className='mt-2 text-[28px] font-semibold leading-none text-slate-950'>{metrics.totalOrders.toLocaleString()}</p>
            </div>
            <Receipt className='h-5 w-5 text-slate-400' />
          </div>
        </article>
        <article className='min-h-[108px] rounded-[22px] border border-slate-200 bg-white px-4 py-4 shadow-sm'>
          <div className='flex items-center justify-between'>
            <div>
              <p className='min-h-[14px] whitespace-nowrap text-[11px] uppercase tracking-[0.18em] text-slate-400'>{t('billingMetricSuccessful')}</p>
              <p className='mt-2 text-[28px] font-semibold leading-none text-slate-950'>{metrics.successCount.toLocaleString()}</p>
            </div>
            <CreditCard className='h-5 w-5 text-slate-400' />
          </div>
        </article>
        <article className='min-h-[108px] rounded-[22px] border border-slate-200 bg-white px-4 py-4 shadow-sm'>
          <div className='flex items-center justify-between'>
            <div>
              <p className='min-h-[14px] whitespace-nowrap text-[11px] uppercase tracking-[0.18em] text-slate-400'>{t('billingMetricPaid')}</p>
              <p className='mt-2 text-[28px] font-semibold leading-none text-slate-950'>${metrics.recentSpend.toFixed(2)}</p>
            </div>
            <Wallet className='h-5 w-5 text-slate-400' />
          </div>
        </article>
      </section>

      <section className='grid gap-5 xl:grid-cols-[440px_minmax(0,1fr)] xl:items-stretch'>
        <article className='flex h-full flex-col rounded-[24px] border border-slate-200 bg-white p-5 shadow-sm'>
          <div>
            <h2 className='text-lg font-semibold text-slate-950'>{t('billingCheckoutTitle')}</h2>
            <p className='mt-1 text-sm leading-6 text-slate-500'>{t('billingCheckoutDescription')}</p>
          </div>

          <div className='mt-6 space-y-4'>
            <div className='grid grid-cols-2 gap-2'>
              {presetAmounts.slice(0, 6).map((preset) => {
                const active = Number(amount) === preset;
                return (
                  <button
                    key={preset}
                    type='button'
                    onClick={() => setAmount(String(preset))}
                    className={
                      active
                        ? 'rounded-2xl border border-slate-950 bg-slate-950 px-4 py-3 text-sm font-medium text-white'
                        : 'rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm font-medium text-slate-700'
                    }
                  >
                    ${preset}
                  </button>
                );
              })}
            </div>

            <div className='space-y-2'>
              <label className='text-sm font-medium text-slate-700'>{t('billingAmount')}</label>
              <div className='flex items-center rounded-2xl border border-slate-200 bg-white px-4'>
                <span className='text-sm font-medium text-slate-400'>$</span>
                <input
                  type='number'
                  min={minTopUp}
                  step='1'
                  value={amount}
                  onChange={(event) => setAmount(event.target.value)}
                  className='h-12 w-full bg-transparent px-2 text-base text-slate-950 outline-none'
                />
              </div>
              <p className='text-xs text-slate-500'>{t('billingMinTopUpHint').replace('{min}', String(minTopUp))}</p>
            </div>

            <div className='space-y-2'>
              <label className='text-sm font-medium text-slate-700'>{t('billingChannel')}</label>
              {channels.length ? (
                <div className='space-y-2'>
                  {channels.map((channel: BillingChannel, index) => {
                    const active = channel.type === activeChannel?.type;
                    return (
                      <button
                        key={`${channel.type}-${index}`}
                        type='button'
                        onClick={() => setSelectedChannel(channel.type)}
                        className={
                          active
                            ? 'flex w-full items-center justify-between rounded-2xl border border-slate-300 bg-slate-100 px-4 py-3 text-left text-sm font-medium text-slate-950'
                            : 'flex w-full items-center justify-between rounded-2xl border border-slate-200 bg-white px-4 py-3 text-left text-sm font-medium text-slate-700'
                        }
                      >
                        <span className='flex items-center gap-3'>
                          <span
                            className={
                              active
                                ? 'h-2.5 w-2.5 rounded-full bg-slate-900'
                                : 'h-2.5 w-2.5 rounded-full border border-slate-300 bg-white'
                            }
                          />
                          <span>{getChannelDisplayName(channel, locale)}</span>
                        </span>
                        {active ? (
                          <span className='inline-flex h-7 w-[96px] items-center justify-center rounded-full bg-white px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500'>
                            {t('billingChannelSelected')}
                          </span>
                        ) : (
                          <span className='inline-flex h-7 w-[96px] items-center justify-center text-[11px] font-medium uppercase tracking-[0.16em] text-slate-300'>
                            {t('billingChannelAvailable')}
                          </span>
                        )}
                      </button>
                    );
                  })}
                </div>
              ) : (
                <div className='flex h-12 items-center rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-4 text-sm text-slate-500'>
                  {t('billingNoChannel')}
                </div>
              )}
            </div>

            {actionError ? <div className='rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700'>{actionError}</div> : null}

            <button
              type='button'
              disabled={!activeChannel || checkoutLoading || billingInfo.loading}
              onClick={handleCheckout}
              className='inline-flex w-full items-center justify-center gap-2 rounded-2xl bg-slate-950 px-5 py-3 text-sm font-semibold text-white disabled:opacity-50'
            >
              {checkoutLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : <ArrowUpRight className='h-4 w-4' />}
              {checkoutLoading ? t('billingRedirecting') : t('billingContinueCheckout')}
            </button>
          </div>
        </article>

        <article className='overflow-hidden rounded-[24px] border border-slate-200 bg-white shadow-sm'>
          <div className='border-b border-slate-200 px-5 py-4'>
            <h2 className='text-lg font-semibold text-slate-950'>{t('billingHistoryTitle')}</h2>
            <p className='mt-1 text-sm leading-6 text-slate-500'>{t('billingHistoryDescription')}</p>
          </div>

          <div className='overflow-hidden'>
            <div className='overflow-x-auto'>
            <table className='min-w-full table-fixed'>
              <colgroup>
                <col className='w-[260px]' />
                <col className='w-[120px]' />
                <col className='w-[110px]' />
                <col className='w-[120px]' />
                <col className='w-[120px]' />
                <col className='w-[180px]' />
              </colgroup>
              <thead>
                <tr className='border-b border-slate-200 text-left text-[11px] uppercase tracking-[0.16em] text-slate-500'>
                  <th className='px-5 py-3 font-medium whitespace-nowrap'>{t('billingTableOrder')}</th>
                  <th className='px-5 py-3 font-medium whitespace-nowrap'>{t('billingTableMethod')}</th>
                  <th className='px-5 py-3 font-medium whitespace-nowrap'>{t('billingTableAmount')}</th>
                  <th className='px-5 py-3 font-medium whitespace-nowrap'>{t('billingTablePaid')}</th>
                  <th className='px-5 py-3 font-medium whitespace-nowrap'>{t('billingTableStatus')}</th>
                  <th className='px-5 py-3 font-medium whitespace-nowrap'>{t('billingTableCreated')}</th>
                </tr>
              </thead>
              <tbody>
                {rows.length ? (
                  rows.map((row: TopUpRecord) => (
                    <tr key={row.id} className='border-b border-slate-100 text-sm text-slate-700'>
                      <td className='px-5 py-4'>
                        <div className='max-w-[240px]'>
                          <p className='truncate font-medium text-slate-950'>{row.trade_no}</p>
                        </div>
                      </td>
                      <td className='px-5 py-4'>{getPaymentMethodLabel(row.payment_method, translate)}</td>
                      <td className='px-5 py-4'>{row.amount.toLocaleString()}</td>
                      <td className='px-5 py-4 font-medium text-slate-950'>${Number(row.money || 0).toFixed(2)}</td>
                      <td className='px-5 py-4'>
                        <span className={`inline-flex h-7 items-center rounded-full px-3 text-xs font-medium ${getStatusTone(row.status)}`}>
                          {getStatusLabel(row.status, translate)}
                        </span>
                      </td>
                      <td className='whitespace-nowrap px-5 py-4 text-slate-500'>{formatTimestamp(row.create_time)}</td>
                    </tr>
                  ))
                ) : null}
              </tbody>
            </table>
            </div>
          </div>

          {!rows.length ? (
            <div className='flex min-h-[180px] items-center justify-center border-t border-slate-100 px-5 py-10 text-center text-sm text-slate-500'>
              {history.loading ? t('billingLoading') : t('billingEmpty')}
            </div>
          ) : null}

          {rows.length ? (
            <div className='flex flex-col gap-3 border-t border-slate-200 bg-white px-4 py-3 xl:flex-row xl:items-center xl:justify-between'>
              <div className='min-w-[220px] whitespace-nowrap text-sm text-slate-500'>{paginationSummary}</div>

              <div className='flex items-center gap-2'>
                <label className='whitespace-nowrap text-sm text-slate-500'>{t('billingPaginationPerPage')}</label>
                <select
                  value={String(pageSize)}
                  onChange={(event) => {
                    setPageSize(Number(event.target.value));
                    setPage(1);
                  }}
                  className='input-shell !h-10 !w-[72px] !rounded-xl !px-3 !py-2 text-sm'
                >
                  {[10, 20, 50, 100].map((size) => (
                    <option key={size} value={size}>
                      {size}
                    </option>
                  ))}
                </select>
              </div>

              <div className='flex flex-wrap items-center gap-0 overflow-hidden rounded-xl border border-slate-200'>
                <button
                  type='button'
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                  disabled={page === 1}
                  className='inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-white text-sm text-slate-600 disabled:opacity-40'
                >
                  ‹
                </button>
                {Array.from({ length: totalPages }, (_, index) => index + 1)
                  .filter((pageNumber) => {
                    if (totalPages <= 6) return true;
                    return pageNumber === 1 || pageNumber === totalPages || Math.abs(pageNumber - page) <= 1;
                  })
                  .map((pageNumber, index, visiblePages) => {
                    const previous = visiblePages[index - 1];
                    const needsDots = previous && pageNumber - previous > 1;

                    return (
                      <div key={pageNumber} className='contents'>
                        {needsDots ? (
                          <div className='inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-white text-sm text-slate-500'>
                            ...
                          </div>
                        ) : null}
                        <button
                          type='button'
                          onClick={() => setPage(pageNumber)}
                          className={
                            page === pageNumber
                              ? 'inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-emerald-50 text-sm font-medium text-emerald-700'
                              : 'inline-flex h-10 w-10 items-center justify-center border-r border-slate-200 bg-white text-sm text-slate-600'
                          }
                        >
                          {pageNumber}
                        </button>
                      </div>
                    );
                  })}
                  <button
                    type='button'
                    onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
                    disabled={page === totalPages}
                    className='inline-flex h-10 w-10 items-center justify-center bg-white text-sm text-slate-600 disabled:opacity-40'
                  >
                    ›
                  </button>
              </div>
            </div>
          ) : null}
        </article>
      </section>
    </div>
  );
}
