import { ArrowSquareOut } from '@phosphor-icons/react'
import type { ReactNode } from 'react'

export function isHttpUrl(raw: string): boolean {
  try {
    const parsed = new URL(raw.trim())
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export function OpenUrl({ href, label = 'Open link' }: { href: string; label?: string }) {
  if (!isHttpUrl(href)) return null
  return (
    <a className="url-open" href={href.trim()} target="_blank" rel="noreferrer" aria-label={label}>
      <ArrowSquareOut size={16} weight="light" aria-hidden />
    </a>
  )
}

type Props = {
  label: ReactNode
  value: string
  onChange: (value: string) => void
  onBlur?: (value: string) => void
  placeholder?: string
}

export function UrlField({ label, value, onChange, onBlur, placeholder = 'https://' }: Props) {
  return (
    <label className="url-field">
      {label}
      <span className="url-field-row">
        <input
          value={value}
          placeholder={placeholder}
          autoComplete="off"
          onChange={(event) => onChange(event.target.value)}
          onBlur={onBlur ? (event) => onBlur(event.currentTarget.value) : undefined}
        />
        <OpenUrl href={value} />
      </span>
    </label>
  )
}
