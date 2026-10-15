import { useEffect, useRef, useState } from 'react';
import { IconCheck, IconChevronDown } from '@tabler/icons-react';

export type CustomSelectOption = {
  value: string;
  label: string;
  disabled?: boolean;
  divider?: boolean;
  tone?: 'default' | 'accent';
};

type CustomSelectProps = {
  value: string;
  options: CustomSelectOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  ariaLabel?: string;
  disabled?: boolean;
  className?: string;
};

export function CustomSelect({ value, options, onChange, placeholder = 'Choose an option', ariaLabel, disabled, className = '' }: CustomSelectProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const selected = options.find((option) => option.value === value && !option.disabled && !option.divider);

  useEffect(() => {
    function closeOnOutsidePointer(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener('pointerdown', closeOnOutsidePointer);
    return () => document.removeEventListener('pointerdown', closeOnOutsidePointer);
  }, []);

  function handleTriggerKeyDown(event: React.KeyboardEvent<HTMLButtonElement>) {
    if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      setOpen(true);
    }
    if (event.key === 'Escape') setOpen(false);
  }

  function choose(option: CustomSelectOption) {
    if (option.disabled || option.divider) return;
    onChange(option.value);
    setOpen(false);
    requestAnimationFrame(() => triggerRef.current?.focus());
  }

  return <div className={`custom-select ${open ? 'is-open' : ''} ${className}`.trim()} ref={rootRef}>
    <button ref={triggerRef} className="custom-select-trigger" type="button" disabled={disabled || options.every((option) => option.disabled || option.divider)} aria-label={ariaLabel} aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen((current) => !current)} onKeyDown={handleTriggerKeyDown}>
      <span className={selected ? '' : 'placeholder'}>{selected?.label ?? placeholder}</span>
      <IconChevronDown size={15} aria-hidden="true" />
    </button>
    {open && <div className="custom-select-menu" role="listbox" aria-label={ariaLabel}>
      {options.map((option) => <button key={`${option.value}-${option.label}`} className={`custom-select-option ${option.divider ? 'divider' : ''} ${option.tone === 'accent' ? 'accent' : ''} ${option.value === value ? 'selected' : ''}`} type="button" role="option" aria-selected={option.value === value} disabled={option.disabled || option.divider} onClick={() => choose(option)}>
        <span>{option.label}</span>
        {option.value === value && !option.divider && <IconCheck size={14} aria-hidden="true" />}
      </button>)}
    </div>}
  </div>;
}
