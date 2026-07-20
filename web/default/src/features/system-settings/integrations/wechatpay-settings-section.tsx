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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldTitle,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

interface WechatPaySettings {
  enabled: boolean
  app_id: string
  mch_id: string
  merchant_cert_serial_no: string
  merchant_private_key_configured: boolean
  api_v3_key_configured: boolean
  public_key_id: string
  public_key_configured: boolean
  notify_url: string
  resolved_notify_url: string
  min_topup: number
  max_topup: number
  order_expire_minutes: number
  pending_order_count: number
  option_crypt_key_configured: boolean
}

interface WechatPayForm extends WechatPaySettings {
  merchant_private_key: string
  api_v3_key: string
  public_key: string
}

const emptySettings: WechatPayForm = {
  enabled: false,
  app_id: '',
  mch_id: '',
  merchant_cert_serial_no: '',
  merchant_private_key_configured: false,
  api_v3_key_configured: false,
  public_key_id: '',
  public_key_configured: false,
  notify_url: '',
  resolved_notify_url: '',
  min_topup: 1,
  max_topup: 4000,
  order_expire_minutes: 10,
  pending_order_count: 0,
  option_crypt_key_configured: false,
  merchant_private_key: '',
  api_v3_key: '',
  public_key: '',
}

export function WechatPaySettingsSection() {
  const { t } = useTranslation()
  const [settings, setSettings] = useState<WechatPayForm>(emptySettings)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [loaded, setLoaded] = useState(false)

  const loadSettings = useCallback(async () => {
    setLoading(true)
    setLoaded(false)
    try {
      const response = await api.get('/api/wechatpay/admin/settings', {
        disableDuplicate: true,
      } as Record<string, unknown>)
      if (response.data.success && response.data.data) {
        setSettings({
          ...emptySettings,
          ...(response.data.data as WechatPaySettings),
        })
        setLoaded(true)
      } else {
        toast.error(
          response.data.message || t('Failed to load WeChat Pay settings')
        )
      }
    } catch {
      toast.error(t('Failed to load WeChat Pay settings'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void loadSettings()
  }, [loadSettings])

  const updateField = <K extends keyof WechatPayForm>(
    key: K,
    value: WechatPayForm[K]
  ) => setSettings((current) => ({ ...current, [key]: value }))

  const handleSave = async () => {
    if (!loaded) return
    setSaving(true)
    try {
      const payload: Record<string, unknown> = {
        enabled: settings.enabled,
        app_id: settings.app_id,
        mch_id: settings.mch_id,
        merchant_cert_serial_no: settings.merchant_cert_serial_no,
        public_key_id: settings.public_key_id,
        notify_url: settings.notify_url,
        min_topup: Number(settings.min_topup),
        max_topup: Number(settings.max_topup),
        order_expire_minutes: Number(settings.order_expire_minutes),
      }
      if (settings.merchant_private_key.trim()) {
        payload.merchant_private_key = settings.merchant_private_key
      }
      if (settings.api_v3_key) payload.api_v3_key = settings.api_v3_key
      if (settings.public_key.trim()) payload.public_key = settings.public_key

      const response = await api.put('/api/wechatpay/admin/settings', payload, {
        skipBusinessError: true,
      } as Record<string, unknown>)
      if (!response.data.success) {
        toast.error(
          response.data.message || t('Failed to save WeChat Pay settings')
        )
        return
      }
      toast.success(t('WeChat Pay settings saved'))
      setSettings((current) => ({
        ...current,
        merchant_private_key: '',
        api_v3_key: '',
        public_key: '',
      }))
      await loadSettings()
    } catch {
      toast.error(t('Failed to save WeChat Pay settings'))
    } finally {
      setSaving(false)
    }
  }

  const clearSecret = async (name: string) => {
    if (
      !window.confirm(
        t(
          'Clearing credentials disables new orders and prevents callbacks and background reconciliation. Reconcile {{count}} pending orders first. Continue only for emergency key revocation.',
          { count: settings.pending_order_count }
        )
      )
    ) {
      return
    }
    setSaving(true)
    try {
      const response = await api.put(
        '/api/wechatpay/admin/settings',
        {
          enabled: false,
          clear_secrets: [name],
          force_clear_secrets: settings.pending_order_count > 0,
        },
        { skipBusinessError: true } as Record<string, unknown>
      )
      if (!response.data.success) {
        toast.error(response.data.message || t('Failed to clear credential'))
        return
      }
      toast.success(t('Credential cleared'))
      setSettings((current) => ({
        ...current,
        merchant_private_key: '',
        api_v3_key: '',
        public_key: '',
      }))
      await loadSettings()
    } catch {
      toast.error(t('Failed to clear credential'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='flex flex-col gap-1'>
            <CardTitle>{t('WeChat Pay Native')}</CardTitle>
            <CardDescription>
              {t('Configure API v3 native QR-code payments for wallet top-ups')}
            </CardDescription>
          </div>
          <Badge variant='secondary'>
            {t('{{count}} pending orders', {
              count: settings.pending_order_count,
            })}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className='flex flex-col gap-5'>
        {loading ? (
          <div className='flex min-h-32 items-center justify-center'>
            <Spinner />
          </div>
        ) : (
          <>
            {!settings.option_crypt_key_configured && (
              <Alert variant='destructive'>
                <AlertTitle>
                  {t('Secret encryption is not configured')}
                </AlertTitle>
                <AlertDescription>
                  {t(
                    'Set OPTION_CRYPT_KEY in the server environment before saving credentials or enabling WeChat Pay.'
                  )}
                </AlertDescription>
              </Alert>
            )}
            <Alert>
              <AlertTitle>{t('Callback URL')}</AlertTitle>
              <AlertDescription className='font-mono text-xs break-all'>
                {settings.resolved_notify_url || '/api/wechatpay/notify'}
              </AlertDescription>
            </Alert>
            <Alert>
              <AlertTitle>{t('No independent sandbox')}</AlertTitle>
              <AlertDescription>
                {t(
                  'WeChat Pay API v3 has no independent sandbox. Use restricted test users and low-value real payments; do not load-test production payment APIs.'
                )}
              </AlertDescription>
            </Alert>

            <FieldGroup>
              <Field orientation='horizontal'>
                <FieldContent>
                  <FieldTitle>{t('Enable WeChat Pay Native')}</FieldTitle>
                  <FieldDescription>
                    {t(
                      'Disabling this only blocks new orders; existing callbacks remain active.'
                    )}
                  </FieldDescription>
                </FieldContent>
                <Switch
                  checked={settings.enabled}
                  onCheckedChange={(value) => updateField('enabled', value)}
                />
              </Field>

              <div className='grid gap-5 md:grid-cols-3'>
                <Field>
                  <FieldLabel htmlFor='wechatpay-app-id'>
                    {t('AppID')}
                  </FieldLabel>
                  <Input
                    id='wechatpay-app-id'
                    value={settings.app_id}
                    onChange={(event) =>
                      updateField('app_id', event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor='wechatpay-max-topup'>
                    {t('Maximum top-up amount')}
                  </FieldLabel>
                  <Input
                    id='wechatpay-max-topup'
                    type='number'
                    min={settings.min_topup}
                    max={4000}
                    value={settings.max_topup}
                    onChange={(event) =>
                      updateField('max_topup', Number(event.target.value))
                    }
                  />
                  <FieldDescription>
                    {t('The server hard limit is 4000 billing units.')}
                  </FieldDescription>
                </Field>
                <Field>
                  <FieldLabel htmlFor='wechatpay-mch-id'>
                    {t('Merchant ID')}
                  </FieldLabel>
                  <Input
                    id='wechatpay-mch-id'
                    value={settings.mch_id}
                    onChange={(event) =>
                      updateField('mch_id', event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor='wechatpay-cert-serial'>
                    {t('Merchant certificate serial number')}
                  </FieldLabel>
                  <Input
                    id='wechatpay-cert-serial'
                    value={settings.merchant_cert_serial_no}
                    onChange={(event) =>
                      updateField('merchant_cert_serial_no', event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor='wechatpay-public-key-id'>
                    {t('WeChat Pay public key ID')}
                  </FieldLabel>
                  <Input
                    id='wechatpay-public-key-id'
                    placeholder='PUB_KEY_ID_...'
                    value={settings.public_key_id}
                    onChange={(event) =>
                      updateField('public_key_id', event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor='wechatpay-min-topup'>
                    {t('Minimum top-up amount')}
                  </FieldLabel>
                  <Input
                    id='wechatpay-min-topup'
                    type='number'
                    min={1}
                    value={settings.min_topup}
                    onChange={(event) =>
                      updateField('min_topup', Number(event.target.value))
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor='wechatpay-expiration'>
                    {t('Order expiration (minutes)')}
                  </FieldLabel>
                  <Input
                    id='wechatpay-expiration'
                    type='number'
                    min={1}
                    max={120}
                    value={settings.order_expire_minutes}
                    onChange={(event) =>
                      updateField(
                        'order_expire_minutes',
                        Number(event.target.value)
                      )
                    }
                  />
                </Field>
              </div>

              <Field>
                <FieldLabel htmlFor='wechatpay-notify-url'>
                  {t('Custom callback URL')}
                </FieldLabel>
                <Input
                  id='wechatpay-notify-url'
                  type='url'
                  placeholder='https://example.com/api/wechatpay/notify'
                  value={settings.notify_url}
                  onChange={(event) =>
                    updateField('notify_url', event.target.value)
                  }
                />
                <FieldDescription>
                  {t(
                    'Must be a public HTTPS URL without query parameters. Leave blank to use the server callback address.'
                  )}
                </FieldDescription>
              </Field>

              <div className='grid gap-5 lg:grid-cols-2'>
                <SecretField
                  id='wechatpay-private-key'
                  label={t('Merchant private key (PKCS#8 PEM)')}
                  configured={settings.merchant_private_key_configured}
                  value={settings.merchant_private_key}
                  onChange={(value) =>
                    updateField('merchant_private_key', value)
                  }
                  onClear={() => void clearSecret('merchant_private_key')}
                  t={t}
                  disabled={!loaded || saving}
                />
                <SecretField
                  id='wechatpay-public-key'
                  label={t('WeChat Pay public key (PEM)')}
                  configured={settings.public_key_configured}
                  value={settings.public_key}
                  onChange={(value) => updateField('public_key', value)}
                  onClear={() => void clearSecret('public_key')}
                  t={t}
                  disabled={!loaded || saving}
                />
              </div>

              <SecretField
                id='wechatpay-api-v3-key'
                label={t('APIv3 key (32 bytes)')}
                configured={settings.api_v3_key_configured}
                value={settings.api_v3_key}
                onChange={(value) => updateField('api_v3_key', value)}
                onClear={() => void clearSecret('api_v3_key')}
                t={t}
                singleLine
                disabled={!loaded || saving}
              />
            </FieldGroup>
          </>
        )}
      </CardContent>

      <CardFooter className='justify-end border-t pt-6'>
        <Button
          onClick={() => void handleSave()}
          disabled={loading || saving || !loaded}
        >
          {saving && <Spinner className='mr-2' />}
          {t('Save WeChat Pay settings')}
        </Button>
      </CardFooter>
    </Card>
  )
}

interface SecretFieldProps {
  id: string
  label: string
  configured: boolean
  value: string
  onChange: (value: string) => void
  onClear: () => void
  t: (key: string) => string
  singleLine?: boolean
  disabled?: boolean
}

function SecretField(props: SecretFieldProps) {
  return (
    <Field>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <FieldLabel htmlFor={props.id}>{props.label}</FieldLabel>
        <div className='flex items-center gap-2'>
          <Badge variant={props.configured ? 'default' : 'outline'}>
            {props.configured
              ? props.t('Configured')
              : props.t('Not configured')}
          </Badge>
          {props.configured && (
            <Button
              type='button'
              size='sm'
              variant='destructive'
              onClick={props.onClear}
              disabled={props.disabled}
            >
              {props.t('Clear')}
            </Button>
          )}
        </div>
      </div>
      {props.singleLine ? (
        <Input
          id={props.id}
          type='password'
          autoComplete='new-password'
          value={props.value}
          placeholder={props.t('Leave blank to keep the existing credential')}
          onChange={(event) => props.onChange(event.target.value)}
          disabled={props.disabled}
        />
      ) : (
        <Textarea
          id={props.id}
          rows={7}
          autoComplete='off'
          value={props.value}
          placeholder={props.t('Leave blank to keep the existing credential')}
          onChange={(event) => props.onChange(event.target.value)}
          className='font-mono text-xs'
          disabled={props.disabled}
        />
      )}
      <FieldDescription>
        {props.t(
          'Stored credentials are encrypted and never returned by the API.'
        )}
      </FieldDescription>
    </Field>
  )
}
