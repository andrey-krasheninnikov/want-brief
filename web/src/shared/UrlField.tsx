import { OpenUrl as DesystOpenUrl, UrlField as DesystUrlField, type OpenUrlProps, type UrlFieldProps } from '@want-foundation/desyst-react'

export function OpenUrl({ label = 'Open link', ...props }: Omit<OpenUrlProps, 'label'> & { label?: string }) {
  return <DesystOpenUrl {...props} label={label} className="url-open" />
}

export function UrlField(props: Omit<UrlFieldProps, 'openLabel'>) {
  return <DesystUrlField {...props} openLabel="Open link" />
}
