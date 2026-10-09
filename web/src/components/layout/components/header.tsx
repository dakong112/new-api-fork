/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { SidebarTrigger } from '@/components/ui/sidebar'
import { cn } from '@/lib/utils'

type HeaderProps = React.HTMLAttributes<HTMLElement> & {
  /** Full-width strip rendered under the header row. */
  banner?: React.ReactNode
}

export function Header({ className, children, banner, ...props }: HeaderProps) {
  return (
    <header
      className={cn(
        'sticky top-0 z-40 flex h-[var(--app-header-height,3rem)] w-full shrink-0 flex-col bg-transparent',
        className
      )}
      {...props}
    >
      <div className='flex h-12 shrink-0 items-center gap-1.5 px-2 sm:gap-2 sm:px-3'>
        <SidebarTrigger variant='ghost' className='size-8' />
        {children}
      </div>
      {banner}
    </header>
  )
}
