/** Пометка модельных данных справочника (BR-04, ADR-016). */
export function ModelDataBadge({ compact = false }: { compact?: boolean }) {
  return (
    <p className={compact ? 'muted' : 'banner'}>
      Модельные данные справочника — сверяйте сроки с документом.
    </p>
  );
}
