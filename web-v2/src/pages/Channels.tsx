import DataTableShell from '../components/ui/DataTableShell';
import StatePanel from '../components/ui/StatePanel';
import { useAsyncData } from '../hooks/useAsyncData';
import { fetchChannels } from '../lib/channels';

const columns = [
  { key: 'name', label: 'Channel' },
  { key: 'type', label: 'Type' },
  { key: 'status', label: 'Status' },
  { key: 'models', label: 'Models' },
];

export default function Channels() {
  const channels = useAsyncData(fetchChannels, []);
  const rows =
    channels.data?.map((channel) => ({
      name: channel.name || `Channel #${channel.id}`,
      type: String(channel.type ?? 'unknown'),
      status: channel.status === 1 ? 'active' : 'inactive',
      models: channel.models || channel.model_mapping || 'N/A',
    })) || [];

  return (
    <div className='space-y-6'>
      <StatePanel
        loading={channels.loading}
        error={channels.error}
        empty={!channels.loading && !channels.error && rows.length === 0}
        title='Preparing channel operations'
        description='This phase-1 page now reads from the current `/api/channel` endpoint and will gain richer management tools next.'
      />
      {rows.length > 0 ? (
        <DataTableShell
          title='Channel operations'
          description='A fresh operational shell for routing, redundancy, and model provider management, now seeded from the existing channel list endpoint.'
          actionLabel='Add channel'
          columns={columns}
          rows={rows}
        />
      ) : null}
    </div>
  );
}
