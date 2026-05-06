import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import DataTableShell from './DataTableShell';

describe('DataTableShell', () => {
  it('renders the extracted API keys table shell structure', () => {
    render(
      <DataTableShell>
        <DataTableShell.Header eyebrow='Access' title='API Keys' actions={<button type='button'>Add key</button>} />
        <DataTableShell.Viewport>
          <DataTableShell.Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              <DataTableShell.EmptyRow colSpan={2}>No keys yet</DataTableShell.EmptyRow>
            </tbody>
          </DataTableShell.Table>
        </DataTableShell.Viewport>
        <DataTableShell.Scrollbar max={120} value={40} onChange={() => undefined} />
        <DataTableShell.Footer>
          <div>Pagination slot</div>
        </DataTableShell.Footer>
      </DataTableShell>,
    );

    expect(screen.getByText('API Keys')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add key' })).toBeInTheDocument();
    expect(screen.getByText('No keys yet')).toBeInTheDocument();
    expect(screen.getByText('Pagination slot')).toBeInTheDocument();
  });

  it('forwards horizontal scrollbar changes', () => {
    const handleChange = vi.fn();

    render(<DataTableShell.Scrollbar max={240} value={32} onChange={handleChange} />);

    fireEvent.change(screen.getByRole('slider'), { target: { value: '96' } });

    expect(handleChange).toHaveBeenCalledWith(96);
  });

  it('hides the scrollbar when there is nothing to scroll', () => {
    const { container } = render(
      <DataTableShell.Scrollbar max={0} value={0} onChange={() => undefined} />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});
