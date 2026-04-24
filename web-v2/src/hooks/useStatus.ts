import { useAsyncData } from './useAsyncData';
import { fetchStatus } from '../lib/api';

export function useStatus() {
  return useAsyncData(fetchStatus, []);
}
