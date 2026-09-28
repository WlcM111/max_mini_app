import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { messageForError } from '../../api/errors';
import type { CalendarExport } from '../../api/client';
import { downloadCalendar } from '../../platform/max/download';
import { Button } from '../../shared/ui/Button';
import { Icon } from '../../shared/ui/Icon';
import { toast } from '../../shared/ui/Toast';
import { createCalendarExport } from './api';

/**
 * Экспорт в календарь двумя шагами: сначала готовится одноразовая ссылка,
 * затем скачивание вызывается синхронно в обработчике клика (spec §6).
 */
export function CalendarExportButton({ organizationId }: { organizationId: string }) {
  const [link, setLink] = useState<CalendarExport | null>(null);
  const prepare = useMutation({
    mutationFn: () => createCalendarExport(organizationId),
    onSuccess: (data) => setLink(data),
    onError: (error) => toast(messageForError(error), 'error'),
  });

  const download = () => {
    if (!link) return;
    void downloadCalendar(link.download_url, link.file_name).then((outcome) => {
      if (outcome === 'failed') toast('Не удалось открыть файл. Подготовьте его заново.', 'error');
      else if (outcome === 'opened') toast('Файл открыт в браузере', 'success');
      else toast('Файл передан в клиент MAX', 'success');
    });
  };

  return (
    <div className="export">
      {link ? (
        <>
          <div className="export__file">
            <Icon name="calendar" />
            <span>{link.file_name}</span>
          </div>
          <Button size="l" stretched icon="download" onClick={download}>
            Скачать файл
          </Button>
          <p className="field__hint">Ссылка действует 10 минут</p>
        </>
      ) : (
        <Button variant="secondary" size="l" stretched icon="calendar" loading={prepare.isPending} onClick={() => prepare.mutate()}>
          Подготовить файл календаря
        </Button>
      )}
    </div>
  );
}
