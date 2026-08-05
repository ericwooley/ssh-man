import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'
import HostApp from './HostApp'

function createApi({
  discovery = {
    needsPassphrase: false,
    ports: [
      { port: 3000, addresses: ['127.0.0.1'], suggestedScheme: 'http' },
      { port: 8443, addresses: ['0.0.0.0'], suggestedScheme: 'https' },
    ],
  },
  links = [
    { id: 'link-1', serverId: 'server-1', port: 3000, name: 'Admin', scheme: 'http', faviconDataUrl: '' },
  ],
} = {}) {
  return {
    initialState: vi.fn(async () => ({
      server: { id: 'server-1', name: 'Production', host: 'prod.example.com', port: 22, username: 'deploy' },
      links,
    })),
    discoverPorts: vi.fn(async () => discovery),
    savePortLink: vi.fn(async (link) => ({ ...link, id: link.id || 'link-new', serverId: 'server-1' })),
    deletePortLink: vi.fn(async () => undefined),
    openPort: vi.fn(async (port, scheme) => ({ url: `${scheme}://127.0.0.1:${port + 10000}` })),
    openExternalURL: vi.fn(async () => undefined),
    close: vi.fn(async () => undefined),
  }
}

describe('HostApp', () => {
  test('lists saved and available ports, then opens a saved link', async () => {
    const user = userEvent.setup()
    const api = createApi()
    render(<HostApp api={api} />)

    expect(await screen.findByRole('heading', { name: 'Production' })).toBeTruthy()
    expect(screen.getByText('Admin')).toBeTruthy()
    expect(screen.getByText('Port 8443')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Open Admin' }))
    await waitFor(() => expect(api.openPort).toHaveBeenCalledWith(3000, 'http'))
    expect(api.openExternalURL).toHaveBeenCalledWith('http://127.0.0.1:13000')
  })

  test('names an available port for later use', async () => {
    const user = userEvent.setup()
    const api = createApi({ links: [] })
    render(<HostApp api={api} />)
    await screen.findByText('Port 3000')

    await user.click(screen.getByRole('button', { name: 'Name port 3000' }))
    const name = screen.getByLabelText('Name')
    await user.clear(name)
    await user.type(name, 'Preview app')
    await user.click(screen.getByRole('button', { name: 'Save link' }))

    await waitFor(() => expect(api.savePortLink).toHaveBeenCalledWith(expect.objectContaining({
      port: 3000,
      name: 'Preview app',
      scheme: 'http',
    })))
    expect(await screen.findByText('Preview app')).toBeTruthy()
  })

  test('asks for the key passphrase and retries discovery', async () => {
    const api = createApi({ discovery: { needsPassphrase: true, ports: [] } })
    api.discoverPorts
      .mockResolvedValueOnce({ needsPassphrase: true, ports: [] })
      .mockResolvedValueOnce({ needsPassphrase: false, ports: [{ port: 8080, addresses: ['127.0.0.1'], suggestedScheme: 'http' }] })
    render(<HostApp api={api} />)

    const passphrase = await screen.findByLabelText('SSH key passphrase')
    fireEvent.change(passphrase, { target: { value: 'secret' } })
    fireEvent.click(screen.getByRole('button', { name: 'Unlock host' }))

    expect(await screen.findByText('Port 8080')).toBeTruthy()
    expect(api.discoverPorts).toHaveBeenLastCalledWith('secret')
  })

  test('shows a useful empty result after discovery', async () => {
    const api = createApi({ discovery: { needsPassphrase: false, ports: [] }, links: [] })
    render(<HostApp api={api} />)
    expect(await screen.findByText('No listening ports found')).toBeTruthy()
  })
})
