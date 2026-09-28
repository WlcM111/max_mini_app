import type { DocumentListItem as Item } from '../../api/client';
import { relativeDays } from '../../shared/lib/format';
import { DateLeaf } from '../../shared/ui/DateLeaf';
import { Icon } from '../../shared/ui/Icon';
import { StatusBadge } from '../../shared/ui/StatusBadge';

/** Строка реестра: листок календаря, название, статус, срок и ответственный. */
export function DocumentListItem({ item, onOpen, responsibleName }: { item: Item; onOpen: (id: string) => void; responsibleName?: string | undefined }) {
  const person = responsibleName || item.responsible_label;
  const when = relativeDays(item.days_left);
  const extra = person || item.reminders_state !== 'actual';
  return (
    <button type="button" className={`list-item doc-row doc-row--${item.status}`} onClick={() => onOpen(item.id)}>
      <DateLeaf date={item.valid_until} status={item.status} />
      <span className="doc-row__body">
        <span className="doc-row__title">{item.title}</span>
        <span className="doc-row__meta">
          <StatusBadge status={item.status} daysLeft={item.days_left} withDays={false} />
          {when ? <span className="doc-row__when">{when}</span> : null}
        </span>
        {extra ? (
          <span className="doc-row__sub">
            {person ? (
              <span className="doc-row__person">
                <Icon name="user" size={14} />
                {person}
              </span>
            ) : null}
            {item.reminders_state === 'pending' ? (
              <span className="doc-row__note">
                <Icon name="refresh" size={14} />
                напоминания пересчитываются
              </span>
            ) : null}
            {item.reminders_state === 'unavailable' ? (
              <span className="doc-row__note">
                <Icon name="alert" size={14} />
                план напоминаний недоступен
              </span>
            ) : null}
          </span>
        ) : null}
      </span>
      <Icon name="chevron-right" className="doc-row__chev" />
    </button>
  );
}
