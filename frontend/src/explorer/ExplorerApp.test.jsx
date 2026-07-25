import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import ExplorerApp from './ExplorerApp'

vi.mock('./MonacoPreview', () => ({
  default: ({ content, label, onChange, onSave, vimMode }) => (
    <div data-testid="monaco-editor" data-vim={vimMode ? 'enabled' : 'disabled'}>
      <textarea aria-label={label} value={content} onChange={(event) => onChange?.(event.target.value)} />
      <button type="button" onClick={() => onSave?.(content)}>Editor save</button>
    </div>
  ),
}))

function fakeApi() {
  const previewWindowListeners = new Set()
  const fileDropListeners = new Set()
  const uploadProgressListeners = new Set()
  const directories = {
    '/home/deploy': {
      path: '/home/deploy',
      entries: [
        { name: 'site', path: '/home/deploy/site', kind: 'directory', size: 0, modifiedAt: '2026-07-22T12:00:00Z' },
        { name: 'README.md', path: '/home/deploy/README.md', kind: 'file', size: 17, modifiedAt: '2026-07-22T12:00:00Z' },
      ],
    },
    '/home/deploy/site': { path: '/home/deploy/site', entries: [] },
  }
  const api = {
    initialState: vi.fn(async () => ({ server: { id: 'server-1', name: 'Production', username: 'deploy', host: 'prod.example.com' } })),
    connect: vi.fn(async () => ({ connected: true, homePath: '/home/deploy' })),
    listDirectory: vi.fn(async (path) => directories[path]),
    previewFile: vi.fn(async () => ({
      path: '/home/deploy/README.md',
      name: 'README.md',
      kind: 'markdown',
      mimeType: 'text/markdown',
      content: '# Production\n\nRemote notes.',
      size: 17,
      revision: 'revision-1',
    })),
    saveFile: vi.fn(async (path, content) => ({
      path,
      name: 'README.md',
      kind: 'markdown',
      mimeType: 'text/markdown',
      content,
      size: content.length,
      revision: 'revision-2',
    })),
    download: vi.fn(async () => ['/Users/eric/Downloads/README.md']),
    uploadFiles: vi.fn(async (_uploadID, remoteDirectory, localPaths) => ({
      uploaded: localPaths.map((localPath) => `${remoteDirectory}/${localPath.split('/').at(-1)}`),
      failures: [],
    })),
    openPreview: vi.fn(async (remotePath) => ({ remotePath, open: true })),
    focusPreview: vi.fn(async (remotePath) => ({ remotePath, open: true })),
    previewWindowState: vi.fn(async (remotePath) => ({ remotePath, open: false })),
    subscribePreviewWindowState: vi.fn((listener) => {
      previewWindowListeners.add(listener)
      return () => previewWindowListeners.delete(listener)
    }),
    subscribeFileDrop: vi.fn((listener) => {
      fileDropListeners.add(listener)
      return () => fileDropListeners.delete(listener)
    }),
    subscribeUploadProgress: vi.fn((listener) => {
      uploadProgressListeners.add(listener)
      return () => uploadProgressListeners.delete(listener)
    }),
    openExternalURL: vi.fn(async () => undefined),
    emitPreviewWindowState(state) {
      previewWindowListeners.forEach((listener) => listener(state))
    },
    emitFileDrop(paths) {
      fileDropListeners.forEach((listener) => listener(120, 200, paths))
    },
    emitUploadProgress(progress) {
      uploadProgressListeners.forEach((listener) => listener(progress))
    },
  }
  return api
}

async function waitForExplorerReady() {
  await screen.findByDisplayValue('/home/deploy')
  await waitFor(() => {
    expect(document.querySelector('.explorer-browser')?.style.getPropertyValue('--wails-drop-target')).toBe('drop')
  })
}

describe('server explorer window', () => {
  beforeEach(() => window.localStorage.clear())

  test('uses browser-compatible local storage', () => {
    expect(typeof window.localStorage.clear).toBe('function')
    expect(typeof window.localStorage.getItem).toBe('function')
  })

  test('keeps the MoonPixels link visible in the explorer sidebar', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('button', { name: 'Visit MoonPixels' }))

    expect(api.openExternalURL).toHaveBeenCalledWith('https://moonpixels.tech')
  })

  test('browses folders and renders remote markdown', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    expect(await screen.findByDisplayValue('/home/deploy')).toBeTruthy()
    await user.click(screen.getByRole('option', { name: /README\.md/ }))

    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
    expect(screen.getByText('Remote notes.')).toBeTruthy()

    await user.dblClick(screen.getByRole('option', { name: /site/ }))
    await waitFor(() => expect(api.listDirectory).toHaveBeenLastCalledWith('/home/deploy/site'))
    expect(await screen.findByText('This folder is empty')).toBeTruthy()
  })

  test('renders video previews with a full-size native video player', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.listDirectory.mockImplementation(async () => ({
      path: '/home/deploy',
      entries: [
        { name: 'capture.mp4', path: '/home/deploy/capture.mp4', kind: 'file', size: 1400000, modifiedAt: '2026-07-22T12:00:00Z' },
      ],
    }))
    api.previewFile.mockResolvedValue({
      path: '/home/deploy/capture.mp4',
      name: 'capture.mp4',
      kind: 'video',
      mimeType: 'video/mp4',
      size: 1400000,
    })
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /capture\.mp4/ }))

    const player = await screen.findByLabelText('capture.mp4 video preview')
    expect(player.tagName).toBe('VIDEO')
    expect(player.hasAttribute('controls')).toBe(true)
    expect(player.getAttribute('src')).toBe('/__ssh_man_remote__/raw/home/deploy/capture.mp4')
    expect(screen.getByText('Video preview')).toBeTruthy()
    expect(screen.queryByTitle('capture.mp4 browser preview')).toBeNull()
  })

  test('focuses a detached preview and restores it after the window closes', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
    await user.click(await screen.findByRole('button', { name: 'Open README.md preview in new window' }))

    await waitFor(() => expect(api.openPreview).toHaveBeenCalledWith('/home/deploy/README.md'))
    expect(await screen.findByText('Preview open in window')).toBeTruthy()
    expect(screen.queryByRole('heading', { name: 'Production', level: 1 })).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Focus preview window for README.md' }))
    expect(api.openPreview).toHaveBeenCalledTimes(1)
    expect(api.focusPreview).toHaveBeenCalledWith('/home/deploy/README.md')

    act(() => {
      api.emitPreviewWindowState({ remotePath: '/home/deploy/README.md', open: false })
    })

    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
  })

  test('downloads the selected remote file through the native destination flow', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(screen.getByRole('button', { name: 'Download' }))

    await waitFor(() => expect(api.download).toHaveBeenCalledWith(['/home/deploy/README.md']))
    expect(await screen.findByText(/Downloaded 1 item/)).toBeTruthy()
  })

  test('uploads dropped local files into the open remote folder and refreshes it', async () => {
    const api = fakeApi()
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt', '/Users/eric/screenshot.png'])
    })

    await waitFor(() => expect(api.uploadFiles).toHaveBeenCalledWith(
      1,
      '/home/deploy',
      ['/Users/eric/report.txt', '/Users/eric/screenshot.png'],
    ))
    await waitFor(() => expect(api.listDirectory).toHaveBeenCalledTimes(2))
    expect(await screen.findByText('Uploaded 2 files to /home/deploy.')).toBeTruthy()
  })

  test('enables the native drop target once the folder is ready', async () => {
    let finishListing
    let finishUpload
    const api = fakeApi()
    api.listDirectory.mockImplementationOnce(() => new Promise((resolve) => {
      finishListing = resolve
    }))
    api.uploadFiles.mockImplementation(() => new Promise((resolve) => {
      finishUpload = resolve
    }))
    const { container } = render(<ExplorerApp api={api} />)
    const browser = container.querySelector('.explorer-browser')
    await waitFor(() => expect(typeof finishListing).toBe('function'))
    expect(browser.style.getPropertyValue('--wails-drop-target')).toBe('')
    act(() => {
      api.emitFileDrop(['/Users/eric/too-early.txt'])
    })
    expect(api.uploadFiles).not.toHaveBeenCalled()

    await act(async () => {
      finishListing({
        path: '/home/deploy',
        entries: [
          { name: 'README.md', path: '/home/deploy/README.md', kind: 'file', size: 17, modifiedAt: '2026-07-22T12:00:00Z' },
        ],
      })
    })
    await waitForExplorerReady()
    expect(browser.style.getPropertyValue('--wails-drop-target')).toBe('drop')

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt'])
    })

    await waitFor(() => expect(api.uploadFiles).toHaveBeenCalledTimes(1))
    expect(browser.style.getPropertyValue('--wails-drop-target')).toBe('drop')
    await act(async () => {
      finishUpload({ uploaded: ['/home/deploy/report.txt'], failures: [] })
    })
    await waitFor(() => expect(browser.style.getPropertyValue('--wails-drop-target')).toBe('drop'))
  })

  test('ignores file-drop events without local file paths', async () => {
    const api = fakeApi()
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['', '   '])
    })

    expect(api.uploadFiles).not.toHaveBeenCalled()
  })

  test('reports skipped files and still refreshes the open folder', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: ['/home/deploy/report.txt'],
      failures: [{ name: 'README.md', code: 'exists' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt', '/Users/eric/README.md'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'Uploaded report.txt to /home/deploy. Rename README.md locally and drop it again; that name is already in /home/deploy.',
    )
    await waitFor(() => expect(api.listDirectory).toHaveBeenCalledTimes(2))
  })

  test('groups mixed batch failures with item-specific guidance', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [
        { name: 'Projects', code: 'directory' },
        { name: 'Photos', code: 'directory' },
        { name: 'README.md', code: 'exists' },
      ],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/Projects', '/Users/eric/Photos', '/Users/eric/README.md'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      '3 dropped items need attention. Open Projects and Photos, then drop the individual files you want to add. Rename README.md locally and drop it again; that name is already in /home/deploy.',
    )
  })

  test('guides the user to a writable folder after a permission denial', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [{ name: 'index.html', code: 'permission' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/index.html'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      '/home/deploy isn’t writable. Open a writable folder, then drop index.html there.',
    )
  })

  test('guides the user to a readable local file after a local permission denial', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [{ name: 'private.txt', code: 'local-permission' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/private.txt'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'Give SSH Man access to private.txt, or check its file permissions, then drop it again.',
    )
  })

  test('explains when a dropped local item is no longer available', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [{ name: 'moved.txt', code: 'missing' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/moved.txt'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'moved.txt isn’t on this Mac anymore. Save it to a folder, then drop it again.',
    )
  })

  test('explains when the server rejects file access settings', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [{ name: 'secret.pem', code: 'permissions' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/secret.pem'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'secret.pem couldn’t be uploaded safely, so nothing was added to /home/deploy. Uploads to this server aren’t supported yet.',
    )
  })

  test('explains when an incomplete remote file could not be removed', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [{ name: 'broken.txt', code: 'incomplete' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/broken.txt'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'broken.txt may be incomplete in /home/deploy. Delete it there before trying again.',
    )
  })

  test('uses consumer-facing guidance for unsupported dropped items', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [{ name: 'local.socket', code: 'unsupported' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/tmp/local.socket'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'local.socket can’t be uploaded. Drop a document, image, or other file instead.',
    )
  })

  test('names an unsupported item after uploading the supported files in a batch', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: ['/home/deploy/report.txt'],
      failures: [{ name: 'local.socket', code: 'unsupported' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt', '/tmp/local.socket'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'Uploaded report.txt to /home/deploy. local.socket can’t be uploaded. Drop a document, image, or other file instead.',
    )
  })

  test('uses singular copy for one additional skipped name', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [
        { name: 'one.txt', code: 'failed' },
        { name: 'two.txt', code: 'failed' },
        { name: 'three.txt', code: 'failed' },
        { name: 'four.txt', code: 'failed' },
      ],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/tmp/one.txt', '/tmp/two.txt', '/tmp/three.txt', '/tmp/four.txt'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      '4 dropped items need attention. Try uploading one.txt, two.txt, three.txt, and 1 other to /home/deploy again.',
    )
  })

  test('uses singular guidance when duplicate local names share one failure', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: [],
      failures: [
        { name: 'report.txt', code: 'exists' },
        { name: 'report.txt', code: 'exists' },
      ],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt', '/tmp/report.txt'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      '2 dropped items need attention. Rename report.txt locally and drop it again; that name is already in /home/deploy.',
    )
  })

  test('keeps the upload destination visible while browsing another folder', async () => {
    const user = userEvent.setup()
    let finishUpload
    const api = fakeApi()
    api.uploadFiles.mockImplementation(() => new Promise((resolve) => {
      finishUpload = resolve
    }))
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt'])
    })
    await screen.findByText('Adding files to /home/deploy')
    await user.dblClick(screen.getByRole('option', { name: /site/ }))

    expect(screen.getByText('Adding files to /home/deploy')).toBeTruthy()
    await act(async () => {
      finishUpload({ uploaded: ['/home/deploy/report.txt'], failures: [] })
    })
    expect(await screen.findByText('Uploaded report.txt to /home/deploy.')).toBeTruthy()
    expect(api.listDirectory).toHaveBeenCalledTimes(2)
  })

  test('shows byte progress, the active file, and queued files in a Transfers tab', async () => {
    let finishUpload
    const api = fakeApi()
    api.uploadFiles.mockImplementation(() => new Promise((resolve) => {
      finishUpload = resolve
    }))
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop([
        '/Users/eric/first.bin',
        '/Users/eric/second.zip',
        '/Users/eric/third.txt',
      ])
    })
    const transfersTab = await screen.findByRole('tab', { name: /Transfers/ })
    expect(transfersTab.textContent).toContain('3')

    act(() => {
      api.emitUploadProgress({
        uploadId: 1,
        fileIndex: 0,
        name: 'first.bin',
        status: 'transferring',
        bytesTransferred: 25,
        totalBytes: 100,
        overallBytesProcessed: 25,
        overallBytesTotal: 400,
        filesProcessed: 0,
        filesTotal: 3,
      })
    })

    expect(screen.getByRole('tab', { name: /Transfers/ }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getByRole('progressbar', { name: 'Overall upload progress' }).getAttribute('aria-valuenow')).toBe('6')
    expect(screen.getByText('first.bin')).toBeTruthy()
    expect(screen.getByText('25 B of 100 B')).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Queued' })).toBeTruthy()
    expect(screen.getByText('second.zip')).toBeTruthy()
    expect(screen.getByText('third.txt')).toBeTruthy()

    await act(async () => {
      api.emitUploadProgress({
        uploadId: 1,
        fileIndex: 0,
        name: 'first.bin',
        status: 'completed',
        bytesTransferred: 100,
        totalBytes: 100,
        overallBytesProcessed: 100,
        overallBytesTotal: 400,
        filesProcessed: 1,
        filesTotal: 3,
      })
      finishUpload({
        uploaded: [
          '/home/deploy/first.bin',
          '/home/deploy/second.zip',
          '/home/deploy/third.txt',
        ],
        failures: [],
      })
    })

    expect(await screen.findByText('Upload complete')).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'Transfers' }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getByRole('heading', { name: 'Completed' })).toBeTruthy()
  })

  test('finishes the transfer UI even when refreshing the uploaded folder never settles', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: ['/home/deploy/report.txt'],
      failures: [],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()
    api.listDirectory.mockImplementationOnce(() => new Promise(() => {}))

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt'])
    })

    expect(await screen.findByText('Upload complete')).toBeTruthy()
    expect(screen.getByText('Refreshing deploy…')).toBeTruthy()
    expect(screen.getByText('Upload complete').closest('.explorer-transfers__status')?.classList.contains('is-completed')).toBe(true)
  })

  test('keeps a new queue isolated from progress events sent by the previous upload', async () => {
    let finishSecondUpload
    const api = fakeApi()
    api.uploadFiles
      .mockResolvedValueOnce({ uploaded: ['/home/deploy/first.txt'], failures: [] })
      .mockImplementationOnce(() => new Promise((resolve) => {
        finishSecondUpload = resolve
      }))
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()
    api.listDirectory.mockImplementationOnce(() => new Promise(() => {}))

    act(() => {
      api.emitFileDrop(['/Users/eric/first.txt'])
    })
    expect(await screen.findByText('Upload complete')).toBeTruthy()

    act(() => {
      api.emitFileDrop(['/Users/eric/second.txt'])
    })
    expect(await screen.findByText('second.txt')).toBeTruthy()

    act(() => {
      api.emitUploadProgress({
        uploadId: 1,
        fileIndex: 0,
        name: 'first.txt',
        status: 'completed',
        bytesTransferred: 10,
        totalBytes: 10,
        overallBytesProcessed: 10,
        overallBytesTotal: 10,
        filesProcessed: 1,
        filesTotal: 1,
      })
    })
    expect(screen.getByRole('heading', { name: 'Queued' })).toBeTruthy()
    expect(screen.getByText('second.txt')).toBeTruthy()

    act(() => {
      api.emitUploadProgress({
        uploadId: 2,
        fileIndex: 0,
        name: 'second.txt',
        status: 'transferring',
        bytesTransferred: 5,
        totalBytes: 10,
        overallBytesProcessed: 5,
        overallBytesTotal: 10,
        filesProcessed: 0,
        filesTotal: 1,
      })
    })
    expect(screen.getByRole('heading', { name: 'Transferring now' })).toBeTruthy()
    expect(screen.getByText('5 B of 10 B')).toBeTruthy()

    await act(async () => {
      finishSecondUpload({ uploaded: ['/home/deploy/second.txt'], failures: [] })
    })
  })

  test('moves focus when switching Preview and Transfers with arrow keys', async () => {
    let finishUpload
    const api = fakeApi()
    api.uploadFiles.mockImplementation(() => new Promise((resolve) => {
      finishUpload = resolve
    }))
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt'])
    })
    const previewTab = screen.getByRole('tab', { name: /Preview/ })
    const transfersTab = await screen.findByRole('tab', { name: /Transfers/ })

    previewTab.focus()
    fireEvent.keyDown(previewTab, { key: 'ArrowRight' })
    expect(document.activeElement).toBe(transfersTab)
    expect(transfersTab.getAttribute('aria-selected')).toBe('true')
    expect(transfersTab.getAttribute('tabindex')).toBe('0')

    fireEvent.keyDown(transfersTab, { key: 'ArrowLeft' })
    expect(document.activeElement).toBe(previewTab)
    expect(previewTab.getAttribute('aria-selected')).toBe('true')
    expect(previewTab.getAttribute('tabindex')).toBe('0')

    await act(async () => {
      finishUpload({ uploaded: ['/home/deploy/report.txt'], failures: [] })
    })
  })

  test('explains when another drop arrives during an upload', async () => {
    let finishUpload
    const api = fakeApi()
    api.uploadFiles.mockImplementation(() => new Promise((resolve) => {
      finishUpload = resolve
    }))
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    act(() => {
      api.emitFileDrop(['/Users/eric/first.txt'])
    })
    await waitFor(() => expect(api.uploadFiles).toHaveBeenCalledTimes(1))
    act(() => {
      api.emitFileDrop(['/Users/eric/second.txt'])
    })

    expect(await screen.findByText('Still uploading to /home/deploy. These files weren’t added — drop them again when it finishes.')).toBeTruthy()
    expect(api.uploadFiles).toHaveBeenCalledTimes(1)
    await act(async () => {
      finishUpload({ uploaded: ['/home/deploy/first.txt'], failures: [] })
    })
  })

  test('describes the drop destination before an upload starts', async () => {
    const api = fakeApi()
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()

    expect(screen.getByText('Add files to /home/deploy')).toBeTruthy()
  })

  test('keeps the upload error visible when the folder refresh also fails', async () => {
    const api = fakeApi()
    api.uploadFiles.mockRejectedValue(new Error('transport reset by peer'))
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()
    api.listDirectory.mockRejectedValueOnce(new Error('refresh failed'))

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'The files could not be uploaded. Check the connection and try again.',
    )
    await waitFor(() => expect(api.listDirectory).toHaveBeenCalledTimes(2))
  })

  test('uses actionable copy when an uploaded folder cannot be refreshed', async () => {
    const api = fakeApi()
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()
    api.listDirectory.mockRejectedValueOnce(new Error('list "/home/deploy": permission denied'))

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'Your files uploaded to /home/deploy, but it could not be refreshed. Reopen it to see them.',
    )
    expect(screen.queryByText('Uploaded report.txt.')).toBeNull()
  })

  test('reports a stale listing after partial success cannot be refreshed', async () => {
    const api = fakeApi()
    api.uploadFiles.mockResolvedValue({
      uploaded: ['/home/deploy/report.txt'],
      failures: [{ name: 'README.md', code: 'exists' }],
    })
    render(<ExplorerApp api={api} />)
    await waitForExplorerReady()
    api.listDirectory.mockRejectedValueOnce(new Error('connection closed'))

    act(() => {
      api.emitFileDrop(['/Users/eric/report.txt', '/Users/eric/README.md'])
    })

    expect((await screen.findByRole('alert')).textContent).toBe(
      'Uploaded report.txt to /home/deploy. Rename README.md locally and drop it again; that name is already in /home/deploy. Reopen /home/deploy to see the files that were added.',
    )
  })

  test('edits and saves a remote file from Monaco', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(await screen.findByRole('button', { name: 'Source' }))
    const editor = await screen.findByRole('textbox', { name: 'README.md source' })
    await user.clear(editor)
    await user.type(editor, '# Updated remotely')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.saveFile).toHaveBeenCalledWith('/home/deploy/README.md', '# Updated remotely', 'revision-1'))
    expect(await screen.findByText('Saved README.md.')).toBeTruthy()
  })

  test('keeps an unsaved editor inline instead of opening a detached preview', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(await screen.findByRole('button', { name: 'Source' }))
    const editor = await screen.findByRole('textbox', { name: 'README.md source' })
    await user.type(editor, ' unsaved')

    const popout = screen.getByRole('button', { name: 'Open README.md preview in new window' })
    expect(popout.disabled).toBe(true)
    await user.click(popout)
    expect(api.openPreview).not.toHaveBeenCalled()
  })

  test('persists Vim controls from the editor checkbox', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(await screen.findByRole('button', { name: 'Source' }))
    await user.click(screen.getByRole('checkbox', { name: 'Vim controls' }))

    expect(window.localStorage.getItem('ssh-man:explorer:vim-enabled')).toBe('true')
    expect(screen.getByTestId('monaco-editor').dataset.vim).toBe('enabled')
  })

  test('favorites the current folder per server and reopens it from the sidebar', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.dblClick(await screen.findByRole('option', { name: /site/ }))
    await screen.findByText('This folder is empty')
    await user.click(screen.getByRole('button', { name: 'Favorite current folder' }))
    await user.click(screen.getByRole('button', { name: 'Open favorite site' }))

    await waitFor(() => expect(api.listDirectory).toHaveBeenLastCalledWith('/home/deploy/site'))
    expect(JSON.parse(window.localStorage.getItem('ssh-man:explorer:favorites:server-1'))).toEqual(['/home/deploy/site'])
  })
})
