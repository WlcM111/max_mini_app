import { Avatar } from './Avatar';
import { BrandMark } from './BrandMark';
import { Button } from './Button';
import { Icon, type IconName } from './Icon';

export type TabId = 'home' | 'documents' | 'members' | 'settings';

const TABS: { id: TabId; label: string; icon: IconName }[] = [
  { id: 'home', label: 'Главная', icon: 'home' },
  { id: 'documents', label: 'Документы', icon: 'docs' },
  { id: 'members', label: 'Участники', icon: 'people' },
  { id: 'settings', label: 'Настройки', icon: 'settings' },
];

interface Props {
  active: TabId | null;
  organizationName?: string | undefined;
  userName: string;
  userInitials: string;
  userSeed: string;
  canAdd: boolean;
  onSelect: (tab: TabId) => void;
  onAdd: () => void;
  onAccount: () => void;
  onHome: () => void;
}

/** Навигация: плавающая нижняя панель на телефоне, боковое меню на компьютере (от 1024 px). */
export function TabBar({ active, organizationName, userName, userInitials, userSeed, canAdd, onSelect, onAdd, onAccount, onHome }: Props) {
  const index = Math.max(0, TABS.findIndex((tab) => tab.id === active));
  return (
    <nav className="tabbar" aria-label="Разделы" data-layer="bottom">
      <button type="button" className="tabbar__brand" aria-label="На главную" onClick={onHome}>
        <BrandMark size={38} />
        <span className="tabbar__brand-text">
          <span className="tabbar__brand-name">Вовремя</span>
          {organizationName ? <span className="tabbar__org">{organizationName}</span> : null}
        </span>
      </button>
      {canAdd ? (
        <Button className="tabbar__add" icon="plus" onClick={onAdd}>
          Добавить документ
        </Button>
      ) : null}
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
      <button type="button" className="tabbar__account" onClick={onAccount}>
        <Avatar text={userInitials} seed={userSeed} />
        <span className="tabbar__account-text">
          <span className="tabbar__account-name">{userName}</span>
          <span className="tabbar__account-hint">Аккаунт</span>
        </span>
      </button>
    </nav>
  );
}
