import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusBadge } from './StatusBadge';

describe('T-FE-CMP: значок статуса срока', () => {
  it('показывает текст для всех четырёх статусов', () => {
    const { rerender } = render(<StatusBadge status="expired" daysLeft={-3} />);
    expect(screen.getByText(/Просрочен/)).toBeInTheDocument();
    expect(screen.getByText(/просрочен на 3 дня/)).toBeInTheDocument();

    rerender(<StatusBadge status="expiring" daysLeft={7} />);
    expect(screen.getByText(/Скоро истекает/)).toBeInTheDocument();
    expect(screen.getByText(/осталось 7 дней/)).toBeInTheDocument();

    rerender(<StatusBadge status="valid" daysLeft={200} />);
    expect(screen.getByText(/В порядке/)).toBeInTheDocument();

    rerender(<StatusBadge status="no_expiry" daysLeft={null} />);
    expect(screen.getByText(/Бессрочный/)).toBeInTheDocument();
  });
});
