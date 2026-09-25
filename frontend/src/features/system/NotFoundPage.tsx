import { Button } from '@maxhub/max-ui';
import { useNavigate } from 'react-router';
import { AppShell } from '../../shared/ui/AppShell';

/** Неизвестный маршрут или недоступный объект. */
export function NotFoundPage() {
  const navigate = useNavigate();
  return (
    <AppShell title="Не найдено">
      <p className="card__text">Страница недоступна или объект удалён.</p>
      <Button size="large" stretched onClick={() => navigate('/', { replace: true })}>
        На главную
      </Button>
    </AppShell>
  );
}
