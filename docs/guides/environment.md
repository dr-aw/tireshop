# Окружение разработчика

[Краткий запуск](../../README.md) · [Разработка](development.md)

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
