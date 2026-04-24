import DataTableShell from '../components/ui/DataTableShell';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchTokens } from '../lib/tokens';

const columns = [
  { key: 'name', label: 'Token' },
  { key: 'group', label: 'Group' },
  { key: 'status', label: 'Status' },
  { key: 'quota', label: 'Quota' },
];

export default function Tokens() {
  const tokens = useAsyncData(fetchTokens, []);
  const rows =
    tokens.data?.map((token) => ({
      name: token.name || `Token #${token.id}`,
      group: token.model_limits_enabled ? 'restricted' : 'default',
      status: token.status === 1 ? 'active' : 'inactive',
      quota: token.unlimited_quota
        ? 'Unlimited'
        : String(token.remain_quota ?? 0),
    })) || [];

  return (
    <div className='space-y-6'>
      <StatePanel
        loading={tokens.loading}
        error={tokens.error}
        empty={!tokens.loading && !tokens.error && rows.length === 0}
        title='Preparing token workspace'
        description='This phase-1 page now reads from the current `/api/token` endpoint and will gain richer actions next.'
      />
      {rows.length > 0 ? (
        <DataTableShell
          title='Token workspace'
          description='A cleaner token management surface for phase 1, now seeded from the existing New API token list endpoint.'
          actionLabel='Create token'
          columns={columns}
          rows={rows}
        />
      ) : null}
    </div>
  );
}
