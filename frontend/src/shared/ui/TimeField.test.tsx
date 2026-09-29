import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { notifyTimeOptions } from '../lib/validation';
import { TimeField } from './TimeField';

function Harness({ onChange, disabled = false }: { onChange: (value: string) => void; disabled?: boolean }) {
  const [value, setValue] = useState('09:00');
  return (
    <TimeField
      label="Время напоминаний"
      value={value}
      options={notifyTimeOptions()}
      disabled={disabled}
      onChange={(next) => {
        setValue(next);
        onChange(next);
      }}
    />
  );
}

describe('T-FE-TIME: выбор времени напоминаний', () => {
  it('показывает время сеткой по частям суток и сохраняет выбор', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    const field = screen.getByLabelText('Время напоминаний');
    expect(field).toHaveValue('09:00');

    await user.click(field);
    const dialog = screen.getByRole('dialog', { name: 'Время напоминаний' });
    expect(within(dialog).getByRole('group', { name: 'Утро' })).toBeInTheDocument();
    expect(within(within(dialog).getByRole('group', { name: 'Вечер' })).getAllByRole('button')).toHaveLength(9);
    const current = within(dialog).getByRole('button', { name: '09:00' });
    expect(current).toHaveAttribute('aria-pressed', 'true');
    expect(current).toHaveFocus();

    await user.click(within(dialog).getByRole('button', { name: '18:30' }));
    expect(onChange).toHaveBeenCalledWith('18:30');
    expect(field).toHaveValue('18:30');
  });

  it('повторный выбор того же времени ничего не сохраняет', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    await user.click(screen.getByLabelText('Время напоминаний'));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '09:00' }));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('выключенное поле не открывает выбор', async () => {
    const user = userEvent.setup();
    render(<Harness onChange={vi.fn()} disabled />);
    await user.click(screen.getByLabelText('Время напоминаний'));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
