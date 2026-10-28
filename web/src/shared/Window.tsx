import { Window as DesystWindow, type WindowProps } from '@want-foundation/desyst-react'

export function Window({ className, ...props }: WindowProps) {
  return <DesystWindow {...props} className={className ? 'window ' + className : 'window'} />
}
