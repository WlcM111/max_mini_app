import { Banner } from './Banner';
import { Icon } from './Icon';

const TEXT = 'Модельные данные справочника — сверяйте сроки с документом.';

/** Предупреждение: сроки из справочника учебные и требуют проверки. */
export function ModelDataBadge({ compact = false }: { compact?: boolean }) {
  if (compact) {
    return (
      <p className="model-note">
        <Icon name="info" size={16} />
        {TEXT}
      </p>
    );
  }
  return (
    <Banner tone="info" icon="info">
      {TEXT}
    </Banner>
  );
}
