import { Button } from '@maxhub/max-ui';
import { useNavigate } from 'react-router';
import { AppShell } from '../../shared/ui/AppShell';

/** Приветствие: ценность продукта и уведомление об обработке данных (NFR-06). */
export function WelcomePage() {
  const navigate = useNavigate();
  return (
    <AppShell
      title="Вовремя"
      subtitle="Сроки документов под контролем"
      actions={
        <Button size="large" stretched onClick={() => navigate('/onboarding/organization')}>
          Продолжить
        </Button>
      }
    >
      <div className="card stack stack--tight">
        <h2 className="card__title">Что делает приложение</h2>
        <p className="card__text">
          Храните сроки лицензий, договоров и медосмотров в одном месте. Бот напомнит заранее — в чате MAX,
          без таблиц и бумажных напоминалок.
        </p>
      </div>
      <div className="card stack stack--tight">
        <h2 className="card__title">Как это работает</h2>
        <p className="card__text">1. Опишите организацию — подберём типовые документы.</p>
        <p className="card__text">2. Укажите сроки окончания.</p>
        <p className="card__text">3. Получайте напоминания и продлевайте вовремя.</p>
      </div>
      <p className="muted">
        Мы сохраняем только имя из профиля MAX и сведения о документах, которые вы вносите сами. Удалить данные
        можно в разделе «Аккаунт».
      </p>
    </AppShell>
  );
}
