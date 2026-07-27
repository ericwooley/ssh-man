import {
  Briefcase,
  Code2,
  Globe2,
  Network,
  Shield,
  Star,
  Terminal,
  X,
} from 'lucide-react'
import { browserAppearanceForeground } from '../model/appModel'

export const BROWSER_APPEARANCE_ICON_OPTIONS = [
  { value: '', label: 'Default' },
  { value: 'icon:x', label: 'X', Icon: X },
  { value: 'icon:shield', label: 'Shield', Icon: Shield },
  { value: 'icon:terminal', label: 'Terminal', Icon: Terminal },
  { value: 'icon:globe', label: 'Globe', Icon: Globe2 },
  { value: 'icon:network', label: 'Network', Icon: Network },
  { value: 'icon:star', label: 'Star', Icon: Star },
  { value: 'icon:briefcase', label: 'Briefcase', Icon: Briefcase },
  { value: 'icon:code', label: 'Code', Icon: Code2 },
]

const ICON_COMPONENTS = Object.fromEntries(
  BROWSER_APPEARANCE_ICON_OPTIONS
    .filter(({ Icon }) => Icon)
    .map(({ value, Icon }) => [value, Icon]),
)

export function BrowserAppearanceMark({ target, appearance, fallbackIcon: FallbackIcon }) {
  const Icon = ICON_COMPONENTS[appearance.icon]
  if (Icon) return <Icon />
  if (appearance.icon) return <span className="browser-target__mark">{appearance.icon}</span>
  if (FallbackIcon) return <FallbackIcon />
  return target.kind === 'proxy' ? <Network /> : <Globe2 />
}

export function browserAppearanceStyle(appearance) {
  if (!appearance.primaryColor) return undefined
  return {
    '--browser-primary': appearance.primaryColor,
    '--browser-on-primary': browserAppearanceForeground(appearance.primaryColor),
  }
}
