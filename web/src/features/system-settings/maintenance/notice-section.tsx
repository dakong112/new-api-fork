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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const noticeSchema = z.object({
  content: z.string().optional(),
})

type NoticeFormValues = z.infer<typeof noticeSchema>

type NoticeSectionProps = {
  defaultValue: string
  /** Option key the content is saved to. Defaults to the system notice. */
  optionKey?: string
  /** i18n keys; default to the system notice wording. */
  titleKey?: string
  placeholderKey?: string
  descriptionKey?: string
}

export function NoticeSection(props: NoticeSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaultValue = props.defaultValue ?? ''
  const form = useForm<NoticeFormValues>({
    resolver: zodResolver(noticeSchema),
    defaultValues: {
      content: defaultValue,
    },
  })

  useEffect(() => {
    form.reset({ content: defaultValue })
  }, [defaultValue, form])

  const onSubmit = async (values: NoticeFormValues) => {
    const normalized = values.content ?? ''
    if (normalized === defaultValue) {
      return
    }
    await updateOption.mutateAsync({
      key: props.optionKey ?? 'Notice',
      value: normalized,
    })
  }

  return (
    <SettingsSection title={t(props.titleKey ?? 'System Notice')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save notice'
          />
          <FormField
            control={form.control}
            name='content'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Announcement content')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={8}
                    placeholder={t(
                      props.placeholderKey ??
                        'Planned maintenance on Friday at 22:00 UTC...'
                    )}
                    {...field}
                  />
                </FormControl>
                {props.descriptionKey && (
                  <FormDescription>{t(props.descriptionKey)}</FormDescription>
                )}
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
