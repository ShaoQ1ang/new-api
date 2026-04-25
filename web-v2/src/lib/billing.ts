import { api } from './api';

export type TopUpInfoResponse = {
  success: boolean;
  message?: string;
  data?: {
    pay_methods?: Array<{
      name?: string;
      type?: string;
      color?: string;
      min_topup?: string | number;
    }> | string;
    enable_stripe_topup?: boolean;
    stripe_min_topup?: number;
    enable_creem_topup?: boolean;
    creem_products?: string;
    amount_options?: number[];
    discount?: Record<string, number>;
  };
};

export type TopUpRecord = {
  id: number;
  amount: number;
  money: number;
  trade_no: string;
  payment_method: string;
  create_time: number;
  complete_time: number;
  status: string;
};

type TopUpHistoryResponse = {
  success: boolean;
  message?: string;
  data?: {
    items?: TopUpRecord[];
    total?: number;
    page?: number;
    page_size?: number;
  };
};

type StripePayResponse = {
  message?: string;
  data?: {
    pay_link?: string;
  };
};

type EpayPayResponse = {
  message?: string;
  url?: string;
  data?: Record<string, string>;
};

export type BillingChannel = {
  name: string;
  type: string;
  minTopUp: number;
  color?: string;
};

export async function fetchBillingInfo() {
  const response = await api.get<TopUpInfoResponse>('/api/user/topup/info');
  if (response.data.success === false) {
    throw new Error(response.data.message || 'Failed to load billing info');
  }

  const raw = response.data.data || {};
  let payMethods = raw.pay_methods || [];

  if (typeof payMethods === 'string') {
    try {
      payMethods = JSON.parse(payMethods);
    } catch {
      payMethods = [];
    }
  }

  const channels: BillingChannel[] = Array.isArray(payMethods)
    ? payMethods
        .filter((method) => method?.name && method?.type)
        .map((method) => {
          const numericMinTopup = Number(method.min_topup);
          const fallbackStripeMin = Number(raw.stripe_min_topup || 0);
          return {
            name: String(method.name),
            type: String(method.type),
            color: method.color,
            minTopUp:
              Number.isFinite(numericMinTopup) && numericMinTopup > 0
                ? numericMinTopup
                : method.type === 'stripe' && Number.isFinite(fallbackStripeMin) && fallbackStripeMin > 0
                  ? fallbackStripeMin
                  : 1,
          };
        })
    : [];

  channels.sort((left, right) => {
    if (left.type === 'stripe' && right.type !== 'stripe') return -1;
    if (left.type !== 'stripe' && right.type === 'stripe') return 1;
    return 0;
  });

  return {
    ...raw,
    channels,
  };
}

export async function fetchTopUpHistory(page: number, pageSize: number) {
  const response = await api.get<TopUpHistoryResponse>(`/api/user/topup/self?p=${page}&page_size=${pageSize}`);
  if (response.data.success === false) {
    throw new Error(response.data.message || 'Failed to load billing history');
  }
  return {
    items: response.data.data?.items || [],
    total: response.data.data?.total || 0,
  };
}

export async function createStripeTopUp(amount: number, options?: { successUrl?: string; cancelUrl?: string }) {
  const response = await api.post<StripePayResponse>('/api/user/stripe/pay', {
    amount,
    payment_method: 'stripe',
    success_url: options?.successUrl,
    cancel_url: options?.cancelUrl,
  });

  if (response.data.message !== 'success' || !response.data.data?.pay_link) {
    throw new Error(typeof response.data.data === 'string' ? response.data.data : response.data.message || 'Failed to create Stripe checkout');
  }

  return response.data.data.pay_link;
}

export async function createEpayTopUp(amount: number, paymentMethod: string) {
  const response = await api.post<EpayPayResponse>('/api/user/pay', {
    amount,
    payment_method: paymentMethod,
  });

  if (response.data.message !== 'success' || !response.data.url || !response.data.data) {
    throw new Error(response.data.message || 'Failed to create payment');
  }

  return {
    url: response.data.url,
    params: response.data.data,
  };
}
