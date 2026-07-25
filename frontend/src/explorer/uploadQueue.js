function localFileName(localPath) {
  const segments = String(localPath || '').split(/[\\/]/).filter(Boolean)
  return segments.at(-1) || 'Dropped item'
}

function consumeMatching(items, predicate) {
  const index = items.findIndex(predicate)
  if (index < 0) return null
  return items.splice(index, 1)[0]
}

function isTerminalFile(file) {
  return file.status === 'completed' || file.status === 'failed'
}

export function createUploadQueue(localPaths, destination, id = 0) {
  return {
    id,
    destination,
    status: 'uploading',
    refreshStatus: 'idle',
    overallBytesProcessed: 0,
    overallBytesTotal: 0,
    filesProcessed: 0,
    filesTotal: localPaths.length,
    files: localPaths.map((localPath, index) => ({
      index,
      localPath,
      name: localFileName(localPath),
      status: 'queued',
      bytesTransferred: 0,
      totalBytes: 0,
      failureCode: '',
    })),
  }
}

export function applyUploadProgress(queue, progress) {
  if (!queue || !progress || queue.status !== 'uploading') return queue
  if (Number(progress.uploadId) !== Number(queue.id)) return queue
  const fileIndex = Number(progress.fileIndex)
  if (!Number.isInteger(fileIndex) || fileIndex < 0 || fileIndex >= queue.files.length) return queue

  const files = queue.files.map((file) => {
    if (file.index !== fileIndex) return file
    return {
      ...file,
      name: progress.name || file.name,
      status: progress.status || file.status,
      bytesTransferred: Math.max(0, Number(progress.bytesTransferred) || 0),
      totalBytes: Math.max(0, Number(progress.totalBytes) || 0),
      failureCode: progress.failureCode || '',
    }
  })

  return {
    ...queue,
    overallBytesProcessed: Math.max(0, Number(progress.overallBytesProcessed) || 0),
    overallBytesTotal: Math.max(0, Number(progress.overallBytesTotal) || 0),
    filesProcessed: Math.max(0, Number(progress.filesProcessed) || 0),
    filesTotal: Math.max(queue.files.length, Number(progress.filesTotal) || 0),
    files,
  }
}

export function completeUploadQueue(queue, result) {
  if (!queue) return queue
  const uploaded = (Array.isArray(result?.uploaded) ? result.uploaded : [])
    .map((remotePath) => ({ name: localFileName(remotePath) }))
  const failures = (Array.isArray(result?.failures) ? result.failures : [])
    .map((failure) => ({
      name: failure?.name || 'Dropped item',
      code: failure?.code || 'failed',
    }))

  const files = queue.files.map((file) => {
    if (isTerminalFile(file)) return file
    const uploadedMatch = consumeMatching(uploaded, (item) => item.name === file.name)
    if (uploadedMatch) {
      return {
        ...file,
        status: 'completed',
        bytesTransferred: file.totalBytes,
        failureCode: '',
      }
    }
    const failureMatch = consumeMatching(failures, (item) => item.name === file.name)
    return {
      ...file,
      status: 'failed',
      failureCode: failureMatch?.code || 'failed',
    }
  })
  const failed = files.filter((file) => file.status === 'failed')

  return {
    ...queue,
    status: failed.length ? 'finished-with-errors' : 'completed',
    overallBytesProcessed: Math.max(queue.overallBytesProcessed, queue.overallBytesTotal),
    filesProcessed: files.length,
    files,
  }
}

export function failUploadQueue(queue) {
  if (!queue) return queue
  return {
    ...queue,
    status: 'failed',
    overallBytesProcessed: Math.max(queue.overallBytesProcessed, queue.overallBytesTotal),
    filesProcessed: queue.files.length,
    files: queue.files.map((file) => (
      isTerminalFile(file)
        ? file
        : { ...file, status: 'failed', failureCode: 'interrupted' }
    )),
  }
}

export function summarizeUploadQueue(queue) {
  if (!queue) {
    return {
      completedFiles: [],
      currentFile: null,
      failedFiles: [],
      overallPercent: 0,
      queuedFiles: [],
      remainingCount: 0,
    }
  }
  const completedFiles = queue.files.filter((file) => file.status === 'completed')
  const failedFiles = queue.files.filter((file) => file.status === 'failed')
  const queuedFiles = queue.files.filter((file) => file.status === 'queued')
  const currentFile = queue.files.find((file) => file.status === 'transferring') || null
  const remainingCount = queuedFiles.length + (currentFile ? 1 : 0)
  let overallPercent = 0
  if (queue.status !== 'uploading') {
    overallPercent = 100
  } else if (queue.overallBytesTotal > 0) {
    overallPercent = Math.round((queue.overallBytesProcessed / queue.overallBytesTotal) * 100)
  } else if (queue.filesTotal > 0) {
    overallPercent = Math.round((queue.filesProcessed / queue.filesTotal) * 100)
  }

  return {
    completedFiles,
    currentFile,
    failedFiles,
    overallPercent: Math.max(0, Math.min(100, overallPercent)),
    queuedFiles,
    remainingCount,
  }
}
