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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { isApiSuccess, requestWechatPayPayment } from '../api'
import type { WechatPayPaymentResponse } from '../types'

export type WechatPayOrder = NonNullable<WechatPayPaymentResponse['data']>

export function useWechatPayPayment() {
  const { t } = useTranslation()
  const [processing, setProcessing] = useState(false)

  const createWechatPayOrder = useCallback(
    async (amount: number): Promise<WechatPayOrder | null> => {
      setProcessing(true)
      try {
        const response = await requestWechatPayPayment({
          amount: Math.floor(amount),
          payment_method: 'wechatpay_native',
        })
        if (!isApiSuccess(response) || !response.data?.code_url) {
          toast.error(response.message || t('Payment request failed'))
          return null
        }
        return response.data
      } catch {
        toast.error(t('Payment request failed'))
        return null
      } finally {
        setProcessing(false)
      }
    },
    [t]
  )

  return { processing, createWechatPayOrder }
}
