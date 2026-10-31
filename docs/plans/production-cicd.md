# Первая публикация: Northflank, Aiven и GitHub Actions

Дата: 2026-10-29. Статус: CI подготовлен локально; запуск GitHub ещё не проверен.

## Актуальное уточнение: Render и ручные миграции

Пользователь выбрал подготовку CI под Render + Aiven. Разделы Northflank ниже
сохранены как предыдущий вариант и **не являются текущим порядком deployment**.
Текущая инструкция — [CI и публикация на Render](../guides/deployment.md),
краткий запуск остаётся в README.
Никаких автоматических production migrations или deploy hooks сейчас нет.

- [x] Создать локальную ветку dev (без push и коммита).
- [x] Подготовить workflow только push main: container, locked toolchain,
      test/lint/build, временная PostgreSQL, browser/integration, secret scan.
- [x] Подготовить упаковку двух проверенных Go executable в non-root images,
      публикацию SHA/digest в GHCR и краткоживущий сборочный artifact.
- [x] Добавить отдельную сборку frontend для Render Static Site и инструкции
      по секретам, manual deployment и миграциям goose с раздельной историей.
- [x] Локально проверить test/race, lint, build и test:browser (включая миграцию
      заполненной БД и повторное применение), скан истории Git и Markdown.
- [ ] Подтвердить первый CI и GHCR push после пользовательского merge в main.
- [ ] Подтвердить Docker packaging на runner: Docker socket в agent отсутствует,
      Compose и инфраструктура из agent не запускаются.
- [ ] Подготовить production runtime для Aiven: TLS verify-full, роли, pool,
      проверки готовности; текущий образ пока использует local dev/test wiring.
- [ ] Подготовить Aiven bootstrap и ручной запуск goose без production credentials
      в GitHub/dev/agent; ограничения ролей Aiven пока не проверены.
- [ ] Доставить WebP из игнорируемого каталога отдельным согласованным способом.
- [ ] Проверить Render build, API rewrites, cookies/Origin, cold start и расход
      общего бюджета free hours. Протестировать первый релиз на синтетике.

## Предыдущий вариант: Northflank

Пользователь выбрал Northflank Sandbox в доступном West Europe и Aiven PostgreSQL.
GitHub уже подключён в Northflank. Подключение не означает готовность CI/CD.
Production credentials агенту не предоставляются; внешняя БД из dev/CI не используется.

## Результат и границы

Проверенный commit проходит CI, из него собираются образы с тегом commit SHA и
фиксируется digest. Northflank запускает выбранные образы; секреты существуют
только в runtime/job окружении платформы. Первая публикация — тестовый стенд;
реальные заявки допускаются после проверки восстановления и условий тарифов.
Frontend остаётся статикой; каталог и заявки сохраняют отдельные процессы,
образы, runtime-роли и владение данными. Миграции не запускаются API при старте.
Google Cloud и Kubernetes в этот этап не входят.

## Шаг 0. Уточнить размещение до создания сервисов

Northflank Sandbox: два сервиса, два jobs, один addon, always-on без сна.
Провайдер описывает Sandbox как среду проверки, а не production applications.
Aiven внешний и не расходует addon Northflank. Тариф/регион/ресурсы сверяются
в аккаунте перед созданием. Публичная страница не гарантирует квоту аккаунта.

Для трёх независимых компонентов нужно выбрать один вариант:

1. Три сервиса Northflank (web proxy/static, catalog, requests), с переходом
   на платный тариф. Самое прямое соответствие текущей архитектуре.
2. Два API Northflank бесплатно, frontend на отдельном статическом хостинге
   с HTTPS reverse proxy для /api. Конкретный хостинг и ограничения proxy
   выбираются отдельно; прямые cross-site запросы усложнят текущую авторизацию.
3. Два сервиса Northflank бесплатно: web/static proxy вместе с catalog в одном
   deployment, requests отдельно. Это предложение временного компромисса,
   требующее обсуждения: совместные релизы и отказ web/catalog, управление
   двумя процессами. Не объединять бизнес-API и не считать их независимыми
   deployment после такого размещения.

Предпочтение для полной изоляции — вариант 1; при обязательной нулевой цене —
обсудить вариант 2. Не менять границы ради лимита тарифа молча.
Вариант 3 не реализовывать без согласования. До выбора можно выполнить шаг 1.

Во всех вариантах предпочтителен один origin браузера: /api/products и
/api/admin/products идут в catalog, /api/requests и остальные /api/admin —
в requests (специфичные маршруты раньше общих). Внешний web принимает HTTPS;
API по возможности private, доступны web proxy по внутренней сети.
Если web вне платформы, отдельно определить аутентификацию/доступ к API.
Origin/CORS не являются запретом запросов от сторонних клиентов.
Сохранить Secure/HttpOnly/SameSite cookies и проверку Origin; проверить Host и
forwarded headers за TLS proxy. SPA fallback нужен для /products/* и
/administrator; отсутствие /api или /media не должно возвращать index.html.

- [ ] Выбрать топологию и подтвердить ресурсы Sandbox в аккаунте.
- [ ] Проверить тариф Aiven: регион, лимит соединений/диска, backup/restore,
      наличие ограниченных пользователей, IP allowlist и отсутствие сна.

## Шаг 1. CI без production-секретов

- [ ] Workflow на pull_request и push default branch; только read contents,
      отмена устаревших CI запусков, таймауты. Не использовать pull_request_target
      для запуска кода PR с секретами.
- [ ] Linux container с toolchain из mise.toml/mise.lock; mise install --locked,
      frontend:install с frozen lockfile. Версию самого mise фиксировать отдельно.
      Версии Go/Bun/Node/CLI не копировать в workflow или Dockerfile.
- [ ] mise run test, lint, build и hooks/secret scanning согласно существующим
      задачам. Проверить, не требует ли hooks недоступных локальных данных.
- [ ] Временная PostgreSQL service с сетевым именем postgres:5432; синтетические
      данные и отдельная БД каждого запуска. Использовать существующие
      test:integration и test:browser, включая проверку миграций заполненной БД.
      Пароль CI генерируется на запуск, хранится файлом, не выводится;
      адаптер CI не снимает защиту local-db-env.sh от внешних адресов.
- [ ] Chromium и системные зависимости только внутри Linux CI container;
      безопасные отчёты ошибок, без credentials и реальных данных.
- [ ] Проверить первый запуск на GitHub. Пользователь при необходимости
      включает обязательный CI check для merge сам; агент настройки repo не меняет.

Готовность: намеренно падающая проверка блокирует дальнейшую публикацию,
обычный PR проходит test/lint/build/integration/browser, Aiven недоступен этому CI.
Локальный успех не отмечать как подтверждённый запуск GitHub Actions.

## Шаг 2. Production runtime и образы

- [ ] Отделить dev/test конфигурацию от production: сейчас runtime фиксирует
      postgres:5432, sslmode=disable, имена dev/test БД и локальные LOGIN.
      Добавить явные host/port/database/user, password file и CA file;
      production только TLS verify-full с проверкой имени сервера.
- [ ] Не логировать URI, пароли и текст ошибок с credentials. Ограничить pool
      каждого API с учётом лимита Aiven, миграций, jobs и перекрытия ревизий.
- [ ] Слушать 0.0.0.0 на настроенном порту; startup/readiness и liveness
      разделить: недоступность БД не должна вызывать бесконечные рестарты API.
      Существующий /healthz проверяет БД — учесть это в настройке checks.
- [ ] Multi-stage images catalog, requests, web и migration/admin job;
      non-root runtime, CA trust, graceful shutdown. Docker build не получает
      секреты и .local. Ужесточить build contexts/.dockerignore.
- [ ] Решить доставку фотографий: frontend/public/media исключён из Git,
      поэтому чистый checkout соберётся без реальных WebP. Выбрать отдельный
      versioned media artifact или публичное объектное хранилище; не загружать
      исходные XLSX/фото и не считать локальную сборку доказательством доставки.
      Проверить соответствие URL в Aiven и полного/thumbnail набора.
- [ ] Проверить образы на локальной синтетической PostgreSQL: HTTP, права,
      cookies через proxy, перезапуск, SPA, фото и лимиты памяти.

Готовность: сборка из чистого checkout воспроизводима, runtime без dev toolchain,
ошибка CA/hostname отвергается, обе версии фото доступны, секреты отсутствуют
в слоях и metadata. Aiven пока не подключается из среды агента.

## Шаг 3. Публикация артефактов

- [ ] После успешных проверок доверенного push default branch собрать/проверить
      и опубликовать GHCR images. PR не публикует и не получает write token.
      Для push образов — GITHUB_TOKEN с packages:write только в publish job.
- [ ] Тег commit SHA, digest каждого образа и общий release manifest.
      Не развёртывать плавающий latest. Проверки относятся к тому же commit.
- [ ] Northflank: deployment из external image, отдельные read-only registry
      credentials для private GHCR. Подключённый GitHub не гарантирует доступ
      к private packages. Build из Git и auto-deploy по push пока отключены,
      чтобы не обойти GitHub CI вторым независимым путём.

Готовность: есть проверенные digest и возможность скачать их в Northflank.

## Шаг 4. Aiven и первый релиз вручную

- [ ] Пользователь создаёт в Aiven отдельные runtime logins каталога/заявок и
      роль мигратора; проверить совместимость bootstrap с ограничениями Aiven.
      Не использовать avnadmin в API. Не переносить dev/test secrets.
- [ ] Пароли — Northflank runtime secrets/secret files; CA — runtime file.
      Secrets мигратора только job, API получают только собственные credentials.
      Не задавать DB secrets как build args или frontend variables.
- [ ] Внешнюю сеть защитить доступным IP allowlist: проверить стабильность
      исходящих IP Northflank и цену этой функции. Не обещать фиксированный
      egress на Sandbox; при его отсутствии явно выбрать подход к доступу.
- [ ] Миграции отдельным manual job, по одной версии релиза, сериализованно;
      сначала backup и проверка восстановления. Существующие локальные scripts
      не направлять в Aiven — отдельная процедура без снятия dev guards.
- [ ] Создать первый admin через отдельный job без пароля в arguments/logs.
      Начать с синтетических товаров; реальный каталог переносить отдельной
      проверяемой операцией с согласованными фото, без dev заявок и сессий.
- [ ] Создать Northflank deployment services по выбранной топологии, настроить
      порты, checks, origins и ресурсы. Сначала API после миграций, затем web.
      Взять сгенерированный HTTPS адрес платформы для первого теста.
- [ ] Проверить каталог/фото/SPA, заказ и повтор, админку, статус и остатки,
      неавторизованный доступ, logs без секретов, rolling update и рестарт.

Готовность: первый стенд проверен по HTTPS, backup восстановлен отдельно,
условия free tiers и эксплуатационные ограничения явно приняты.

## Шаг 5. Управляемый CD

- [ ] workflow_dispatch выбирает ранее проверенный release/digest;
      ограниченный Northflank API token хранится в GitHub environment secret.
      DB credentials в GitHub для деплоя не требуются: миграции выполняет job.
- [ ] Один релиз одновременно; backup → migration job → API rollout → web
      rollout → smoke checks. Ошибка на любом шаге блокирует следующие.
      Новый релиз не отменяет уже запущенную миграцию.
- [ ] Записать предыдущие digest и план возврата images. Миграции обратно
      автоматически не откатывать; изменения схемы совместимы со старой
      ревизией во время rollout (expand/contract при необходимости).
- [ ] Наблюдение: внешняя HTTPS проверка, ошибки/задержки API, память и соединения
      БД, место/лимиты Aiven, сроки backup и процедура восстановления.

Готовность: повторный релиз и возврат images подтверждены, при провале
миграции приложение не обновляется. Полный auto-deploy после push — позже,
после двух успешных управляемых релизов.

## Проверка плана и следующий шаг

2026-10-29: прочитаны runtimeapp, dev DB scripts, задачи mise, Vite proxy,
ADR о границах и README о фото. CI и deployment ещё не создавались.
Локальный файл .local/v1.json ранее проверен: только сообщение service is online,
конфигурации deployment в нём нет. Следующий исполняемый шаг — CI (шаг 1),
параллельно пользователь выбирает размещение из шага 0.

Источники (проверены 2026-10-29):

- [Northflank pricing](https://northflank.com/pricing)
- [Sandbox и назначение free tier](https://northflank.com/docs/v1/application/billing/pricing-on-northflank)
- [Образы registry](https://northflank.com/docs/v1/application/build/import-container-images)
- [Northflank API](https://northflank.com/docs/v1/api/use-the-api)
- [Подключение Aiven с TLS](https://aiven.io/docs/products/postgresql/howto/list-code-samples)
