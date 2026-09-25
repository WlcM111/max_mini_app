import { useState } from 'react';
import { Button } from '@maxhub/max-ui';
import { useMutation } from '@tanstack/react-query';
import { messageForError } from '../../api/errors';
import type { CalendarExport } from '../../api/client';
import { downloadCalendar } from '../../platform/max/download';
import { createCalendarExport } from './api';

/**
 * Экспорт в календарь двумя шагами: сначала готовится одноразовая ссылка,
 * затем скачивание вызывается синхронно в обработчике клика (spec §6).
 */
export function CalendarExportButton({ organizationId }: { organizationId: string }) {
  const [link, setLink] = useState<CalendarExport | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const prepare = useMutation({
    mutationFn: () => createCalendarExport(organizationId),
    onSuccess: (data) => {
      setLink(data);
      setNotice('Ссылка готова, действует 10 минут');
    },
    onError: (error) => setNotice(messageForError(error)),
  });

  const download = () => {
    if (!link) return;
    void downloadCalendar(link.download_url, link.file_name).then((outcome) => {
      if (outcome === 'failed') setNotice('Не удалось открыть файл. Скопируйте ссылку и откройте в браузере.');
      else if (outcome === 'opened') setNotice('Файл открыт в браузере');
      else setNotice('Файл передан в клиент MAX');
    });
  };

  return (
    <div className="stack stack--tight">
      {link ? (
        <Button size="large" stretched variant="secondary" onClick={download}>
          Скачать {link.file_name}
        </Button>
      ) : (
        <Button
          size="large"
          stretched
          variant="secondary"
          loading={prepare.isPending}
          disabled={prepare.isPending}
          onClick={() => prepare.mutate()}
        >
          Подготовить файл календаря
        </Button>
      )}
      {notice ? (
        <p className="muted" aria-live="polite">
          {notice}
        </p>
      ) : null}
    </div>
  );
}
