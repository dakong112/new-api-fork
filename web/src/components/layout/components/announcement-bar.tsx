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
import {
  Link as LinkIcon,
  Mail,
  Megaphone,
  MessageCircle,
  X,
} from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconDiscord, IconTelegram, IconWeChat } from '@/assets/brand-icons'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

import { type ContactLink, toPlainSummary } from '../lib/announcement-bar'

const DISMISS_STORAGE_KEY = 'announcement-bar-dismissed'
// ponytail: duration scales with text length so short and long notices scroll at a similar speed.
const MARQUEE_SECONDS_PER_CHAR = 0.25
const MARQUEE_MIN_SECONDS = 15

const CONTACT_ICONS: Record<
  ContactLink['icon'],
  React.ComponentType<{ className?: string }>
> = {
  qq: MessageCircle,
  wechat: IconWeChat,
  telegram: IconTelegram,
  discord: IconDiscord,
  email: Mail,
  link: LinkIcon,
}

function readDismissedKey(): string | null {
  try {
    return localStorage.getItem(DISMISS_STORAGE_KEY)
  } catch {
    return null
  }
}

function ContactItem(props: { link: ContactLink }) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const Icon = CONTACT_ICONS[props.link.icon] ?? LinkIcon
  const text = props.link.value
    ? `${props.link.label} ${props.link.value}`
    : props.link.label
  const content = (
    <>
      <Icon className='size-4' aria-hidden='true' />
      <span className='hidden max-w-48 truncate md:inline'>
        {props.link.label}
        {props.link.value ? (
          <span className='ms-1 font-semibold tabular-nums'>
            {props.link.value}
          </span>
        ) : null}
      </span>
    </>
  )
  const commonProps = {
    variant: 'outline' as const,
    size: 'sm' as const,
    title: text,
    'aria-label': text,
    className:
      'border-primary/40 bg-primary/10 text-primary hover:bg-primary/20 hover:text-primary rounded-full font-medium',
  }

  if (props.link.icon === 'wechat' && props.link.url) {
    return (
      <Popover>
        <PopoverTrigger render={<Button {...commonProps}>{content}</Button>} />
        <PopoverContent align='center' className='w-auto'>
          <div className='flex flex-col items-center gap-2'>
            <QRCodeSVG value={props.link.url} size={160} />
            <span className='text-muted-foreground text-xs'>
              {t('Scan with WeChat')}
            </span>
          </div>
        </PopoverContent>
      </Popover>
    )
  }

  if (props.link.url) {
    return (
      <Button
        {...commonProps}
        render={
          <a href={props.link.url} target='_blank' rel='noopener noreferrer' />
        }
      >
        {content}
      </Button>
    )
  }

  return (
    <Button
      {...commonProps}
      onClick={() => copyToClipboard(props.link.value ?? '')}
    >
      {content}
    </Button>
  )
}

/** Admin-configured contact buttons shown in the middle of the header. */
export function HeaderContactLinks(props: { className?: string }) {
  const { status } = useStatus()
  const contacts =
    status?.contact_links_enabled === false
      ? []
      : ((status?.contact_links as ContactLink[] | undefined) ?? [])

  if (contacts.length === 0) return null

  return (
    <div className={cn('flex min-w-0 items-center gap-1.5', props.className)}>
      {contacts.map((link) => (
        <ContactItem key={`${link.icon}:${link.label}`} link={link} />
      ))}
    </div>
  )
}

type AnnouncementBarProps = {
  /** System notice (Markdown) published in system settings. */
  notice: string
  /** Opens the notification center to read the full notice. */
  onOpenNotice: () => void
}

export function AnnouncementBar(props: AnnouncementBarProps) {
  const { t } = useTranslation()
  const [dismissedNotice, setDismissedNotice] = useState(readDismissedKey)
  const notice = props.notice.trim()

  // Dismissal is tied to the notice content so publishing a new one brings the bar back.
  if (!notice || dismissedNotice === notice) return null

  const dismiss = () => {
    try {
      localStorage.setItem(DISMISS_STORAGE_KEY, notice)
    } catch {
      // Storage unavailable: still hide for this session.
    }
    setDismissedNotice(notice)
  }
  const text = toPlainSummary(notice)
  const duration = Math.max(
    MARQUEE_MIN_SECONDS,
    text.length * MARQUEE_SECONDS_PER_CHAR
  )
  const renderText = (copy: 'a' | 'b') => (
    <button
      type='button'
      tabIndex={copy === 'b' ? -1 : undefined}
      onClick={props.onOpenNotice}
      className='shrink-0 pe-24 whitespace-nowrap hover:underline'
    >
      {text}
    </button>
  )

  return (
    <div
      data-announcement-bar
      role='region'
      aria-label={t('System Notice')}
      className='bg-muted/50 flex h-8 shrink-0 items-center gap-2 border-y px-2 text-xs sm:px-3'
    >
      <Megaphone
        className='text-primary size-3.5 shrink-0'
        aria-hidden='true'
      />
      <div className='group/marquee min-w-0 flex-1 overflow-hidden'>
        {/* Two copies so translating by -50% loops seamlessly. */}
        <div
          className='flex w-max animate-[announcement-marquee_linear_infinite] group-focus-within/marquee:[animation-play-state:paused] group-hover/marquee:[animation-play-state:paused] motion-reduce:animate-none'
          style={{ animationDuration: `${duration}s` }}
        >
          {renderText('a')}
          <div aria-hidden='true' className='flex motion-reduce:hidden'>
            {renderText('b')}
          </div>
        </div>
      </div>
      <Button
        variant='ghost'
        size='icon-xs'
        aria-label={t('Close')}
        onClick={dismiss}
      >
        <X aria-hidden='true' />
      </Button>
    </div>
  )
}
