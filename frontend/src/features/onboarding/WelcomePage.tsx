import { useNavigate } from 'react-router';
import { AppShell } from '../../shared/ui/AppShell';
import { BrandMark } from '../../shared/ui/BrandMark';
import { Button } from '../../shared/ui/Button';
import { DateLeaf } from '../../shared/ui/DateLeaf';
import { StatusBadge } from '../../shared/ui/StatusBadge';
import { shiftDate, todayInTimeZone } from '../../shared/lib/dates';

// Живой пример списка на первом экране: сразу видно, что делает приложение.
const PREVIEW = [
  { title: 'Ключ ФН онлайн-кассы', status: 'expired', shift: -3 },
  { title: 'Подпись руководителя (КЭП)', status: 'expiring', shift: 12 },
  { title: 'Договор аренды помещения', status: 'valid', shift: 140 },
] as const;

/** Первый экран: что умеет приложение и как начать. */
export function WelcomePage() {
  const navigate = useNavigate();
  const today = todayInTimeZone('Europe/Moscow');
  return (
    <AppShell
      title="Вовремя"
      bare
      actions={
        <Button size="l" stretched iconRight="arrow-right" onClick={() => navigate('/onboarding/organization')}>
          Продолжить
        </Button>
      }
    >
      <section className="welcome">
        <BrandMark size={56} />
        <h1 className="welcome__title">Сроки документов под контролем</h1>
        <p className="welcome__lead">
          Лицензии, подписи, касса, аренда и медосмотры — в одном списке. Бот заранее напомнит в чате MAX, чтобы ничего не просрочить.
        </p>
        <div className="preview" aria-hidden="true" data-decor="true">
          {PREVIEW.map((item) => (
            <div key={item.title} className="preview__row">
              <DateLeaf date={shiftDate(today, { days: item.shift })} status={item.status} />
              <div className="preview__body">
                <span className="preview__title">{item.title}</span>
                <StatusBadge status={item.status} withDays={false} />
              </div>
            </div>
          ))}
        </div>
      </section>
      <section className="group" aria-labelledby="how-title">
        <h2 className="group__title" id="how-title">
          Как это работает
        </h2>
        <ol className="how">
          <li className="how__step">
            <span className="how__num">1</span>
            <span className="how__text">
              <strong>Опишите организацию</strong> — подберём документы, которые обычно нужны такому бизнесу.
            </span>
          </li>
          <li className="how__step">
            <span className="how__num">2</span>
            <span className="how__text">
              <strong>Укажите сроки</strong> — можно вставить текст документа, поля заполнятся сами.
            </span>
          </li>
          <li className="how__step">
            <span className="how__num">3</span>
            <span className="how__text">
              <strong>Получайте напоминания</strong> в чате с ботом и приглашайте коллег.
            </span>
          </li>
        </ol>
      </section>
      <p className="fine-print">
        Приложение хранит имя из профиля MAX и сведения о документах, которые вы вносите. Удалить все данные можно в разделе «Аккаунт».
      </p>
    </AppShell>
  );
}
