# CI и публикация на Render

Обновление 2026-11-05: frontend остаётся на Render; API размещаются в Cloud Run.
Актуальный порядок backend deployment и секретов — [запуск GCP](gcp.md).
Инструкции ниже о Render Web Service для API описывают предыдущий вариант.

[Краткий запуск и архитектура](../../README.md)

## CI в GitHub и публикация на Render

### Секреты: сначала разделить доступ

Для текущего CI **не добавлять секреты вручную в GitHub**. Workflow получает
автоматический короткоживущий `GITHUB_TOKEN`; право `packages:write` есть только
у job публикации образов. Проверки используют одноразовую синтетическую
PostgreSQL в изолированной сети GitHub, без открытого порта на runner.
Aiven URI, пароль БД, пароль администратора магазина и CA не передаются CI.

В Render создаются отдельные сервисы каталога и заявок. Каждый получает только
собственный LOGIN/пароль с ограниченными правами. Не использовать `avnadmin`
в приложении и не создавать общую environment group с паролями обоих API.
Пароль добавляется через **Environment → Secret Files** (например,
`catalog_password` или `requests_password`), CA — отдельным `aiven-ca.pem`.
Runtime-пути Render: `/etc/secrets/catalog_password`,
`/etc/secrets/requests_password`, `/etc/secrets/aiven-ca.pem`.
CA — публичный сертификат, но удобнее доставлять его тем же файловым механизмом.
Значения вводит пользователь в UI Render; не отправлять их в чат или Git.
Проверка TLS должна быть `verify-full`, с DNS-именем из Aiven и этим CA.

**Важно:** текущий runtime ещё принимает только локальные dev/test параметры.
Эти пути — соглашение для следующего шага настройки production, а не обещание,
что публикация текущего образа уже подключится к Aiven. Нужна отдельная правка
конфигурации host/port/user/database/TLS/pool. Локальные scripts имеют защиту от
внешних БД; не отключать её для миграций Aiven.

Для private GHCR image в Render добавить registry credential: отдельный GitHub
PAT classic с минимальным `read:packages`, сроком действия и доступом только
к нужным packages/repository там, где это позволяет модель GitHub. PAT наследует
права владельца: `read:packages` не ограничивает его одним package автоматически.
Публичный image скачивается без токена; прежде чем делать его публичным,
проверить содержимое. Не выдавать Render `write:packages` или персональный токен
с широкими правами, если это не требуется политиками организации.

Frontend Static Site не получает ни одного секрета БД. Всё, что попадает в
`VITE_*`, HTML/JS, считается публичным. Не печатать окружение/URI в logs,
не включать `set -x`. Secret storage не предотвращает утечку из кода, который
сам печатает секрет или отправляет его по сети. При утечке отозвать/сменить
credential; удаление строки из Git или log не заменяет ротацию.

### Что делает workflow

Файл `.github/workflows/ci.yml` запускается на **push в main**, включая merge,
и вручную через Actions → CI → Run workflow. Push в dev и PR сами по себе
его не запускают. Публикация образов разрешена только из main.

Job `repository` сравнивает checkout с последним успешным запуском этого workflow
в main через GitHub API (`actions:read`). Поэтому изменения из отменённого или
упавшего push также попадут в проверки. При отсутствии успешной базы, ошибке API,
переписанной истории и ручном запуске выполняется полный набор. Фильтр учитывает
все commits, удаления и оба пути rename; неизвестные файлы включают полный набор.
Набор выбранных проверок виден в summary job `repository`.

| Изменения | Проверки и сборки |
| --- | --- |
| Любой push | История Git через gitleaks с redaction, shell syntax, тесты CI-фильтра |
| Markdown, `.rumdl.toml` | Markdown lint |
| `frontend/`, скрипты сборки Render и обработки media | Frontend lint/typecheck/build, PostgreSQL integration и browser; без Go-образов |
| `cmd/`, `internal/`, Go dependencies, миграции, bootstrap Aiven, прочие `scripts/` | Go test/race/lint, миграции и bootstrap; frontend lint/build, integration/browser; оба Go-образа |
| `deploy/terraform/` | Только Terraform fmt/init/validate и mock plans, без cloud credentials |
| Dockerfile, Compose, `.devcontainer/`, `.dockerignore`, mise config/lock, `.github/`, `.env.example`, `.gitignore` | Полный набор и оба Go-образа |
| `docs/contracts/` | Проверки обоих компонентов, Markdown и оба образа |
| Неизвестные пути | Полный набор и оба Go-образа |

Job `checks` с PostgreSQL запускается только для изменений приложения.
Браузерные тесты frontend используют настоящие Go API и временную синтетическую
БД: Go в этих тестах нужен, даже когда production-образы не собираются.
Миграции применяются только к новой тестовой БД и проверяются повторно.
Job `infrastructure` запускается отдельно по своему фильтру.
Каждый job устанавливает нужный набор locked tools из общего профиля mise;
профиль agent в CI не используется. Повторная обычная Go-сборка перед portable
Linux-сборкой удалена: CI компилирует release executable один раз.

Job `checks` восстанавливает три кеша через закреплённый `actions/cache`:

- Go modules (`go env GOMODCACHE`): ОС, архитектура, версия Go и хеш `go.mod`/`go.sum`.
- Go compilation (`go env GOCACHE`): те же поля плюс хеш Go-файлов в `cmd/` и
  `internal/`, чтобы изменения кода сохраняли новый build cache.
- Bun packages (`bun pm --cwd frontend cache`): ОС, архитектура, версия Bun и
  хеш `frontend/bun.lock`.

Пути определяются внутри job container после установки locked toolchain;
версии берутся у установленных mise инструментов. `restore-keys` позволяют
использовать предыдущие загрузки при смене зависимостей; для Go build сначала
ищется кеш с теми же зависимостями. Версии инструментов и ОС/архитектура при
fallback сохраняются. Отсутствующий кеш не мешает обычной установке и проверкам.
`bun install --frozen-lockfile` выполняется и при cache hit; `node_modules` и
release artifacts в dependency cache не входят. Новые кеши сохраняются после
успеха job; существующая запись с точным ключом не перезаписывается.
Правила ключей и fallback — [GitHub dependency caching](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching),
путь Bun — [Bun global cache](https://bun.sh/docs/pm/global-cache).

Артефакты хранятся семь дней: `linux-amd64-<commit SHA>` содержит два Go executable
и создаётся только при backend-сборке, `frontend-<commit SHA>` — `frontend/dist`
при frontend-сборке. Render собирает frontend самостоятельно из Git.
Только после успеха необходимых проверок job `images` упаковывает executable
в минимальные non-root images и публикует `ghcr.io/<owner>/<repo>/catalog:<SHA>`
и `ghcr.io/<owner>/<repo>/requests:<SHA>`. Digest каждого — в job summary.
Образы не содержат исходников, toolchain, `.local` или секретов.

После frontend-only изменения новых API images с SHA этого commit не будет:
сохраняются предыдущие проверенные API digest, обновляется только frontend.
Аналогично Terraform-only правка не требует новых образов. Для принудительной
пересборки (например, при обновлении runtime base tag) выбрать Run workflow
в main: он выполняет полный набор и публикует оба образа.

Нет `latest`, вызовов deploy hooks, доступа к Aiven или production migrations.
SHA GitHub Actions закреплены в workflow. Runtime base image обновляется
отдельно; сейчас tag базы не делает сборку побайтно воспроизводимой.
Первые запуски проверяются в GitHub: локальные тесты не подтверждают работу runner,
скачивание toolchain, права GHCR или доступ Render.

### После push пользователя

1. Изменения подготовлены в локальной ветке `dev`; коммит, push и merge выполняет
   пользователь. После merge в `main` открыть Actions → CI. Если публикация
   запрещена политиками аккаунта/организации, разрешить packages для workflow,
   не заменять `GITHUB_TOKEN` широким персональным токеном.
2. Проверить выбранные jobs `repository`, `checks`, `infrastructure` и `images`.
   Пропущенные по фильтру jobs ожидаемы; при backend-сборке проверить наличие
   обоих packages и digest в summary.
   До правки production runtime образы являются проверенными сборками, но ещё
   не готовыми к подключению Aiven.
3. В Render создать два **Web Service → Existing Image**, архитектура
   `linux/amd64`. Выбрать digest из одного успешного CI запуска, настроить
   runtime только после подготовки подключения Aiven. Не публиковать порт БД.
   Автоматический deployment пока не включать.
4. Для frontend создать **Static Site** из того же репозитория, ветка `main`,
   Root Directory оставить пустой. Build Command:
   `bash scripts/render-static-build.sh`. Publish Directory: `frontend/dist`.
   Auto-Deploy: **Off**, чтобы сначала вручную обновлять БД и API.
   Render сам повторно собирает frontend из выбранного commit; это не загрузка ZIP.
   Выбирать Manual Deploy → Deploy a specific commit с тем же SHA, что у API.
5. Настроить Rewrite `/api/admin/products` и `/api/admin/products/*` в catalog,
   `/api/products` и `/api/products/*` в catalog; `/api/admin` и `/api/admin/*`,
   `/api/requests` и `/api/requests/*` в requests. Destination содержит полный
   HTTPS URL API и исходный путь, с `*` для wildcard. Правила API идут перед SPA.
   Для SPA использовать явные маршруты `/products/*` и `/administrator` →
   `/index.html`, чтобы отсутствующий API или media не превращался в HTML.
   Проверить proxy методов POST/PATCH, cookies и Origin на настоящем HTTPS;
   до этой проверки не считать админку готовой. `APP_ORIGINS` должен содержать
   точный origin frontend, `ADMIN_LOCAL_HTTP` в production не задаётся.
6. Решить доставку фотографий **до** публикации каталога: `frontend/public/media`
   исключён из Git. CI и Render из чистого checkout не имеют пользовательских
   WebP. Текущая инструкция не загружает их и не публикует исходные материалы;
   нужен отдельный согласованный media artifact/storage.
7. На стенде проверить HTTPS, фото, reload карточки, создание/повтор заявки,
   вход/выход админки, смену статуса и права. Free API засыпает после простоя;
   обрабатывать ошибки/timeout, не создавать повторы с новым ключом.

Подключение репозитория само по себе не переносит секреты или GitHub artifacts.
Free Render имеет общий бюджет 750 работающих instance-hours на workspace,
холодный старт может занимать около минуты. Эти ограничения проверяются отдельно
от производительности Go.

### Миграции: вручную, отдельно от релиза

Рекомендуемый инструмент — **goose**, уже выбранный проектом. DBeaver можно
использовать для осмотра/проверки, но простое выполнение SQL из файлов не ведёт
таблицы истории goose и может исполнить также секцию Down. Не исполнять файл
миграции целиком в SQL editor и не редактировать историю вручную.

Пользователь запускает goose из доверенной среды с отдельной ролью мигратора.
Production credentials остаются в его окружении, вне dev/agent и GitHub Actions.
Параметры подключения передаются защищённым config/файлами credentials и
окружением, не URI с паролем в shell history или аргументах.
Версия goose берётся из `mise.lock`. Этот этап пока **не** предоставляет готовый
production migration script: сначала проверить bootstrap/GRANT и права Aiven.
Миграции используют роли `<database>_catalog`, `<database>_requests` и
`<database>_migrator`; runtime LOGIN — отдельные роли с нужным membership.

Порядок ручного релиза:

1. Выбрать успешный SHA/digest; просмотреть новые SQL и определить целевые версии
   обоих наборов миграций. Сделать backup и иметь проверенное восстановление.
2. Проверить `status` в `migrations/catalog`, затем `migrations/requests`.
   Использовать раздельные таблицы `catalog.goose_db_version` и
   `requests.goose_db_version` и сначала корректный bootstrap схем/ролей.
3. Применить до выбранных версий через `up-to` (каталог, затем заявки),
   убедиться в успехе и повторно проверить status. Один оператор одновременно.
4. Вручную развернуть API по digest выбранного SHA, затем frontend того же SHA.
   Если миграция не прошла, остановить релиз и разобраться, не обновлять API.
5. Проверить ключевые сценарии и сохранить запись о версиях схем и образов.

Обновления схем должны поддерживать работающую старую ревизию API: сначала
добавление совместимых полей/таблиц, затем новый код, удаление старого — отдельным
релизом. Для несовместимой миграции согласовать окно обслуживания.
Возврат старого image не откатывает БД. `down` автоматически не выполнять;
выбор между исправляющей миграцией и восстановлением требует отдельной оценки.

Справка: [Render images](https://render.com/docs/deploying-an-image),
[Secret files](https://render.com/docs/configure-environment-variables),
[Rewrites](https://render.com/docs/redirects-rewrites),
[Manual deploy](https://render.com/docs/deploys),
[Free limitations](https://render.com/docs/free).
