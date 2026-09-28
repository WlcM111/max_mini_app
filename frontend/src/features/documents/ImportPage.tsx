import { useMemo, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router';
import type { DocumentCreate } from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { messageForError } from '../../api/errors';
import { AppShell } from '../../shared/ui/AppShell';
import { Banner } from '../../shared/ui/Banner';
import { Button } from '../../shared/ui/Button';
import { SelectField } from '../../shared/ui/Field';
import { Icon } from '../../shared/ui/Icon';
import { ToggleRow } from '../../shared/ui/Switch';
import { toast } from '../../shared/ui/Toast';
import { cx } from '../../shared/lib/cx';
import { formatDate } from '../../shared/lib/dates';
import { plural } from '../../shared/lib/plural';
import { columnLetter, readSpreadsheet } from '../../shared/lib/spreadsheet';
import { useSession } from '../../session/useSession';
import { createDocumentsBatch } from './api';
import { buildRows, detectMapping, IMPORT_FIELDS, type ImportField, type Mapping } from './importRows';

const BATCH = 30; // domain.MaxDocumentsPerBatch
const MAX_ROWS = 1000;
const PREVIEW = 100;
const DOCS: [string, string, string] = ['документ', 'документа', 'документов'];

/** Массовый импорт документов из .xlsx/.csv через пакетное создание. */
export function ImportPage() {
  const { orgId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { me } = useSession();
  const [fileName, setFileName] = useState('');
  const [table, setTable] = useState<string[][] | null>(null);
  const [hasHeader, setHasHeader] = useState(true);
  const [mapping, setMapping] = useState<Mapping | null>(null);
  const [excluded, setExcluded] = useState<Set<string>>(() => new Set());
  const [imported, setImported] = useState<Set<string>>(() => new Set());
  const [reading, setReading] = useState(false);
  const [readError, setReadError] = useState<string | null>(null);
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const counter = useRef({ done: 0, total: 0 });
  const organizationName = me.memberships.find((item) => item.organization_id === orgId)?.organization_name;

  const rows = useMemo(() => (table && mapping ? buildRows(table, mapping, hasHeader) : []), [table, mapping, hasHeader]);
  const pending = rows.filter((row) => row.body && !excluded.has(row.key) && !imported.has(row.key));
  const invalid = rows.filter((row) => !row.body).length;
  const width = table ? Math.max(0, ...table.slice(0, 50).map((cells) => cells.length)) : 0;
  const header = table?.[0] ?? [];
  const columnOptions = [
    { value: '-1', label: 'Не загружать' },
    ...Array.from({ length: width }, (_, index) => ({
      value: String(index),
      label: `Столбец ${columnLetter(index)}${hasHeader && header[index] ? ` — ${header[index]}` : ''}`,
    })),
  ];

  const pick = async (file: File) => {
    setReading(true);
    setReadError(null);
    setFailure(null);
    try {
      const data = await readSpreadsheet(file);
      if (data.length === 0) throw new Error('В файле нет строк с данными');
      const detected = detectMapping(data);
      setTable(data.slice(0, MAX_ROWS + 1));
      setMapping(detected.mapping);
      setHasHeader(detected.hasHeader);
      setExcluded(new Set());
      setImported(new Set());
      setProgress(null);
      setFileName(file.name);
    } catch (error) {
      setReadError(error instanceof Error ? error.message : 'Не удалось прочитать файл');
    } finally {
      setReading(false);
    }
  };

  const run = useMutation({
    mutationFn: async () => {
      const queue = pending.filter((row): row is typeof row & { body: DocumentCreate } => row.body !== null);
      counter.current = { done: 0, total: queue.length };
      setProgress({ ...counter.current });
      for (let start = 0; start < queue.length; start += BATCH) {
        const chunk = queue.slice(start, start + BATCH);
        await createDocumentsBatch(
          orgId,
          chunk.map((row) => row.body),
        );
        counter.current.done += chunk.length;
        setImported((prev) => new Set([...prev, ...chunk.map((row) => row.key)]));
        setProgress({ ...counter.current });
      }
      return counter.current.done;
    },
    onSuccess: async (created) => {
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      await queryClient.invalidateQueries({ queryKey: queryKeys.organization(orgId) });
      toast(`Добавлено: ${created} ${plural(created, DOCS)}`, 'success');
    },
    onError: async (error) => {
      await queryClient.invalidateQueries({ queryKey: ['documents'] });
      setFailure(`${messageForError(error)} Загружено ${counter.current.done} из ${counter.current.total} — повторите, уже добавленные не задвоятся.`);
    },
  });

  const done = progress !== null && progress.total > 0 && progress.done === progress.total && !run.isPending;
  const setField = (field: ImportField, value: string) => setMapping((prev) => (prev ? { ...prev, [field]: Number(value) } : prev));
  const toggleRow = (key: string) =>
    setExcluded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const filePicker = (
    <label className={cx('btn', table ? 'btn--neutral' : 'btn--primary', 'btn--l', table ? null : 'btn--stretched', reading && 'is-loading')}>
      {reading ? <span className="btn__spinner" aria-hidden="true" /> : <Icon name="upload" />}
      <span className="btn__label">{table ? 'Другой файл' : 'Выбрать файл'}</span>
      <input
        type="file"
        className="visually-hidden"
        accept=".xlsx,.csv,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) void pick(file);
          event.target.value = '';
        }}
      />
    </label>
  );

  return (
    <AppShell
      title="Импорт из Excel"
      subtitle={organizationName}
      actionsNote={
        failure ? (
          <p className="actionbar__note" role="alert">
            <Icon name="alert" size={18} />
            {failure}
          </p>
        ) : null
      }
      actions={
        done ? (
          <Button size="l" stretched icon="docs" onClick={() => navigate(`/o/${orgId}/documents`, { replace: true })}>
            К документам
          </Button>
        ) : table ? (
          <Button size="l" stretched loading={run.isPending} disabled={pending.length === 0} onClick={() => run.mutate()}>
            {pending.length > 0 ? `Импортировать ${pending.length} ${plural(pending.length, DOCS)}` : 'Нет строк для импорта'}
          </Button>
        ) : undefined
      }
    >
      {!table ? (
        <>
          <section className="dropzone" aria-labelledby="import-title">
            <span className="dropzone__glyph" aria-hidden="true">
              <Icon name="table" size={30} />
            </span>
            <h2 className="dropzone__title" id="import-title">
              Загрузите таблицу со сроками
            </h2>
            <p className="dropzone__text">Файл .xlsx или .csv: каждая строка — документ. Перед загрузкой покажем, что получилось.</p>
            {filePicker}
            {readError ? (
              <p className="field__error" role="alert">
                <Icon name="alert" size={16} />
                {readError}
              </p>
            ) : null}
          </section>
          <section className="group" aria-labelledby="import-format-title">
            <h2 className="group__title" id="import-format-title">
              Какие столбцы подойдут
            </h2>
            <div className="sample" tabIndex={0} aria-label="Пример таблицы">
              <table>
                <thead>
                  <tr>
                    <th>Название</th>
                    <th>Номер</th>
                    <th>Кем выдан</th>
                    <th>Действует с</th>
                    <th>Действует до</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>Лицензия на алкоголь</td>
                    <td>78РПА0012345</td>
                    <td>Комитет по промышленной политике</td>
                    <td>14.03.2024</td>
                    <td>13.03.2029</td>
                  </tr>
                  <tr>
                    <td>Устав организации</td>
                    <td></td>
                    <td></td>
                    <td></td>
                    <td>бессрочно</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p className="group__foot">
              Обязателен только столбец с названием. Названия столбцов распознаются автоматически, их можно поправить. Даты — как 31.12.2026 или ячейки-даты Excel.
            </p>
          </section>
        </>
      ) : (
        <>
          <section className="group" aria-labelledby="import-file-title">
            <div className="group__card group__card--pad">
              <div className="import-file">
                <span className="import-file__icon" aria-hidden="true">
                  <Icon name="table" />
                </span>
                <div className="import-file__text">
                  <h2 className="import-file__name" id="import-file-title">
                    {fileName}
                  </h2>
                  <p className="import-file__meta">
                    Строк: {rows.length}. Готово к загрузке: {rows.length - invalid}. С ошибками: {invalid}.
                  </p>
                </div>
              </div>
              {progress ? (
                <div className="import-progress" aria-live="polite">
                  <div className="progress">
                    <span className="progress__fill" style={{ transform: `scaleX(${progress.total ? progress.done / progress.total : 0})` }} />
                  </div>
                  <span className="field__hint">
                    {done ? `Готово: добавлено ${progress.done} ${plural(progress.done, DOCS)}` : `Загружено ${progress.done} из ${progress.total}`}
                  </span>
                </div>
              ) : null}
              {!done ? filePicker : null}
            </div>
          </section>

          {!done ? (
            <section className="group" aria-labelledby="import-map-title">
              <h2 className="group__title" id="import-map-title">
                Столбцы
              </h2>
              <div className="form-card">
                <ToggleRow flush title="Первая строка — заголовки" checked={hasHeader} onChange={setHasHeader} />
                <div className="mapping">
                  {IMPORT_FIELDS.map((field) => (
                    <SelectField
                      key={field.key}
                      label={field.label}
                      value={String(mapping?.[field.key] ?? -1)}
                      options={columnOptions}
                      onChange={(value) => setField(field.key, value)}
                    />
                  ))}
                </div>
              </div>
            </section>
          ) : null}

          {rows.length > 0 && !done ? (
            <section className="group" aria-labelledby="import-rows-title">
              <h2 className="group__title" id="import-rows-title">
                Строки файла
              </h2>
              <div className="group__card">
                {rows.slice(0, PREVIEW).map((row) => {
                  const ok = row.body !== null;
                  const on = ok && !excluded.has(row.key);
                  return (
                    <label key={row.key} className="import-row" data-invalid={ok ? 'false' : 'true'} data-done={imported.has(row.key) ? 'true' : 'false'}>
                      <input
                        type="checkbox"
                        className="import-row__input"
                        checked={on}
                        disabled={!ok || imported.has(row.key)}
                        onChange={() => toggleRow(row.key)}
                      />
                      <span className="import-row__box" aria-hidden="true">
                        <Icon name={ok ? 'check' : 'x'} size={16} />
                      </span>
                      <span className="import-row__body">
                        <span className="import-row__title">{row.title || 'Без названия'}</span>
                        <span className="import-row__meta">
                          Строка {row.line}
                          {row.validUntil ? `, до ${formatDate(row.validUntil)}` : row.indefinite ? ', бессрочный' : ', без даты окончания'}
                        </span>
                        {row.errors.map((error) => (
                          <span key={error} className="import-row__error">
                            {error}
                          </span>
                        ))}
                      </span>
                    </label>
                  );
                })}
              </div>
              {rows.length > PREVIEW ? <p className="group__foot">Показаны первые {PREVIEW} строк из {rows.length}.</p> : null}
            </section>
          ) : null}

          {done ? (
            <Banner tone="success" icon="check" title={`Готово: ${progress?.done ?? 0} ${plural(progress?.done ?? 0, DOCS)} в реестре.`}>
              Напоминания по новым документам запланируются автоматически.
            </Banner>
          ) : null}
        </>
      )}
    </AppShell>
  );
}
