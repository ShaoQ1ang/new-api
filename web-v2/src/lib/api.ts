import axios from 'axios';

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  headers: {
    'Cache-Control': 'no-store',
  },
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
