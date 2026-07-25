import { describe, expect, test } from 'vitest'
import {
  applyUploadProgress,
  completeUploadQueue,
  createUploadQueue,
  failUploadQueue,
  summarizeUploadQueue,
} from './uploadQueue'

describe('upload queue', () => {
  test('starts every dropped file in order with a visible destination', () => {
    const queue = createUploadQueue(
      ['/Users/eric/first.txt', '/tmp/second.zip'],
      '/home/deploy',
    )

    expect(queue).toMatchObject({
      destination: '/home/deploy',
      status: 'uploading',
      files: [
        { index: 0, name: 'first.txt', status: 'queued' },
        { index: 1, name: 'second.zip', status: 'queued' },
      ],
    })
  })

  test('tracks byte progress for the active file and preserves what remains', () => {
    const queue = createUploadQueue(
      ['/tmp/first.bin', '/tmp/second.bin'],
      '/srv/files',
    )

    const next = applyUploadProgress(queue, {
      uploadId: 0,
      fileIndex: 0,
      name: 'first.bin',
      status: 'transferring',
      bytesTransferred: 25,
      totalBytes: 100,
      overallBytesProcessed: 25,
      overallBytesTotal: 300,
      filesProcessed: 0,
      filesTotal: 2,
    })

    expect(next.files).toEqual([
      expect.objectContaining({
        name: 'first.bin',
        status: 'transferring',
        bytesTransferred: 25,
        totalBytes: 100,
      }),
      expect.objectContaining({ name: 'second.bin', status: 'queued' }),
    ])
    expect(summarizeUploadQueue(next)).toMatchObject({
      currentFile: expect.objectContaining({ name: 'first.bin' }),
      queuedFiles: [expect.objectContaining({ name: 'second.bin' })],
      overallPercent: 8,
      remainingCount: 2,
    })
  })

  test('settles every item when the upload call resolves even if an event was missed', () => {
    const queue = createUploadQueue(
      ['/tmp/report.txt', '/tmp/README.md'],
      '/home/deploy',
    )

    const completed = completeUploadQueue(queue, {
      uploaded: ['/home/deploy/report.txt'],
      failures: [{ name: 'README.md', code: 'exists' }],
    })

    expect(completed.status).toBe('finished-with-errors')
    expect(completed.files).toEqual([
      expect.objectContaining({ name: 'report.txt', status: 'completed' }),
      expect.objectContaining({
        name: 'README.md',
        status: 'failed',
        failureCode: 'exists',
      }),
    ])
    expect(summarizeUploadQueue(completed)).toMatchObject({
      overallPercent: 100,
      remainingCount: 0,
      completedFiles: [expect.objectContaining({ name: 'report.txt' })],
      failedFiles: [expect.objectContaining({ name: 'README.md' })],
    })
  })

  test('marks unfinished files interrupted when the upload call rejects', () => {
    const queue = applyUploadProgress(
      createUploadQueue(['/tmp/first.txt', '/tmp/second.txt'], '/srv/files'),
      {
        uploadId: 0,
        fileIndex: 0,
        name: 'first.txt',
        status: 'transferring',
        bytesTransferred: 5,
        totalBytes: 10,
        overallBytesProcessed: 5,
        overallBytesTotal: 20,
        filesProcessed: 0,
        filesTotal: 2,
      },
    )

    const failed = failUploadQueue(queue)

    expect(failed.status).toBe('failed')
    expect(failed.files).toEqual([
      expect.objectContaining({ name: 'first.txt', status: 'failed', failureCode: 'interrupted' }),
      expect.objectContaining({ name: 'second.txt', status: 'failed', failureCode: 'interrupted' }),
    ])
    expect(summarizeUploadQueue(failed)).toMatchObject({
      overallPercent: 100,
      remainingCount: 0,
    })
  })

  test('preserves the failure reason when one of two same-name files uploads', () => {
    const queue = createUploadQueue(
      ['/Users/eric/report.txt', '/tmp/report.txt'],
      '/home/deploy',
    )

    const completed = completeUploadQueue(queue, {
      uploaded: ['/home/deploy/report.txt'],
      failures: [{ name: 'report.txt', code: 'exists' }],
    })

    expect(completed.files).toEqual([
      expect.objectContaining({ index: 0, status: 'completed' }),
      expect.objectContaining({ index: 1, status: 'failed', failureCode: 'exists' }),
    ])
  })

  test('ignores progress from an earlier upload batch', () => {
    const queue = createUploadQueue(['/tmp/current.txt'], '/srv/files', 2)

    const next = applyUploadProgress(queue, {
      uploadId: 1,
      fileIndex: 0,
      name: 'previous.txt',
      status: 'completed',
      bytesTransferred: 10,
      totalBytes: 10,
      overallBytesProcessed: 10,
      overallBytesTotal: 10,
      filesProcessed: 1,
      filesTotal: 1,
    })

    expect(next).toBe(queue)
    expect(next.files[0]).toMatchObject({ name: 'current.txt', status: 'queued' })
  })
})
