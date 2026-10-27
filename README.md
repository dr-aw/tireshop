# tireshop

Планы: [PLANS.md](PLANS.md). Правила: [AGENTS.md](AGENTS.md).

## Первый код и проверки

Реализованы [правила заявки](internal/requests/quote.go) и
[сохранение в PostgreSQL](internal/requests/postgres/store.go): расчёт целых рублей,
проверка цены, исторический снимок и идемпотентное создание в одной транзакции.
Добавлены два HTTP API и React/TypeScript форма: каталог, корзина, контакты,
повтор отправки и подтверждение новой цены. Правила смены статусов пока только
в памяти; защищённая админка и сохранение истории статусов — следующий шаг.

В Linux dev/agent container, из /workspace:

```bash
mise run test
mise run lint
mise run build
```

Test запускает обычные Go-тесты с race detector без подключения к БД.
Lint — go vet (включая integration-файлы), gofmt, синтаксис shell, SQL-аннотации goose
TypeScript и форматирование frontend (Prettier). Build создаёт build/catalog, build/requests и frontend/dist.
Параметр go в go.mod задаёт требования языка модуля;
установка toolchain по-прежнему управляется mise.toml/mise.lock.

Для навигации по Go-коду откройте `/workspace` в VS Code внутри Dev Container,
установите туда расширение Go (`golang.go`) и инструмент gopls через
`Go: Install/Update Tools`. В `.vscode/settings.json` для gopls включён тег
`integration`, чтобы редактор анализировал и интеграционные тесты.
После изменения настроек выполните `Go: Restart Language Server`.
Интеграционные тесты запускайте через `mise run test:integration`:
эта команда подготавливает отдельную тестовую БД.

## Запуск магазина локально

На основной ОС из корня проекта один раз создайте два runtime-secret и подключите
их к dev (PostgreSQL должен работать):

```bash
bash scripts/create-runtime-secrets.sh
docker compose -f compose.yaml -f compose.agent.yaml up -d postgres
docker compose -f compose.yaml -f compose.agent.yaml up -d --no-deps --force-recreate dev
```

Затем внутри Linux dev container:

```bash
mise install --locked
mise run frontend:install
mise run db:bootstrap
mise run db:migrate
mise run db:runtime
mise run db:seed
mise run dev
```

Для ежедневного запуска достаточно последней команды. Откройте порт 5173 через
VS Code Ports и перейдите на <http://localhost:5173>. API слушают loopback внутри
контейнера: каталог 8081, заявки 8082; Vite проксирует /api к ним. Ctrl+C останавливает
все три процесса. Это локальный dev-сервер, не схема production-развёртывания.
Бинарники можно запускать независимо; они не выполняют миграции автоматически.

Настройки API: PGDATABASE (только локальная dev/test БД), HTTP_ADDR,
CATALOG_PASSWORD_FILE или REQUESTS_PASSWORD_FILE (пути к secret), APP_ORIGINS
для сервиса заявок (разрешённые Origin через запятую; по умолчанию localhost:5173
и 127.0.0.1:5173 с http). Пароли не передавайте аргументами или текстом в environment.
При другом адресе браузера явно задайте APP_ORIGINS. Статическая сборка frontend/dist
ожидает /api на том же origin; proxy production пока не настроен.

Форма хранит только незавершённую отправку в sessionStorage текущей вкладки;
после успешного ответа оставляет номер заявки без контактов. При потере ответа
повторяет прежние ключ и тело. Корзина до отправки пока находится в памяти страницы.
Каталог содержит демонстрационные данные; фотографии и полноценные фильтры впереди.

## Миграции и интеграционные тесты

Goose устанавливается через mise; Compose/Dockerfile не перечисляют проектные CLI.
Agent делает установку при запуске; в открытом dev container выполнить:

```bash
mise install --locked
mise exec -- goose --version
```

Проверка слоя хранения внутри Linux-контейнера при запущенном локальном postgres:

```bash
mise run test:integration
```

Команда создаёт новую БД tireshop_test_<случайный суффикс>, три отдельные NOLOGIN-роли,
пустые схемы и применяет миграции. Затем проверяет SQL-права, сохранение,
конкурентные повторы, конфликты и откат; повторный up сохраняет записанные данные.
Также проверяет HTTP JSON/лимиты, создание, повтор и подтверждение новой цены.
В конце удаляет только созданную этой командой БД и её роли. Dev-БД не сбрасывается.
Тесты с build tag integration нельзя запускать напрямую без подготовленного
окружения: они завершаются ошибкой, а не пропускают проверку молча.

Для браузерной проверки один раз установите Chromium внутри Linux, затем запустите:

```bash
mise exec -- playwright install chromium
mise run test:browser
```

Тест поднимает настоящие HTTP-обработчики и Vite на 127.0.0.1:5174, работает
с отдельной test DB и обрывает ответ уже после записи заявки. После перезагрузки
страницы проверяет повтор с прежним ключом/телом и ровно одну запись в БД.
Скриншоты desktop/mobile сохраняются в .tmp. Системные библиотеки Chromium добавлены в dev Dockerfile; старый контейнер
нужно пересобрать с хоста, чтобы получить их. При их отсутствии тест завершится
ошибкой, а не пропустит проверку.
Frontend-библиотеки закреплены в frontend/bun.lock, CLI — mise.toml/mise.lock
и связанных .mise/locks (их нужно хранить в Git).

Для первоначальной подготовки уже существующей **локальной dev-БД**
(эти команды не создают тестовые товары):

```bash
mise run db:bootstrap
mise run db:migrate
```

В текущем окружении миграции уже применены. Синтетический каталог добавляется
отдельной командой db:seed; повтор команды не перезаписывает товары.
В DBeaver обновите список схем catalog и requests в БД tireshop_dev.

Bootstrap создаёт технические роли и пустые схемы catalog/requests для таблиц
версий goose. Прикладные таблицы и view создаются только SQL-миграциями:
[каталог](migrations/catalog/00001_products.sql),
[заявки](migrations/requests/00001_requests.sql). Миграции выполняются под
ролью <имя БД>_migrator; runtime-роли <имя БД>_catalog и <имя БД>_requests
не получают DDL и доступа к внутренним таблицам другой области.

Bootstrap/migrations и интеграционные тесты используют административное
подключение с переключением эффективной роли. Работающие API подключаются
отдельными LOGIN-ролями <имя БД>_catalog_login и <имя БД>_requests_login,
наследующими только свою прикладную роль. Настройка — db:runtime ниже.
Пароль администратора не используется в API-пулах; сами dev/agent контейнеры
по-прежнему имеют административный secret для миграций и тестов.

Скрипты работают только с postgres:5432 и разрешёнными dev/test именами БД.
Пароль читается из Compose secret, передаётся через окружение процесса без вывода
и не включается в SQL/аргументы CLI. Скрипты отключают загрузку .env в goose.
Доступ к production не предусмотрен. В первой миграции нет Down: удаление таблиц
не восстанавливает данные; исправления добавляются следующими миграциями.

Для DBeaver на основной ОС PostgreSQL опубликован на localhost:5432;
из контейнеров адрес по-прежнему postgres:5432. Публикация задана на 127.0.0.1.

## Запуск агентного окружения

Команды Docker выполнять в терминале **macOS**, из корня проекта.
Runtime должен работать, активный Docker context — `lighter`.
Перед первым запуском создайте файл локального пароля по инструкции ниже.

```bash
docker compose -f compose.yaml -f compose.agent.yaml up -d --build agent
docker compose -f compose.yaml -f compose.agent.yaml logs -f agent
```

Первый запуск скачивает инструменты. Дождаться `bootstrap complete`; Ctrl+C
закрывает просмотр логов, а контейнер продолжает работать.

```bash
docker compose -f compose.yaml -f compose.agent.yaml ps
docker compose -f compose.yaml -f compose.agent.yaml exec agent bash
```

Внутри контейнера:

```bash
mise exec -- codex --version
mise exec -- codex login --device-auth
mise exec -- codex
```

Вход выполняется пользователем: открыть показанный URL в браузере Mac и ввести
код. Device login должен быть разрешён для аккаунта/организации. Если он
недоступен, подобрать поддерживаемый способ входа отдельно; токены не помещать
в проект. Проверка входа: `mise exec -- codex login status`.
Настройки хранения авторизации по умолчанию используют файл или доступное
системное хранилище; в этом Linux-образе keyring не настроен. При необходимости
явно использовать `codex -c 'cli_auth_credentials_store="file"' login --device-auth`.
См. [авторизацию Codex](https://learn.chatgpt.com/docs/auth).

Каждый запуск entrypoint доверяет конфигурации этого проекта, выполняет
`mise install --locked`, обновляет shims и проверяет CLI. Lockfile не обновляется.
При ошибке контейнер завершается и не объявляется готовым; причина видна в logs.
После исправления выполнить `up -d agent`. Healthcheck проверяет завершение
bootstrap, а не вход в аккаунт и не доступность модели.

## Git: hooks и авторство

Проект использует `.githooks/pre-commit` и `.pre-commit-config.yaml`.
Hadolint отключён для этого проекта; глобальная конфигурация хоста не меняется.
Проверяются размеры файлов, YAML/JSON5, маркеры конфликтов, окончания строк,
пробелы, Markdown и секреты. Отдельный hook ищет точное значение текущего
локального пароля БД в staged diff, включая удалённые строки; совпадения
не выводятся. Без читаемого secret-файла проверка блокирует коммит.
Gitleaks дополнительно ищет известные форматы секретов. Эти проверки не ловят
все преобразования/кодировки, старые пароли и могут быть обойдены через
`--no-verify`; они защищают от случайного коммита, а не от действий процесса.
Markdown проверяется без автоформатирования.
Ограничение Markdown-строк в 80 символов отключено в `.rumdl.toml`;
остальные правила остаются включены.
Версии CLI закреплены в `mise.toml`/`mise.lock`.

Для новой рабочей копии выполнить **на хосте** до запуска agent:

```bash
git config --local user.name "$(git config user.name)"
git config --local user.email "$(git config user.email)"
git config --local core.hooksPath .githooks
```

Первые две команды копируют ваше текущее Git-авторство в `.git/config`.
Если имя/email ещё не настроены, укажите их явно. Этот файл не коммитится,
но доступен контейнеру вместе с рабочей копией. Bootstrap agent и создание
Dev Container также подключают `.githooks`. Запускать `prek install` не нужно.

Обычный `git commit` на Mac вызывает проверки в уже запущенном сервисе `agent`
через Compose; внутри dev/agent проверки выполняются напрямую через mise.
Если agent остановлен или инструмент отсутствует, коммит прерывается.
На Mac используйте `git add` и обычный `git commit`; коммиты с временным индексом
(например, `git commit --only`) выполняйте внутри контейнера.

Установка инструментов и ручная проверка из терминала контейнера:

```bash
mise install --locked
mise run hooks
git var GIT_AUTHOR_IDENT
git var GIT_COMMITTER_IDENT
```

Первая проверка скачивает окружения hooks; требуется сеть. `mise run hooks`
проверяет tracked-файлы, а gitleaks — staged-изменения. Hook при коммите проверяет
staged-содержимое. Если исправлены пробелы или окончания строк, просмотрите diff
и снова добавьте исправления через `git add`. Новые файлы сначала добавьте в Git.
Проверки Go-кода запускаются отдельно задачами test/lint/build, описанными выше.

Агент создаёт только локальные коммиты от вашего имени. Push выполняете вы;
SSH-ключи и токен GitHub контейнеру для авторства не требуются.
Для [статистики GitHub](https://docs.github.com/en/account-and-profile/how-tos/contribution-settings/troubleshooting-missing-contributions)
email автора должен быть привязан к вашему аккаунту (можно использовать ваш
GitHub noreply), а коммиты должны попасть в default branch или `gh-pages`.

## Что сохраняется

| Данные | Место |
|---|---|
| Код | Bind mount проекта `/workspace` |
| XLSX/HTML/фото | `/workspace/.local`, read-only в agent |
| Установленные инструменты mise | Volume `agent_tools` → `/home/vscode/.local/share/mise` |
| Настройки, сессии и файловая авторизация Codex | Volume `agent_codex` → `/home/vscode/.codex`, права 700 |
| PostgreSQL | Существующий volume `postgres_data` |
| Локальный пароль БД | Файл вне проекта `../tireshop-secrets/postgres_password` |

Compose добавляет к именам volumes имя проекта (обычно `tireshop_`).
Путь home с Mac не монтируется. Старые настройки/авторизация из `dev` не
копируются: первый вход в agent выполняется отдельно. Процессы пользователя
vscode внутри agent имеют доступ к его состоянию CLI, включая credentials.

Обычный restart и пересоздание agent сохраняют volumes:

```bash
docker compose -f compose.yaml -f compose.agent.yaml up -d --force-recreate --no-deps agent
```

Не использовать `down -v` для обычной остановки: он удаляет volumes проекта,
включая состояние CLI и данные PostgreSQL. Остановить только агента:

```bash
docker compose -f compose.yaml -f compose.agent.yaml stop agent
```

Выйти из аккаунта: внутри agent выполнить `mise exec -- codex logout`.
Это не удаляет историю сессий. Volumes не являются резервной копией: удаление
данных runtime/VM может их уничтожить.

## Работа с VS Code и ограничения

Текущий `Reopen in Container` по-прежнему открывает человеческий сервис `dev`.
Agent запускается приведёнными командами с Mac; код виден в открытом редакторе.
Отдельная конфигурация VS Code для agent и проверка credential forwarding
ещё не настроены. Не запускать двух писателей одновременно в общей рабочей копии.

Agent работает без root, с cap_drop ALL/no-new-privileges, ограничениями CPU,
RAM и процессов, без sudo-разрешения vscode и без Docker/SSH socket mounts.
Сеть пока не ограничена allowlist. Настройки sandbox Codex не отключены;
полный запрос к модели и совместимость её sandbox проверяются после login.
Это не завершённая настройка для неограниченной автономной работы.

`.local` исключена из Git. `.dockerignore` по решению пользователя сохранён:
это не влияет на read-only mount исходников. Производные данные сохранять
в другом каталоге, например `artifacts/`; приватные отчёты не добавлять в Git.

Обновление инструментов выполняется намеренно через mise внутри Linux,
с review изменений `mise.toml`/`mise.lock`. Для применения нового lockfile
перезапустить agent. Если изменён Dockerfile/bootstrap, повторить `up -d --build agent`.

## Проверка сохранности локальной PostgreSQL

В контейнере выполнить:

```bash
bash scripts/check-db-persistence.sh prepare
```

Скрипт создаёт отдельную техническую БД `tireshop_envcheck` и синтетическую
контрольную запись. Он использует локальный сервис `postgres:5432` и параметры
разработки из Compose; пароль читается из `PGPASSWORD_FILE`, переданного Compose.
Маркер и время запуска сервера сохраняются в игнорируемой `.tmp/`.
Не удалять этот файл между подготовкой и проверкой.

В терминале Mac из корня проекта:

```bash
docker compose -f compose.yaml -f compose.agent.yaml restart postgres
```

После готовности PostgreSQL, снова в контейнере:

```bash
bash scripts/check-db-persistence.sh verify
```

Успех требует и изменения времени запуска сервера, и наличия ранее записанного
маркера. Без перезапуска скрипт завершится с ошибкой. Техническая БД остаётся
для повторной проверки; данные приложения скрипт не изменяет. Эта проверка
подтверждает обычный restart, но не заменяет проверку восстановления из backup.

## Локальные секреты

Пароль хранится вне рабочей копии и build context. Первичная настройка файла
на Mac из корня проекта (команда не перезаписывает существующий файл):

```bash
mkdir -p ../tireshop-secrets
chmod 700 ../tireshop-secrets
(umask 077; set -C; openssl rand -hex 32 > ../tireshop-secrets/postgres_password)
```

`openssl` здесь только создаёт локальный файл; проектные инструменты работают
в Linux. Файл нельзя добавлять в Git или выводить в терминал. Для другого пути
скопируйте `.env.example` в `.env` и задайте `POSTGRES_PASSWORD_PATH`: это путь
на хосте, а не пароль. `.env` игнорируется Git и Docker; `.secrets/` тоже
исключена на случай ошибочного хранения внутри проекта.

Compose монтирует файл read-only в `/run/secrets/postgres_password` у
`postgres`, `dev` и `agent` (agent наследует доступ от dev). PostgreSQL использует
`POSTGRES_PASSWORD_FILE`, наши скрипты — `PGPASSWORD_FILE`. Это переменные
с путём; libpq/обычный `psql` самостоятельно `PGPASSWORD_FILE` не понимает.
Скрипт persistence читает файл и задаёт `PGPASSWORD` только своему процессу
и дочерним клиентам БД. В конфигурации environment контейнеров пароля нет.
Compose secrets — локальные файлы, не зашифрованное хранилище.

Применить конфигурацию с Mac, сохранив volumes:

```bash
docker compose -f compose.yaml -f compose.agent.yaml up -d postgres
docker compose -f compose.yaml -f compose.agent.yaml exec -T --user postgres postgres bash -s < scripts/apply-db-password.sh
docker compose -f compose.yaml -f compose.agent.yaml up -d dev agent
```

Первая команда пересоздаёт контейнер БД с secret mount. Для уже созданной БД
`POSTGRES_PASSWORD_FILE` не меняет пароль роли: вторая команда применяет его
через локальный Unix socket, без значения в аргументах и без вывода пароля.
Она предназначена только для нашей локальной БД. Данные и volume сохраняются.
Подключения с прежним паролем перестанут работать; открытое окно VS Code может
потребовать повторного подключения к dev после пересоздания контейнера.
Для смены пароля запишите новое случайное значение в файл и повторите команды;
если файл заменён целиком, используйте `--force-recreate` у обеих команд `up`,
чтобы bind mounts получили новый файл. Не используйте `down -v`.

Агент получает только локальный пароль для синтетических данных. Он может
прочитать его из secret mount и технически отправить через доступную сеть.
Правила AGENTS.md и hooks уменьшают риск ошибки, но не дают гарантии изоляции.
Production-секреты не передаются в dev/agent, AI-сессии и эту Compose-конфигурацию;
в будущем их передаёт отдельно управляемая среда production только runtime-сервисам.
Для строгого запрета отправки локальных данных потребуется отдельное ограничение
исходящей сети; сейчас оно не настроено.

См. [Compose secrets](https://docs.docker.com/compose/how-tos/use-secrets/)
и [настройки образа PostgreSQL](https://hub.docker.com/_/postgres).
