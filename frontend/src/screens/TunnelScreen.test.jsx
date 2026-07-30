import { render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { TunnelDetailScreen } from './TunnelScreen'

const configuration = {
  id: 'server-socks:server-1',
  serverId: 'server-1',
  label: 'Browser proxy',
  connectionType: 'socks_proxy',
  socksPort: 55123,
  autoReconnectEnabled: true,
  startOnLaunch: false,
  notes: '',
}

const session = {
  configurationId: configuration.id,
  status: 'connected',
  boundPort: configuration.socksPort,
  statusDetail: 'Connected.',
}

function tunnelProps(browserState) {
  return {
    configuration,
    session,
    history: [],
    historyLoading: false,
    runtimeFresh: true,
    browserState,
    pending: {},
    onStart: vi.fn(),
    onStop: vi.fn(),
    onRetry: vi.fn(),
    onEdit: vi.fn(),
    onDelete: vi.fn(),
    onCopyHistory: vi.fn(),
    onRefreshBrowsers: vi.fn(),
    onSelectBrowser: vi.fn(),
    onLaunchBrowser: vi.fn(),
    onUnlock: vi.fn(),
    onRefreshRuntime: vi.fn(),
  }
}

describe('SOCKS browser guidance', () => {
  test('directs empty and populated states to the current browser workflow', () => {
    const { rerender } = render(<TunnelDetailScreen {...tunnelProps({
      items: [],
      selectedId: '',
      loading: false,
      preview: '',
    })} />)

    expect(screen.getByText('Enable a proxy-capable installed browser under Settings → Browsers, then refresh.')).toBeTruthy()
    expect(screen.queryByText(/application path/i)).toBeNull()

    rerender(<TunnelDetailScreen {...tunnelProps({
      items: [{ id: 'chrome', displayName: 'Chrome', supportsProxyLaunch: true }],
      selectedId: 'chrome',
      loading: false,
      preview: '',
    })} />)

    expect(screen.getByText('Browser destination')).toBeTruthy()
    expect(screen.queryByText('Installed browser')).toBeNull()
  })
})
