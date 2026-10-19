# tireshop

Планы: [PLANS.md](PLANS.md). Правила: [AGENTS.md](AGENTS.md).

## Запуск агентного окружения

Команды Docker выполнять в терминале **macOS**, из корня проекта.
Runtime должен работать, активный Docker context — `lighter`.

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

## Что сохраняется

| Данные | Место |
|---|---|
| Код | Bind mount проекта `/workspace` |
| XLSX/HTML/фото | `/workspace/.local`, read-only в agent |
| Установленные инструменты mise | Volume `agent_tools` → `/home/vscode/.local/share/mise` |
| Настройки, сессии и файловая авторизация Codex | Volume `agent_codex` → `/home/vscode/.codex`, права 700 |
| PostgreSQL | Существующий volume `postgres_data` |

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
