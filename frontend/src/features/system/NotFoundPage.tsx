import { useNavigate } from 'react-router';
import { AppShell } from '../../shared/ui/AppShell';
import { Button } from '../../shared/ui/Button';
import { EmptyView } from '../../shared/ui/StateViews';

/** Неизвестный адрес внутри приложения. */
export function NotFoundPage() {
  const navigate = useNavigate();
  return (
    <AppShell title="Не найдено">
      <EmptyView
        art="search"
        title="Здесь ничего нет"
        description="Страница недоступна или объект удалён."
        action={
          <Button variant="secondary" onClick={() => navigate('/', { replace: true })}>
            На главную
          </Button>
        }
      />
    </AppShell>
  );
}
