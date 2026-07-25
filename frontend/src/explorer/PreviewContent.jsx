import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { File } from 'lucide-react'
import { remoteContentURL } from '../lib/explorerApi'
import MonacoPreview from './MonacoPreview'
import {
  editorLanguage,
  formatFileSize,
  resolveRemoteReference,
} from './pathUtils'

export default function PreviewContent({
  content,
  contentKey = '',
  mode = 'rendered',
  onChange,
  onSave,
  preview,
  readOnly = false,
  vimMode = false,
}) {
  if (!preview) return null

  const sourceMode = mode === 'source' && preview.content != null && ['markdown', 'browser'].includes(preview.kind)
  const editor = (
    <MonacoPreview
      content={content}
      language={editorLanguage(preview.name)}
      label={`${preview.name} source`}
      modelKey={`${preview.path}:${contentKey}`}
      onChange={onChange}
      onSave={onSave}
      readOnly={readOnly}
      vimMode={vimMode}
    />
  )

  return (
    <div className="explorer-preview__body">
      {preview.truncated ? <div className="explorer-preview-note">Preview limited to the first 2 MB. Download for the full file.</div> : null}
      {sourceMode ? editor : null}
      {mode === 'rendered' && preview.kind === 'markdown' ? (
        <article className="explorer-markdown">
          <ReactMarkdown
            remarkPlugins={[remarkGfm]}
            urlTransform={(url) => resolveRemoteReference(preview.path, url)}
            components={{ a: ({ node: _node, ...props }) => <a {...props} target="_blank" rel="noreferrer" /> }}
          >{content}</ReactMarkdown>
        </article>
      ) : null}
      {mode === 'rendered' && preview.kind === 'browser' ? <iframe key={`${preview.revision}:${contentKey}`} title={`${preview.name} browser preview`} src={remoteContentURL(preview.path)} sandbox="allow-forms" /> : null}
      {mode === 'rendered' && preview.kind === 'image' ? <div className="explorer-image"><img key={contentKey} src={remoteContentURL(preview.path)} alt={preview.name} /></div> : null}
      {mode === 'rendered' && preview.kind === 'video' ? (
        <div className="explorer-video">
          <video key={contentKey} aria-label={`${preview.name} video preview`} controls playsInline preload="metadata" src={remoteContentURL(preview.path)} />
        </div>
      ) : null}
      {mode === 'rendered' && preview.kind === 'unsupported' ? <div className="explorer-preview-empty"><File /><strong>No preview available</strong><span>{formatFileSize(preview.size)} · Download to open locally.</span></div> : null}
      {preview.kind === 'text' ? editor : null}
    </div>
  )
}
