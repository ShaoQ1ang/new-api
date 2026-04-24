import { useCallback, useEffect, useState } from 'react';
import type { DependencyList } from 'react';

type AsyncState<T> = {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => Promise<void>;
};

export function useAsyncData<T>(
  loader: () => Promise<T>,
  deps: DependencyList = [],
): AsyncState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await loader();
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, deps);

  useEffect(() => {
    load().catch(() => undefined);
  }, [load]);

  return {
    data,
    loading,
    error,
    reload: load,
  };
}
