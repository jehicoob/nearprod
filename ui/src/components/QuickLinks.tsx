import { ExternalLink } from 'lucide-react'
import type { ProductGroup } from '../types'
import { quickLinkEntries } from '../lib/dashboard'

export interface QuickLinksProps {
  readonly group: ProductGroup
}

export function QuickLinks({ group }: QuickLinksProps) {
  const links = quickLinkEntries(group)
  if (!links.length) return <p className="muted">Sin accesos rápidos registrados.</p>
  return (
    <div className="quick-links" aria-label={`Accesos rápidos de ${group.label}`}>
      {links.map((link) => (
        <a key={`${link.kind}-${link.href}`} className={`quick-link quick-link--${link.kind}`} href={link.href} target="_blank" rel="noreferrer">
          {link.label}
          <ExternalLink size={13} />
        </a>
      ))}
    </div>
  )
}
