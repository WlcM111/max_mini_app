import type { DocumentListItem as Item } from '../../api/client';
import { StatusBadge } from '../../shared/ui/StatusBadge';
import { formatDate } from '../../shared/lib/dates';

/** Строка реестра: название, срок, состояние плана напоминаний. */
export function DocumentListItem({ item, onOpen }: { item: Item; onOpen: (id: string) => void }) {
  return (
    <button type="button" className="list-item" onClick={() => onOpen(item.id)}>
      <span className="grow">
        <span className="list-item__title truncate">{item.title}</span>
        <span className="list-item__meta">
          {item.valid_until ? `до ${formatDate(item.valid_until)}` : 'бессрочный'}
          {item.responsible_label ? ` · ${item.responsible_label}` : ''}
          {item.reminders_state === 'pending' ? ' · напоминания пересчитываются' : ''}
          {item.reminders_state === 'unavailable' ? ' · план временно недоступен' : ''}
        </span>
      </span>
      <StatusBadge status={item.status} daysLeft={item.days_left} withDays={false} />
    </button>
  );
}
