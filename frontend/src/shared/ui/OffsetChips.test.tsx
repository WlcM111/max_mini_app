import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { OffsetChips } from './OffsetChips';

describe('T-FE-CMP: выбор отступов напоминаний', () => {
  it('добавляет и убирает значения', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<OffsetChips value={[30]} onChange={onChange} />);
    await user.click(screen.getByRole('button', { name: 'за 7 дней' }));
    expect(onChange).toHaveBeenCalledWith([30, 7]);
    await user.click(screen.getByRole('button', { name: 'за 30 дней' }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });

  it('не позволяет выбрать больше пяти напоминаний', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<OffsetChips value={[90, 60, 30, 14, 7]} onChange={onChange} />);
    await user.click(screen.getByRole('button', { name: 'за 3 дня' }));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toHaveTextContent('Не более 5 напоминаний');
  });

  it('принимает своё значение в днях', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<OffsetChips value={[]} onChange={onChange} />);
    await user.type(screen.getByLabelText('Своё количество дней'), '45');
    await user.click(screen.getByRole('button', { name: 'Добавить' }));
    expect(onChange).toHaveBeenCalledWith([45]);
  });
});
