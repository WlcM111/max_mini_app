import { BrandMark } from './BrandMark';
import { Icon, type IconName } from './Icon';

export type TabId = 'home' | 'documents' | 'members' | 'settings';

const TABS: { id: TabId; label: string; icon: IconName }[] = [
  { id: 'home', label: 'Главная', icon: 'home' },
  { id: 'documents', label: 'Документы', icon: 'docs' },
  { id: 'members', label: 'Участники', icon: 'people' },
  { id: 'settings', label: 'Настройки', icon: 'settings' },
];

interface Props {
  active: TabId;
  organizationName?: string | undefined;
  onSelect: (tab: TabId) => void;
}

/** Нижняя навигация разделов; на широком экране — боковая панель. */
export function TabBar({ active, organizationName, onSelect }: Props) {
  const index = Math.max(0, TABS.findIndex((tab) => tab.id === active));
  return (
    <nav className="tabbar" aria-label="Разделы" data-layer="bottom">
      <div className="tabbar__brand">
        <BrandMark size={36} />
        <span className="tabbar__brand-text">
          <span className="tabbar__brand-name">Вовремя</span>
          {organizationName ? <span className="tabbar__org">{organizationName}</span> : null}
        </span>
      </div>
      <div className="tabbar__inner">
        <span className="tabbar__pill" style={{ transform: `translateX(${index * 100}%)` }} aria-hidden="true" />
        {TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            className="tabbar__item"
            aria-current={tab.id === active ? 'page' : undefined}
            onClick={() => onSelect(tab.id)}
          >
            <span className="tabbar__icon">
              <Icon name={tab.icon} size={24} />
            </span>
            <span className="tabbar__label">{tab.label}</span>
          </button>
        ))}
      </div>
    </nav>
  );
}
