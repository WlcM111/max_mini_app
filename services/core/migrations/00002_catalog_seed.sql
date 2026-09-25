-- Модельные данные справочника (data_status = 'model'): НЕ сверены с нормативными актами.
-- Сроки действия не утверждаются — пользователь вводит даты из своих документов.

-- +goose Up
INSERT INTO core.business_categories (code, title, sort_order) VALUES
    ('food_service',    'Общественное питание', 10),
    ('food_retail',     'Розничная торговля продуктами', 20),
    ('nonfood_retail',  'Розничная торговля непродовольственными товарами', 30),
    ('beauty_services', 'Бытовые и косметические услуги', 40),
    ('food_production', 'Производство пищевой продукции', 50),
    ('agriculture',     'Сельское хозяйство (КФХ, ИП)', 60),
    ('other',           'Другой вид деятельности', 99);

INSERT INTO core.regions (code, title, default_timezone, sort_order) VALUES
    ('RU-SPE', 'Санкт-Петербург', 'Europe/Moscow', 10),
    ('RU-MOW', 'Москва', 'Europe/Moscow', 20),
    ('RU-LEN', 'Ленинградская область', 'Europe/Moscow', 30),
    ('RU-MOS', 'Московская область', 'Europe/Moscow', 40),
    ('RU-TA',  'Республика Татарстан', 'Europe/Moscow', 50),
    ('RU-KDA', 'Краснодарский край', 'Europe/Moscow', 60),
    ('RU-NIZ', 'Нижегородская область', 'Europe/Moscow', 70),
    ('RU-KGD', 'Калининградская область', 'Europe/Kaliningrad', 80),
    ('RU-SAM', 'Самарская область', 'Europe/Samara', 90),
    ('RU-SVE', 'Свердловская область', 'Asia/Yekaterinburg', 100),
    ('RU-BA',  'Республика Башкортостан', 'Asia/Yekaterinburg', 110),
    ('RU-NVS', 'Новосибирская область', 'Asia/Novosibirsk', 120),
    ('RU-KYA', 'Красноярский край', 'Asia/Krasnoyarsk', 130),
    ('RU-IRK', 'Иркутская область', 'Asia/Irkutsk', 140),
    ('RU-PRI', 'Приморский край', 'Asia/Vladivostok', 150),
    ('XX-OTHER', 'Другой регион (часовой пояс выбирается вручную)', 'Europe/Moscow', 999);

INSERT INTO core.features (code, question, hint, sort_order) VALUES
    ('has_premises',     'Есть помещение: зал, кухня, цех или торговая точка', NULL, 10),
    ('rents_premises',   'Помещение арендовано', 'Отметьте, если есть договор аренды', 20),
    ('has_employees',    'Есть наёмные сотрудники', NULL, 30),
    ('sells_alcohol',    'Продаёте алкогольную продукцию', NULL, 40),
    ('uses_kkt',         'Используете контрольно-кассовую технику', NULL, 50),
    ('uses_scales',      'Используете весы или другие измерительные приборы в расчётах', NULL, 60),
    ('produces_food',    'Производите пищевую продукцию на продажу', NULL, 70),
    ('seasonal_outdoor', 'Работаете с летней верандой или нестационарным объектом', NULL, 80),
    ('has_machinery',    'Есть самоходная техника (трактор, погрузчик)', NULL, 90);

INSERT INTO core.catalog_sources (id, title, url, checked_on) VALUES
    (1, 'Портал государственных услуг Российской Федерации', 'https://www.gosuslugi.ru', NULL),
    (2, 'Федеральная налоговая служба', 'https://www.nalog.gov.ru', NULL),
    (3, 'Федеральная служба по аккредитации', 'https://fsa.gov.ru', NULL),
    (4, 'Росалкогольтабакконтроль', 'https://fsrar.gov.ru', NULL);

INSERT INTO core.document_types (code, title, description, data_status, source_id, sort_order) VALUES
    ('qualified_esignature', 'Сертификат квалифицированной электронной подписи (КЭП)', 'Срок указан в сертификате. Без действующей КЭП недоступны отчётность, ЭДО и госсервисы.', 'model', 2, 10),
    ('kkt_fiscal_drive', 'Ключ фискального накопителя ККТ', 'Дату окончания срока ключа показывают отчёты кассы и личный кабинет ОФД.', 'model', 2, 20),
    ('alcohol_retail_license', 'Лицензия на розничную продажу алкогольной продукции', 'Срок действия указан в лицензии и в реестре лицензий.', 'model', 4, 30),
    ('premises_lease', 'Договор аренды помещения', 'Срок и порядок продления указаны в договоре.', 'model', NULL, 40),
    ('waste_removal_contract', 'Договор на вывоз твёрдых коммунальных отходов', 'Договор с региональным оператором по обращению с ТКО.', 'model', NULL, 50),
    ('pest_control_contract', 'Договор на дезинсекцию и дератизацию', 'Договор с исполнителем и график обработок.', 'model', NULL, 60),
    ('employee_medical_exam', 'Периодический медосмотр сотрудника (медицинская книжка)', 'Одна запись на сотрудника. В поле «Ответственный» указывайте должность, а не ФИО.', 'model', NULL, 70),
    ('production_control_program', 'Программа производственного контроля (пересмотр)', 'Дата утверждения и плановый пересмотр программы.', 'model', NULL, 80),
    ('scales_verification', 'Поверка весов и других средств измерений', 'Дата следующей поверки указана в сведениях о поверке.', 'model', NULL, 90),
    ('fire_extinguishers_service', 'Техническое обслуживание огнетушителей', 'Дата следующего обслуживания указана на ярлыке или в паспорте.', 'model', NULL, 100),
    ('labor_safety_training', 'Обучение по охране труда', 'Даты обучения указаны в протоколах.', 'model', NULL, 110),
    ('special_labor_assessment', 'Специальная оценка условий труда (СОУТ)', 'Дата указана в отчёте о проведении СОУТ.', 'model', NULL, 120),
    ('conformity_declaration', 'Декларация о соответствии продукции', 'Срок указан в декларации и в реестре деклараций.', 'model', 3, 130),
    ('outdoor_seasonal_permit', 'Разрешение на сезонное кафе или нестационарный объект', 'Срок указан в разрешении или договоре размещения.', 'model', 1, 140),
    ('property_insurance', 'Договор страхования имущества или ответственности', 'Дата окончания указана в полисе.', 'model', NULL, 150),
    ('machinery_inspection', 'Технический осмотр самоходной техники', 'Дата следующего осмотра указана в документах на технику.', 'model', 1, 160);

INSERT INTO core.document_type_default_offsets (document_type_code, days_before) VALUES
    ('qualified_esignature', 30), ('qualified_esignature', 14), ('qualified_esignature', 3),
    ('kkt_fiscal_drive', 30), ('kkt_fiscal_drive', 14), ('kkt_fiscal_drive', 3),
    ('alcohol_retail_license', 90), ('alcohol_retail_license', 60), ('alcohol_retail_license', 30),
    ('premises_lease', 60), ('premises_lease', 30), ('premises_lease', 7),
    ('waste_removal_contract', 30), ('waste_removal_contract', 7),
    ('pest_control_contract', 30), ('pest_control_contract', 7),
    ('employee_medical_exam', 30), ('employee_medical_exam', 14), ('employee_medical_exam', 3),
    ('production_control_program', 30), ('production_control_program', 7),
    ('scales_verification', 30), ('scales_verification', 7),
    ('fire_extinguishers_service', 30), ('fire_extinguishers_service', 7),
    ('labor_safety_training', 60), ('labor_safety_training', 30),
    ('special_labor_assessment', 90), ('special_labor_assessment', 30),
    ('conformity_declaration', 60), ('conformity_declaration', 30),
    ('outdoor_seasonal_permit', 60), ('outdoor_seasonal_permit', 30),
    ('property_insurance', 30), ('property_insurance', 7),
    ('machinery_inspection', 30), ('machinery_inspection', 7);

INSERT INTO core.document_type_renewal_steps (document_type_code, step_no, step_text) VALUES
    ('qualified_esignature', 1, 'Проверьте дату окончания в сертификате или в программе электронной подписи.'),
    ('qualified_esignature', 2, 'Заранее запишитесь в удостоверяющий центр для выпуска нового сертификата.'),
    ('qualified_esignature', 3, 'Установите новый сертификат в программах отчётности и ЭДО.'),
    ('kkt_fiscal_drive', 1, 'Уточните дату окончания срока ключа в отчёте кассы или у ОФД.'),
    ('kkt_fiscal_drive', 2, 'Закажите новый фискальный накопитель у поставщика ККТ.'),
    ('kkt_fiscal_drive', 3, 'Замените накопитель и перерегистрируйте кассу в личном кабинете ФНС.'),
    ('alcohol_retail_license', 1, 'Проверьте срок в лицензии или в реестре лицензий.'),
    ('alcohol_retail_license', 2, 'Подготовьте заявление и документы по перечню лицензирующего органа вашего региона.'),
    ('alcohol_retail_license', 3, 'Подайте заявление заранее, до окончания срока действия лицензии.'),
    ('premises_lease', 1, 'Проверьте в договоре условия продления и срок уведомления арендодателя.'),
    ('premises_lease', 2, 'Согласуйте продление или новый договор.'),
    ('waste_removal_contract', 1, 'Проверьте срок договора с региональным оператором.'),
    ('waste_removal_contract', 2, 'Запросите новый договор или дополнительное соглашение.'),
    ('pest_control_contract', 1, 'Проверьте срок договора и график обработок.'),
    ('pest_control_contract', 2, 'Согласуйте продление с исполнителем и сохраните акты работ.'),
    ('employee_medical_exam', 1, 'Уточните дату следующего медосмотра по отметке в книжке.'),
    ('employee_medical_exam', 2, 'Заранее запишите сотрудника в медицинскую организацию.'),
    ('employee_medical_exam', 3, 'После прохождения внесите новую дату через «Продлить».'),
    ('production_control_program', 1, 'Проверьте дату утверждения и плановый срок пересмотра.'),
    ('production_control_program', 2, 'Актуализируйте программу при изменении ассортимента или оборудования.'),
    ('scales_verification', 1, 'Проверьте дату следующей поверки в сведениях о поверке.'),
    ('scales_verification', 2, 'Закажите поверку в аккредитованной организации.'),
    ('fire_extinguishers_service', 1, 'Проверьте дату следующего обслуживания на ярлыке огнетушителя.'),
    ('fire_extinguishers_service', 2, 'Закажите обслуживание или перезарядку.'),
    ('labor_safety_training', 1, 'Проверьте даты обучения в протоколах.'),
    ('labor_safety_training', 2, 'Запланируйте обучение до окончания срока.'),
    ('special_labor_assessment', 1, 'Проверьте дату отчёта о проведении СОУТ.'),
    ('special_labor_assessment', 2, 'Заранее заключите договор с аккредитованной организацией.'),
    ('conformity_declaration', 1, 'Проверьте срок в реестре деклараций.'),
    ('conformity_declaration', 2, 'Подготовьте документы и испытания для новой декларации.'),
    ('outdoor_seasonal_permit', 1, 'Уточните срок разрешения или договора размещения.'),
    ('outdoor_seasonal_permit', 2, 'Подайте заявление на новый сезон в порядке, установленном муниципалитетом.'),
    ('property_insurance', 1, 'Проверьте дату окончания полиса.'),
    ('property_insurance', 2, 'Запросите предложения страховщиков и продлите договор.'),
    ('machinery_inspection', 1, 'Проверьте дату следующего осмотра в документах на технику.'),
    ('machinery_inspection', 2, 'Запишитесь на осмотр в орган гостехнадзора.');

INSERT INTO core.applicability_rules (id, document_type_code, business_category_code) VALUES
    (1, 'qualified_esignature', NULL),
    (2, 'kkt_fiscal_drive', NULL),
    (3, 'alcohol_retail_license', NULL),
    (4, 'premises_lease', NULL),
    (5, 'waste_removal_contract', NULL),
    (6, 'pest_control_contract', 'food_service'),
    (7, 'pest_control_contract', 'food_retail'),
    (8, 'pest_control_contract', 'food_production'),
    (9, 'employee_medical_exam', 'food_service'),
    (10, 'employee_medical_exam', 'food_retail'),
    (11, 'employee_medical_exam', 'food_production'),
    (12, 'employee_medical_exam', 'beauty_services'),
    (13, 'production_control_program', 'food_service'),
    (14, 'production_control_program', 'food_production'),
    (15, 'scales_verification', NULL),
    (16, 'fire_extinguishers_service', NULL),
    (17, 'labor_safety_training', NULL),
    (18, 'special_labor_assessment', NULL),
    (19, 'conformity_declaration', 'food_production'),
    (20, 'conformity_declaration', NULL),
    (21, 'outdoor_seasonal_permit', NULL),
    (22, 'property_insurance', NULL),
    (23, 'machinery_inspection', 'agriculture');

INSERT INTO core.applicability_rule_features (rule_id, feature_code) VALUES
    (2, 'uses_kkt'), (3, 'sells_alcohol'), (4, 'rents_premises'), (5, 'has_premises'),
    (6, 'has_premises'), (7, 'has_premises'), (8, 'has_premises'),
    (9, 'has_employees'), (10, 'has_employees'), (11, 'has_employees'), (12, 'has_employees'),
    (15, 'uses_scales'), (16, 'has_premises'), (17, 'has_employees'), (18, 'has_employees'),
    (20, 'produces_food'), (21, 'seasonal_outdoor'), (22, 'has_premises'), (23, 'has_machinery');

-- +goose Down
DELETE FROM core.applicability_rule_features;
DELETE FROM core.applicability_rules;
DELETE FROM core.document_type_renewal_steps;
DELETE FROM core.document_type_default_offsets;
DELETE FROM core.document_types;
DELETE FROM core.catalog_sources;
DELETE FROM core.features;
DELETE FROM core.regions;
DELETE FROM core.business_categories;
