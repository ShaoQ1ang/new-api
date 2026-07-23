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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Pencil } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import {
  MANAGEMENT_PERMISSION,
  type ManagementPermission,
} from '@/lib/management-permissions'
import { ROLE } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
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
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import {
  SecureVerificationDialog,
  useSecureVerification,
  type VerificationMethod,
} from '@/features/auth/secure-verification'
import {
  createUser,
  updateUser,
  getUser,
  getGroups,
  getUserManagementPermissions,
  updateUserManagementPermissions,
} from '../api'
import { BINDING_FIELDS, ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  userFormSchema,
  type UserFormValues,
  USER_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformUserToFormDefaults,
} from '../lib'
import { type User } from '../types'
import { UserQuotaDialog } from './user-quota-dialog'
import { useUsers } from './users-provider'

type UsersMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: User
}

export function UsersMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: UsersMutateDrawerProps) {
  const { t } = useTranslation()
  const authUser = useAuthStore((state) => state.auth.user)
  const isUpdate = !!currentRow
  const canAssignManagementPermissions =
    isUpdate && authUser?.role === ROLE.SUPER_ADMIN
  const { triggerRefresh } = useUsers()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [quotaDialogOpen, setQuotaDialogOpen] = useState(false)
  const [managementPermissions, setManagementPermissions] = useState<
    ManagementPermission[]
  >([])
  const [managementPermissionRole, setManagementPermissionRole] = useState<
    number | null
  >(null)
  const [managementPermissionsLoading, setManagementPermissionsLoading] =
    useState(false)
  const [managementPermissionsSaving, setManagementPermissionsSaving] =
    useState(false)

  const {
    open: verificationOpen,
    setOpen: setVerificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    cancel: cancelVerification,
    setCode,
    switchMethod,
    withVerification,
  } = useSecureVerification()

  // Fetch groups
  const { data: groupsData } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
    staleTime: 5 * 60 * 1000,
  })

  const groups = groupsData?.data || []

  const form = useForm<UserFormValues>({
    resolver: zodResolver(userFormSchema),
    defaultValues: USER_FORM_DEFAULT_VALUES,
  })

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      // For update, fetch fresh data
      getUser(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformUserToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      // For create, reset to defaults
      form.reset(USER_FORM_DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

  useEffect(() => {
    let active = true
    if (!open || !canAssignManagementPermissions || !currentRow) {
      setManagementPermissions([])
      setManagementPermissionRole(null)
      return () => {
        active = false
      }
    }

    setManagementPermissionsLoading(true)
    getUserManagementPermissions(currentRow.id)
      .then((result) => {
        if (!active) return
        if (!result.success || !result.data) {
          throw new Error(
            result.message || t('Failed to load management permissions')
          )
        }
        setManagementPermissions(result.data.permissions)
        setManagementPermissionRole(result.data.role)
      })
      .catch((error: unknown) => {
        if (!active) return
        toast.error(
          error instanceof Error
            ? error.message
            : t('Failed to load management permissions')
        )
      })
      .finally(() => {
        if (active) setManagementPermissionsLoading(false)
      })

    return () => {
      active = false
    }
  }, [canAssignManagementPermissions, currentRow, open, t])

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'

  const currentQuotaRaw = form.watch('quota_dollars') || 0

  const onSubmit = async (data: UserFormValues) => {
    setIsSubmitting(true)
    try {
      const payload = transformFormDataToPayload(data, currentRow?.id)
      const result = isUpdate
        ? await updateUser(payload as typeof payload & { id: number })
        : await createUser(payload)

      if (result.success) {
        toast.success(
          isUpdate
            ? t(SUCCESS_MESSAGES.USER_UPDATED)
            : t(SUCCESS_MESSAGES.USER_CREATED)
        )
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(
          result.message ||
            (isUpdate
              ? t(ERROR_MESSAGES.UPDATE_FAILED)
              : t(ERROR_MESSAGES.CREATE_FAILED))
        )
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsSubmitting(false)
    }
  }

  const refreshUserData = async () => {
    if (!currentRow) return
    const result = await getUser(currentRow.id)
    if (result.success && result.data) {
      form.reset(transformUserToFormDefaults(result.data))
    }
    triggerRefresh()
  }

  const toggleManagementPermission = (
    permission: ManagementPermission,
    checked: boolean
  ) => {
    setManagementPermissions((current) =>
      checked
        ? Array.from(new Set([...current, permission]))
        : current.filter((item) => item !== permission)
    )
  }

  const persistManagementPermissions = async () => {
    if (!currentRow) return
    setManagementPermissionsSaving(true)
    try {
      const result = await updateUserManagementPermissions(
        currentRow.id,
        managementPermissions
      )
      if (!result.success || !result.data) {
        throw new Error(
          result.message || t('Failed to save management permissions')
        )
      }
      setManagementPermissions(result.data.permissions)
      setManagementPermissionRole(result.data.role)
      toast.success(t('Management permissions saved'))
    } finally {
      setManagementPermissionsSaving(false)
    }
  }

  const saveManagementPermissions = async () => {
    try {
      await withVerification(persistManagementPermissions, {
        title: t('Security verification'),
        description: t(
          'Confirm your identity before changing management permissions.'
        ),
      })
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save management permissions')
      )
    }
  }

  const handleVerification = async (
    method: VerificationMethod,
    code?: string
  ) => {
    try {
      await executeVerification(method, code)
    } catch {
      // The verification hook displays the actionable error.
    }
  }

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(v) => {
          onOpenChange(v)
          if (!v) {
            form.reset()
          }
        }}
      >
        <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]'>
          <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
            <SheetTitle>
              {isUpdate ? t('Update') : t('Create')} {t('User')}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t('Update the user by providing necessary info.')
                : t('Add a new user by providing necessary info.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='user-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
            >
              {/* Basic Information */}
              <div className='space-y-4'>
                <h3 className='text-sm font-medium'>
                  {t('Basic Information')}
                </h3>

                <FormField
                  control={form.control}
                  name='username'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Username')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Enter username')}
                          disabled={isUpdate}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {!isUpdate && (
                  <FormField
                    control={form.control}
                    name='role'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Role')}</FormLabel>
                        <Select
                          items={[
                            { value: '1', label: t('Common User') },
                            { value: '10', label: t('Admin') },
                          ]}
                          onValueChange={(value) =>
                            value !== null && field.onChange(parseInt(value))
                          }
                          value={String(field.value)}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t('Select a role')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='1'>
                                {t('Common User')}
                              </SelectItem>
                              <SelectItem value='10'>{t('Admin')}</SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t("Set the user's role (cannot be Root)")}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <FormField
                  control={form.control}
                  name='display_name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Display Name')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Enter display name')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Leave empty to use username')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='password'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Password')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='password'
                          placeholder={
                            isUpdate
                              ? t('Leave empty to keep unchanged')
                              : t('Enter password (min 8 characters)')
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              {/* Group & Quota Settings (Update only) */}
              {isUpdate && (
                <div className='space-y-4'>
                  <h3 className='text-sm font-medium'>{t('Group & Quota')}</h3>

                  <FormField
                    control={form.control}
                    name='group'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Group')}</FormLabel>
                        <Select
                          items={[
                            ...groups.map((group) => ({
                              value: group,
                              label: group,
                            })),
                          ]}
                          onValueChange={field.onChange}
                          value={field.value}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t('Select a group')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {groups.map((group) => (
                                <SelectItem key={group} value={group}>
                                  {group}
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
                    name='quota_dollars'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Remaining Quota ({{currency}})', {
                            currency: currencyLabel,
                          })}
                        </FormLabel>
                        <div className='flex gap-2'>
                          <FormControl>
                            <Input
                              value={
                                tokensOnly
                                  ? String(field.value || 0)
                                  : (field.value || 0).toFixed(6)
                              }
                              readOnly
                              className='flex-1'
                            />
                          </FormControl>
                          <Button
                            type='button'
                            variant='outline'
                            onClick={() => setQuotaDialogOpen(true)}
                          >
                            <Pencil className='mr-1 h-4 w-4' />
                            {t('Adjust Quota')}
                          </Button>
                        </div>
                        <FormDescription>
                          {formatQuota(parseQuotaFromDollars(field.value || 0))}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='remark'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Remark')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            placeholder={t(
                              'Admin notes (only visible to admins)'
                            )}
                            rows={3}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}

              {canAssignManagementPermissions && (
                <div className='space-y-4'>
                  <div>
                    <h3 className='text-sm font-medium'>
                      {t('Management Permissions')}
                    </h3>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t(
                        'Grant individual management capabilities without promoting this user to administrator.'
                      )}
                    </p>
                  </div>

                  {managementPermissionRole !== null &&
                  managementPermissionRole !== ROLE.USER ? (
                    <p className='text-muted-foreground rounded-md border p-3 text-sm'>
                      {t(
                        'Administrators receive all management permissions automatically. Explicit permissions can only be assigned to common users.'
                      )}
                    </p>
                  ) : (
                    <FieldSet disabled={managementPermissionsLoading}>
                      <FieldLegend variant='label'>
                        {t('Available capabilities')}
                      </FieldLegend>
                      <FieldGroup data-slot='checkbox-group'>
                        {MANAGEMENT_PERMISSION_OPTIONS.map((option) => (
                          <Field
                            key={option.permission}
                            orientation='horizontal'
                          >
                            <Checkbox
                              id={`management-permission-${option.permission}`}
                              checked={managementPermissions.includes(
                                option.permission
                              )}
                              onCheckedChange={(checked) =>
                                toggleManagementPermission(
                                  option.permission,
                                  checked === true
                                )
                              }
                            />
                            <FieldContent>
                              <FieldLabel
                                htmlFor={`management-permission-${option.permission}`}
                              >
                                {t(option.label)}
                              </FieldLabel>
                              <FieldDescription>
                                {t(option.description)}
                              </FieldDescription>
                            </FieldContent>
                          </Field>
                        ))}
                      </FieldGroup>
                      <Button
                        type='button'
                        variant='outline'
                        disabled={
                          managementPermissionsLoading ||
                          managementPermissionsSaving
                        }
                        onClick={() => void saveManagementPermissions()}
                      >
                        {managementPermissionsSaving
                          ? t('Saving...')
                          : t('Save management permissions')}
                      </Button>
                    </FieldSet>
                  )}
                </div>
              )}

              {/* Binding Information (Read-only) */}
              {isUpdate && (
                <div className='space-y-4'>
                  <h3 className='text-sm font-medium'>
                    {t('Binding Information')}
                  </h3>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Third-party account bindings (read-only, managed by user in profile settings)'
                    )}
                  </p>

                  <div className='space-y-3'>
                    {BINDING_FIELDS.map(({ key, label }) => (
                      <div key={key}>
                        <Label className='text-muted-foreground text-xs'>
                          {t(label)}
                        </Label>
                        <Input
                          value={
                            (currentRow?.[key as keyof User] as string) || '-'
                          }
                          disabled
                          className='mt-1'
                        />
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </form>
          </Form>
          <SheetFooter className='grid grid-cols-2 gap-2 border-t px-4 py-3 sm:flex sm:px-6 sm:py-4'>
            <SheetClose render={<Button variant='outline' />}>
              {t('Close')}
            </SheetClose>
            <Button form='user-form' type='submit' disabled={isSubmitting}>
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* Adjust Quota Dialog */}
      {currentRow && (
        <UserQuotaDialog
          open={quotaDialogOpen}
          onOpenChange={setQuotaDialogOpen}
          userId={currentRow.id}
          currentQuota={parseQuotaFromDollars(currentQuotaRaw || 0)}
          onSuccess={refreshUserData}
        />
      )}

      <SecureVerificationDialog
        open={verificationOpen}
        onOpenChange={setVerificationOpen}
        methods={verificationMethods}
        state={verificationState}
        onVerify={handleVerification}
        onCancel={cancelVerification}
        onCodeChange={setCode}
        onMethodChange={switchMethod}
      />
    </>
  )
}

const MANAGEMENT_PERMISSION_OPTIONS: Array<{
  permission: ManagementPermission
  label: string
  description: string
}> = [
  {
    permission: MANAGEMENT_PERMISSION.SKILL_HUB_CONTENT,
    label: 'Skill Hub content management',
    description: 'Create, edit, publish, and delete skills and tags.',
  },
  {
    permission: MANAGEMENT_PERMISSION.SKILL_HUB_REPORTS,
    label: 'Skill Hub report management',
    description: 'Review and update Skill Hub reports.',
  },
  {
    permission: MANAGEMENT_PERMISSION.CHAT_MODELS,
    label: 'Chat model management',
    description: 'Manage models shown in chat model selectors.',
  },
  {
    permission: MANAGEMENT_PERMISSION.CLIENT_RELEASES,
    label: 'Client release management',
    description: 'Create, edit, upload, and delete client releases.',
  },
  {
    permission: MANAGEMENT_PERMISSION.CLIENT_RELEASES_PUBLISH,
    label: 'Client release publishing',
    description: 'Publish or unpublish existing client releases.',
  },
]
