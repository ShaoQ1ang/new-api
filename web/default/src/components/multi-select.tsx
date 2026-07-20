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
import * as React from 'react'
import { Command as CommandPrimitive } from 'cmdk'
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Command, CommandGroup, CommandItem } from '@/components/ui/command'

export type Option = {
  label: string
  value: string
}

interface MultiSelectProps {
  options: Option[]
  selected: string[]
  onChange: (values: string[]) => void
  placeholder?: string
  className?: string
}

export function MultiSelect({
  options,
  selected,
  onChange,
  placeholder,
  className,
}: MultiSelectProps) {
  const { t } = useTranslation()
  const resolvedPlaceholder = placeholder ?? t('Select items...')
  const inputRef = React.useRef<HTMLInputElement>(null)
  const [open, setOpen] = React.useState(false)
  const [inputValue, setInputValue] = React.useState('')

  const selectedSet = React.useMemo(
    () => new Set(props.selected),
    [props.selected]
  )

  // Lookup of value -> display label so chips and items can show friendly names
  // even when the underlying option list changes (e.g. custom-added values).
  const labelMap = React.useMemo(() => {
    const map = new Map<string, string>()
    for (const option of props.options) {
      map.set(option.value, option.label)
    }
    return map
  }, [props.options])

  const trimmedInput = inputValue.trim()
  const inputMatchesExisting =
    trimmedInput.length > 0 &&
    (selectedSet.has(trimmedInput) ||
      props.options.some(
        (option) =>
          option.value === trimmedInput || option.label === trimmedInput
      ))

  const canCreate =
    props.allowCreate === true &&
    trimmedInput.length > 0 &&
    !inputMatchesExisting

  // We expose all known option values + every currently selected value to Base
  // UI's items list. This way Base UI filters them by the search query and the
  // user can still see the chip labels mapped correctly.
  const items = React.useMemo(() => {
    const set = new Set<string>(props.options.map((option) => option.value))
    for (const value of props.selected) {
      set.add(value)
    }
    if (canCreate) {
      set.add(trimmedInput)
    }
    return Array.from(set)
  }, [props.options, props.selected, canCreate, trimmedInput])

  const addValues = React.useCallback(
    (values: string[]) => {
      const next: string[] = []
      const seen = new Set<string>(props.selected)
      for (const raw of values) {
        const value = raw.trim()
        if (!value) continue
        if (seen.has(value)) continue
        seen.add(value)
        next.push(value)
      }
      if (next.length === 0) return
      props.onChange([...props.selected, ...next])
    },
    [props]
  )

  const handleInputValueChange = (value: string) => {
    if (!props.allowCreate) {
      setInputValue(value)
      return
    }
    const parsed = splitDraft(value)
    if (parsed.completed.length > 0) {
      addValues(parsed.completed)
      setInputValue(parsed.draft)
      return
    }
    setInputValue(value)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const input = inputRef.current
    if (input) {
      if (e.key === 'Delete' || e.key === 'Backspace') {
        if (input.value === '' && selected.length > 0) {
          onChange(selected.slice(0, -1))
        }
      }
      if (e.key === 'Escape') {
        input.blur()
      }
    }
  }

  const selectables = options.filter(
    (option) => !selected.includes(option.value)
  )

  return (
    <Command
      onKeyDown={handleKeyDown}
      className={`overflow-visible bg-transparent ${className || ''}`}
    >
      <div className='group border-input ring-offset-background focus-within:ring-ring rounded-md border px-3 py-2 text-sm focus-within:ring-2 focus-within:ring-offset-2'>
        <div className='flex flex-wrap gap-1'>
          {selected.map((value) => {
            const option = options.find((o) => o.value === value)
            return (
              <Badge key={value} variant='secondary'>
                {option?.label || value}
                <Button
                  variant='ghost'
                  size='icon-sm'
                  aria-label='Remove'
                  className='ml-1 size-auto p-0'
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      handleUnselect(value)
                    }
                  }}
                  onMouseDown={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                  }}
                  onClick={() => handleUnselect(value)}
                >
                  <X
                    className='text-muted-foreground hover:text-foreground h-3 w-3'
                    aria-hidden='true'
                  />
                </Button>
              </Badge>
            )
          })}
          <CommandPrimitive.Input
            ref={inputRef}
            value={inputValue}
            onValueChange={setInputValue}
            onBlur={() => setOpen(false)}
            onFocus={() => setOpen(true)}
            placeholder={selected.length === 0 ? resolvedPlaceholder : ''}
            className='placeholder:text-muted-foreground flex-1 bg-transparent outline-none'
          />
        </div>
      </div>
      <div className='relative'>
        {open && selectables.length > 0 ? (
          <div className='bg-popover text-popover-foreground animate-in absolute top-0 z-10 w-full rounded-md border shadow-md outline-none'>
            <CommandGroup className='h-full max-h-60 overflow-auto'>
              {selectables.map((option) => {
                return (
                  <CommandItem
                    key={option.value}
                    onMouseDown={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                    }}
                    onSelect={() => {
                      setInputValue('')
                      onChange([...selected, option.value])
                    }}
                    className='cursor-pointer'
                  >
                    {option.label}
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </div>
        ) : null}
      </div>
    </Command>
  )
}
