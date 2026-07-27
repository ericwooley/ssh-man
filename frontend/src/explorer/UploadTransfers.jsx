import {
  CheckCircle2,
  CircleAlert,
  Clock3,
  File,
  LoaderCircle,
  RefreshCw,
} from 'lucide-react'
import { formatFileSize } from './pathUtils'
import { summarizeUploadQueue } from './uploadQueue'

function folderName(remotePath) {
  return String(remotePath || '').split('/').filter(Boolean).at(-1) || '/'
}

function failureLabel(code) {
  if (code === 'exists') return 'A file with this name is already here'
  if (code === 'directory') return 'Open this folder and drop its files instead'
  if (code === 'permission') return 'This destination is not writable'
  if (code === 'local-permission') return 'SSH Man could not read this local file'
  if (code === 'missing') return 'This local file is no longer available'
  if (code === 'permissions') return 'The server rejected this file’s permissions'
  if (code === 'incomplete') return 'An incomplete remote file may remain'
  if (code === 'unsupported') return 'This item cannot be uploaded'
  if (code === 'interrupted') return 'The transfer was interrupted'
  return 'This file could not be uploaded'
}

function TransferRows({ files, status }) {
  if (!files.length) return null
  const Icon = status === 'completed' ? CheckCircle2 : status === 'failed' ? CircleAlert : Clock3
  const title = status === 'completed' ? 'Completed' : status === 'failed' ? 'Needs attention' : 'Queued'
  return (
    <section className={`explorer-transfers__section is-${status}`}>
      <h3>{title}</h3>
      <div className="explorer-transfer-list">
        {files.map((file) => (
          <div className="explorer-transfer-row" key={`${file.index}:${file.localPath}`}>
            <Icon aria-hidden="true" />
            <div>
              <strong title={file.name}>{file.name}</strong>
              {status === 'failed' ? <span>{failureLabel(file.failureCode)}</span> : null}
            </div>
            {file.totalBytes > 0 ? <small>{formatFileSize(file.totalBytes)}</small> : null}
          </div>
        ))}
      </div>
    </section>
  )
}

export default function UploadTransfers({ queue }) {
  const summary = summarizeUploadQueue(queue)
  const isUploading = queue.status === 'uploading'
  const title = isUploading
    ? `Uploading ${Math.min(queue.filesProcessed + 1, queue.filesTotal)} of ${queue.filesTotal}`
    : queue.status === 'completed'
      ? 'Upload complete'
      : queue.status === 'finished-with-errors'
        ? `Finished with ${summary.failedFiles.length} ${summary.failedFiles.length === 1 ? 'issue' : 'issues'}`
        : 'Upload interrupted'
  const StatusIcon = isUploading ? LoaderCircle : queue.status === 'completed' ? CheckCircle2 : CircleAlert
  const currentPercent = summary.currentFile?.totalBytes > 0
    ? Math.round((summary.currentFile.bytesTransferred / summary.currentFile.totalBytes) * 100)
    : 0

  return (
    <div className="explorer-transfers" role="status" aria-label="Upload progress">
      <div className={`explorer-transfers__status is-${queue.status}`}>
        <StatusIcon className={isUploading ? 'spin' : ''} aria-hidden="true" />
        <div>
          <strong aria-live="polite">{title}</strong>
          <span title={queue.destination}>Adding files to {queue.destination}</span>
        </div>
        <b>{summary.overallPercent}%</b>
      </div>

      <progress
        className="explorer-transfers__progress"
        aria-label="Overall upload progress"
        aria-valuenow={summary.overallPercent}
        max="100"
        value={summary.overallPercent}
      />
      <div className="explorer-transfers__summary">
        <span>{queue.filesProcessed} of {queue.filesTotal} files finished</span>
        {summary.remainingCount > 0 ? <span>{summary.remainingCount} left</span> : null}
      </div>

      {summary.currentFile ? (
        <section className="explorer-transfers__current">
          <h3>Transferring now</h3>
          <div>
            <File aria-hidden="true" />
            <strong title={summary.currentFile.name}>{summary.currentFile.name}</strong>
            <span>{formatFileSize(summary.currentFile.bytesTransferred)} of {formatFileSize(summary.currentFile.totalBytes)}</span>
          </div>
          <progress
            aria-label={`${summary.currentFile.name} upload progress`}
            aria-valuenow={currentPercent}
            max="100"
            value={currentPercent}
          />
        </section>
      ) : null}

      <TransferRows files={summary.queuedFiles} status="queued" />
      <TransferRows files={summary.completedFiles} status="completed" />
      <TransferRows files={summary.failedFiles} status="failed" />

      {queue.refreshStatus === 'refreshing' ? (
        <div className="explorer-transfers__refresh">
          <RefreshCw className="spin" aria-hidden="true" />
          <span>Refreshing {folderName(queue.destination)}…</span>
        </div>
      ) : null}
      {queue.refreshStatus === 'completed' ? (
        <div className="explorer-transfers__refresh is-complete">
          <CheckCircle2 aria-hidden="true" />
          <span>Folder refreshed</span>
        </div>
      ) : null}
      {queue.refreshStatus === 'failed' ? (
        <div className="explorer-transfers__refresh is-failed">
          <CircleAlert aria-hidden="true" />
          <span>Reopen {folderName(queue.destination)} to refresh its file list.</span>
        </div>
      ) : null}
    </div>
  )
}
