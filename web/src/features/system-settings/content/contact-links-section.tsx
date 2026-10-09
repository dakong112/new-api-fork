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
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Save } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Dialog } from '@/components/dialog'
import type { ContactLink } from '@/components/layout/lib/announcement-bar'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { handleServerError } from '@/lib/handle-server-error'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type ContactLinkRow = ContactLink & { id: number }

type ContactLinksSectionProps = {
  enabled: boolean
  data: string
}

const MAX_CONTACT_LINKS = 10
const CONTACT_LINK_FORM_ID = 'contact-link-form'

const iconOptions: { value: ContactLink['icon']; label: string }[] = [
  { value: 'qq', label: 'QQ' },
  { value: 'wechat', label: 'WeChat' },
  { value: 'telegram', label: 'Telegram' },
  { value: 'discord', label: 'Discord' },
  { value: 'email', label: 'Email' },
  { value: 'link', label: 'Link' },
]

const createContactLinkSchema = (t: (key: string) => string) =>
  z
    .object({
      icon: z.enum(['qq', 'wechat', 'telegram', 'discord', 'email', 'link']),
      label: z.string().trim().min(1, t('Name is required')).max(30),
      value: z.string().trim().max(100),
      url: z
        .string()
        .trim()
        .max(500)
        .refine(
          (v) => v === '' || /^(https?:\/\/|mailto:).+/.test(v),
          t('Must start with http://, https:// or mailto:')
        ),
    })
    .refine((v) => v.value !== '' || v.url !== '', {
      message: t('Fill in a display value or a link'),
      path: ['value'],
    })

type ContactLinkFormValues = z.infer<ReturnType<typeof createContactLinkSchema>>

const emptyForm: ContactLinkFormValues = {
  icon: 'qq',
  label: '',
  value: '',
  url: '',
}

function parseContactLinks(data: string): ContactLinkRow[] {
  try {
    const parsed = JSON.parse(data || '[]')
    if (!Array.isArray(parsed)) return []
    return parsed.map((item, idx) => ({ ...item, id: idx + 1 }))
  } catch {
    return []
  }
}

export function ContactLinksSection(props: ContactLinksSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const parsedList = useMemo(() => parseContactLinks(props.data), [props.data])
  const [draftList, setDraftList] = useState<ContactLinkRow[] | null>(null)
  const [isEnabledDraft, setIsEnabledDraft] = useState<boolean | null>(null)
  const [editing, setEditing] = useState<ContactLinkRow | null>(null)
  const [showDialog, setShowDialog] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ContactLinkRow | null>(null)
  const list = draftList ?? parsedList
  const isEnabled = isEnabledDraft ?? props.enabled

  const form = useForm<ContactLinkFormValues>({
    resolver: zodResolver(createContactLinkSchema(t)),
    defaultValues: emptyForm,
  })

  const handleToggleEnabled = async (checked: boolean) => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.contact_links_enabled',
        value: checked,
      })
      setIsEnabledDraft(checked)
      toast.success(t('Setting saved'))
    } catch (error) {
      handleServerError(error, t('Failed to update setting'))
    }
  }

  const openDialog = (row: ContactLinkRow | null) => {
    setEditing(row)
    form.reset(
      row
        ? {
            icon: row.icon,
            label: row.label,
            value: row.value ?? '',
            url: row.url ?? '',
          }
        : emptyForm
    )
    setShowDialog(true)
  }

  const handleSubmitForm = (values: ContactLinkFormValues) => {
    if (editing) {
      setDraftList(
        list.map((item) =>
          item.id === editing.id ? { ...values, id: item.id } : item
        )
      )
    } else {
      const newId = Math.max(...list.map((item) => item.id), 0) + 1
      setDraftList([...list, { ...values, id: newId }])
    }
    toast.success(t('Contact updated. Click "Save Settings" to apply.'))
    setShowDialog(false)
  }

  const handleSaveAll = async () => {
    // Drop the local row id and empty optional fields before persisting.
    const payload = list.map((item) => ({
      icon: item.icon,
      label: item.label,
      ...(item.value ? { value: item.value } : {}),
      ...(item.url ? { url: item.url } : {}),
    }))
    try {
      const result = await updateOption.mutateAsync({
        key: 'console_setting.contact_links',
        value: JSON.stringify(payload),
      })
      if (result.success) {
        setDraftList(null)
      }
    } catch (error) {
      handleServerError(error, t('Failed to update setting'))
    }
  }

  return (
    <SettingsSection title={t('Contact Links')}>
      <div className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Shown in the bar under the top header. Items with a link open it; items without a link copy the display value; WeChat with a link shows a QR code.'
          )}
        </p>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button
              onClick={() => openDialog(null)}
              size='sm'
              disabled={list.length >= MAX_CONTACT_LINKS}
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add Contact')}
            </Button>
            <Button
              onClick={handleSaveAll}
              size='sm'
              variant='secondary'
              disabled={draftList === null || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending ? t('Saving...') : t('Save Settings')}
            </Button>
          </div>
          <SettingsSwitchField
            checked={isEnabled}
            onCheckedChange={handleToggleEnabled}
            label={t('Enabled')}
            className='py-0'
          />
        </div>

        <StaticDataTable
          data={list}
          getRowKey={(row) => row.id}
          emptyContent={t(
            'No contacts yet. Click "Add Contact" to create one.'
          )}
          columns={[
            {
              id: 'icon',
              header: t('Type'),
              cell: (row) =>
                iconOptions.find((option) => option.value === row.icon)
                  ?.label ?? row.icon,
            },
            { id: 'label', header: t('Name'), cell: (row) => row.label },
            {
              id: 'value',
              header: t('Display Value'),
              cellClassName: 'max-w-xs truncate',
              cell: (row) => row.value || '-',
            },
            {
              id: 'url',
              header: t('Link'),
              cellClassName: 'max-w-xs truncate font-mono text-xs',
              cell: (row) => row.url || '-',
            },
            {
              id: 'actions',
              header: t('Actions'),
              cell: (row) => (
                <StaticRowActions
                  editLabel={t('Edit')}
                  deleteLabel={t('Delete')}
                  menuLabel={t('Open menu')}
                  onEdit={() => openDialog(row)}
                  onDelete={() => setDeleteTarget(row)}
                />
              ),
            },
          ]}
        />
      </div>

      <Dialog
        open={showDialog}
        onOpenChange={setShowDialog}
        title={editing ? t('Edit Contact') : t('Add Contact')}
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => setShowDialog(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='submit' form={CONTACT_LINK_FORM_ID}>
              {editing ? t('Update') : t('Add')}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={CONTACT_LINK_FORM_ID}
            onSubmit={form.handleSubmit(handleSubmitForm)}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='icon'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Type')}</FormLabel>
                  <Select
                    items={iconOptions}
                    onValueChange={field.onChange}
                    value={field.value}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {iconOptions.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='label'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('e.g., QQ Group')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='value'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Display Value')}</FormLabel>
                  <FormControl>
                    <Input placeholder='123456789' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Copied on click when no link is set')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Link')}</FormLabel>
                  <FormControl>
                    <Input placeholder='https://qm.qq.com/q/xxxx' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Optional. http(s) or mailto: link opened on click')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Are you sure?')}
        desc={t('This contact will be removed from the list.')}
        confirmText={t('Delete')}
        destructive
        handleConfirm={() => {
          setDraftList(list.filter((item) => item.id !== deleteTarget?.id))
          setDeleteTarget(null)
        }}
      />
    </SettingsSection>
  )
}
