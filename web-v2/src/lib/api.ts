import axios from 'axios';

type StoredUser = {
  id?: number | string;
};

function getStoredUserId() {
  const raw = window.localStorage.getItem('user');
  if (!raw) return '';

  try {
    const user = JSON.parse(raw) as StoredUser;
    return user.id ? String(user.id) : '';
  } catch {
    return '';
  }
}

function getRequestLanguage() {
  return window.localStorage.getItem('web-v2-locale') || 'en';
}

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  withCredentials: true,
  headers: {
    'Cache-Control': 'no-store',
    'New-API-User': getStoredUserId(),
    'Accept-Language': getRequestLanguage(),
  },
});

api.interceptors.request.use((config) => {
  config.headers = config.headers || {};
  config.headers['New-API-User'] = getStoredUserId();
  config.headers['Accept-Language'] = getRequestLanguage();
  return config;
});

export type StatusPayload = {
  success?: boolean;
  data?: {
    system_name?: string;
    server_address?: string;
    docs_link?: string;
    logo?: string;
    version?: string;
    passkey_login?: boolean;
    setup?: boolean;
    user_agreement_enabled?: boolean;
    privacy_policy_enabled?: boolean;
    quota_per_unit?: number;
    quota_display_type?: 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM';
    custom_currency_symbol?: string;
    custom_currency_exchange_rate?: number;
    usd_exchange_rate?: number;
  };
};

export async function fetchStatus() {
  const response = await api.get<StatusPayload>('/api/status');
  const payload = response.data;

  if (payload.success === false) {
    throw new Error('Failed to load status');
  }

  return payload.data || {};
}
