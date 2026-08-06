import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'
import HostApp from './HostApp'

function createDeferred() {
  let resolve
  const promise = new Promise((next) => {
    resolve = next
  })
  return { promise, resolve }
}

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
  theme = 'dark',
} = {}) {
  return {
    initialState: vi.fn(async () => ({
      server: { id: 'server-1', name: 'Production', host: 'prod.example.com', port: 22, username: 'deploy' },
      links,
      theme,
    })),
    discoverPorts: vi.fn(async () => discovery),
    savePortLink: vi.fn(async (link) => ({ ...link, id: link.id || 'link-new', serverId: 'server-1' })),
    deletePortLink: vi.fn(async () => undefined),
    findPortFavicon: vi.fn(async () => ({ faviconDataUrl: 'data:image/png;base64,aWNvbg==' })),
    openPort: vi.fn(async (port, scheme) => ({ url: `${scheme}://127.0.0.1:${port + 10000}` })),
    openExternalURL: vi.fn(async () => undefined),
    close: vi.fn(async () => undefined),
  }
}

afterEach(() => {
  delete document.documentElement.dataset.theme
  document.documentElement.style.colorScheme = ''
})

describe('HostApp', () => {
  test.each(['light', 'dark'])('applies the saved %s theme', async (theme) => {
    const api = createApi({ theme })
    render(<HostApp api={api} />)

    await screen.findByRole('heading', { name: 'Production' })

    expect(document.documentElement.dataset.theme).toBe(theme)
    expect(document.documentElement.style.colorScheme).toBe(theme)
  })

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

  test('keeps focus on a port while it opens', async () => {
    const user = userEvent.setup()
    const request = createDeferred()
    const api = createApi()
    api.openPort.mockReturnValueOnce(request.promise)
    render(<HostApp api={api} />)

    const openButton = await screen.findByRole('button', { name: 'Open Admin' })
    await user.click(openButton)
    await waitFor(() => expect(api.openPort).toHaveBeenCalledWith(3000, 'http'))

    expect(openButton.disabled).toBe(false)
    expect(openButton.getAttribute('aria-disabled')).toBe('true')
    expect(document.activeElement).toBe(openButton)

    await act(async () => {
      request.resolve({ url: 'http://127.0.0.1:13000' })
      await request.promise
    })
    await waitFor(() => expect(openButton.getAttribute('aria-disabled')).toBe('false'))
    expect(document.activeElement).toBe(openButton)
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

  test('finds and saves a favicon for a named port', async () => {
    const user = userEvent.setup()
    const api = createApi({ links: [] })
    render(<HostApp api={api} />)
    await screen.findByText('Port 3000')

    await user.click(screen.getByRole('button', { name: 'Name port 3000' }))
    await user.click(screen.getByRole('button', { name: 'Find favicon' }))
    await waitFor(() => expect(api.findPortFavicon).toHaveBeenCalledWith(3000, 'http'))
    expect(screen.getByRole('img', { name: 'Selected favicon' })).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Save link' }))
    await waitFor(() => expect(api.savePortLink).toHaveBeenCalledWith(expect.objectContaining({
      faviconDataUrl: 'data:image/png;base64,aWNvbg==',
    })))
  })

  test('keeps focus on the favicon action while it searches', async () => {
    const user = userEvent.setup()
    const request = createDeferred()
    const api = createApi({ links: [] })
    api.findPortFavicon.mockReturnValueOnce(request.promise)
    render(<HostApp api={api} />)
    await screen.findByText('Port 3000')

    await user.click(screen.getByRole('button', { name: 'Name port 3000' }))
    const findButton = screen.getByRole('button', { name: 'Find favicon' })
    await user.click(findButton)
    await waitFor(() => expect(api.findPortFavicon).toHaveBeenCalledWith(3000, 'http'))

    expect(findButton.disabled).toBe(false)
    expect(findButton.getAttribute('aria-disabled')).toBe('true')
    expect(document.activeElement).toBe(findButton)

    await act(async () => {
      request.resolve({ faviconDataUrl: 'data:image/png;base64,aWNvbg==' })
      await request.promise
    })
    await waitFor(() => expect(findButton.getAttribute('aria-disabled')).toBe('false'))
    expect(document.activeElement).toBe(findButton)
  })

  test('restores focus after the link editor closes and saves', async () => {
    const user = userEvent.setup()
    const api = createApi({ links: [] })
    render(<HostApp api={api} />)
    await screen.findByText('Port 3000')

    const nameButton = screen.getByRole('button', { name: 'Name port 3000' })
    await user.click(nameButton)
    await user.click(screen.getByRole('button', { name: 'Close link editor' }))
    await waitFor(() => expect(document.activeElement).toBe(nameButton))

    await user.click(nameButton)
    await user.click(screen.getByRole('button', { name: 'Save link' }))
    const editButton = await screen.findByRole('button', { name: 'Edit Port 3000' })
    await waitFor(() => expect(document.activeElement).toBe(editButton))
  })

  test('restores focus after a saved link is removed', async () => {
    const user = userEvent.setup()
    const api = createApi()
    render(<HostApp api={api} />)
    await screen.findByText('Admin')

    await user.click(screen.getByRole('button', { name: 'Edit Admin' }))
    await user.click(screen.getByRole('button', { name: 'Remove' }))

    const nameButton = await screen.findByRole('button', { name: 'Name port 3000' })
    await waitFor(() => expect(document.activeElement).toBe(nameButton))
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

  test('retries the complete startup after the initial state fails', async () => {
    const user = userEvent.setup()
    const api = createApi({ links: [] })
    api.initialState
      .mockRejectedValueOnce(new Error('Temporary startup failure'))
      .mockResolvedValueOnce({
        server: { id: 'server-1', name: 'Production', host: 'prod.example.com', port: 22, username: 'deploy' },
        links: [],
        theme: 'light',
      })
    render(<HostApp api={api} />)

    expect(await screen.findByText('Temporary startup failure')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByRole('heading', { name: 'Production' })).toBeTruthy()
    expect(await screen.findByText('Port 3000')).toBeTruthy()
    expect(api.initialState).toHaveBeenCalledTimes(2)
    expect(api.discoverPorts).toHaveBeenCalledTimes(1)
  })
})
