import { useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DateField } from './DateField';

function Harness({ initial = null, min, onChange }: { initial?: string | null; min?: string; onChange?: (value: string | null) => void }) {
  const [value, setValue] = useState<string | null>(initial);
  return (
    <DateField
      label="Действует до"
      value={value}
      min={min}
      onChange={(next) => {
        setValue(next);
        onChange?.(next);
      }}
    />
  );
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(new Date(2026, 8, 24, 12));
});
afterEach(() => {
  vi.useRealTimers();
});

describe('T-FE-DATE: выбор даты в календаре приложения', () => {
  it('открывает собственный календарь, а не системный выбор даты', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const field = screen.getByLabelText('Действует до');
    expect(field).toHaveAttribute('type', 'text');
    expect(field).toHaveAttribute('readonly');

    await user.click(field);
    const dialog = screen.getByRole('dialog', { name: 'Действует до' });
    expect(within(dialog).getByRole('button', { name: 'Сентябрь 2026: выбрать месяц и год' })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: /^24 сентября 2026/ })).toHaveAttribute('aria-current', 'date');

    await user.click(within(dialog).getByRole('button', { name: /^15 сентября 2026/ }));
    expect(field).toHaveValue('15.09.2026');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('дни раньше минимальной даты недоступны', async () => {
    const user = userEvent.setup();
    render(<Harness min="2026-09-20" />);
    await user.click(screen.getByLabelText('Действует до'));
    expect(screen.getByRole('button', { name: /^19 сентября 2026/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /^20 сентября 2026/ })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Предыдущий месяц' })).toBeDisabled();
  });

  it('принимает дату, введённую вручную, и проверяет её', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    await user.click(screen.getByLabelText('Действует до'));
    const typed = screen.getByLabelText('Дата вручную');

    await user.type(typed, '31022027');
    await user.keyboard('{Enter}');
    expect(screen.getByText('Введите дату в формате ДД.ММ.ГГГГ')).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();

    await user.clear(typed);
    await user.type(typed, '31012027');
    expect(typed).toHaveValue('31.01.2027');
    expect(screen.getByRole('button', { name: 'Январь 2027: выбрать месяц и год' })).toBeInTheDocument();
    await user.keyboard('{Enter}');
    expect(onChange).toHaveBeenCalledWith('2027-01-31');
  });

  it('переходит к месяцу другого года через выбор месяца', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const field = screen.getByLabelText('Действует до');
    await user.click(field);
    await user.click(screen.getByRole('button', { name: 'Сентябрь 2026: выбрать месяц и год' }));
    await user.click(screen.getByRole('button', { name: 'Следующий год' }));
    await user.click(screen.getByRole('button', { name: 'Март 2027' }));
    await user.click(screen.getByRole('button', { name: /^13 марта 2027/ }));
    expect(field).toHaveValue('13.03.2027');
  });

  it('управляется с клавиатуры', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const field = screen.getByLabelText('Действует до');
    field.focus();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('button', { name: /^24 сентября 2026/ })).toHaveFocus();
    await user.keyboard('{ArrowRight}{ArrowDown}');
    expect(screen.getByRole('button', { name: /^2 октября 2026/ })).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(field).toHaveValue('02.10.2026');
  });

  it('очищает выбранную дату', async () => {
    const user = userEvent.setup();
    render(<Harness initial="2026-10-01" />);
    const field = screen.getByLabelText('Действует до');
    expect(field).toHaveValue('01.10.2026');
    await user.click(screen.getByRole('button', { name: 'Очистить дату' }));
    expect(field).toHaveValue('');
  });
});
