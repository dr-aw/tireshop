# tireshop

Магазин шин и дисков: каталог, корзина, заявки и админка `/administrator`.
Покупатель отправляет заявку, сотрудник обрабатывает её; онлайн-оплаты нет.

## Архитектура и инструменты

| Компонент | Для чего используется |
| --- | --- |
| Go: `catalog` | Товары, цены, остатки и API каталога |
| Go: `requests` | Заявки, статусы, история и авторизация администратора |
| PostgreSQL | Общая БД с отдельными схемами и правами сервисов |
| React + TypeScript | Витрина и админка; frontend поставляется как статика |
| Bun + Vite | Установка frontend-зависимостей и сборка HTML/CSS/JS |
| goose | Версионируемые SQL-миграции отдельно от запуска API |
| mise | Версии инструментов в `mise.toml`/`mise.lock`, команды проекта |
| Docker Compose | Локальная PostgreSQL и Linux dev container |
| GitHub Actions + GHCR | Проверки и публикация образов двух API |
| Render + Aiven | Планируемый хостинг frontend/API и PostgreSQL |

Каждый API запускается независимо и изменяет только свои данные. Для проверки
цены заявки читают ограниченное SQL-представление каталога. Backend проверяет
цены и права, атомарно сохраняет заявки и защищает от повторной отправки.
Обоснования — в [архитектурных решениях](docs/decisions/).

## Локальный запуск

Нужны Docker/Compose и VS Code Dev Containers. На хосте из корня проекта
однократно подготовить секреты и контейнеры (существующие пароли сохраняются):

```bash
mkdir -p ../tireshop-secrets
chmod 700 ../tireshop-secrets
(umask 077; set -C; openssl rand -hex 32 > ../tireshop-secrets/postgres_password)
bash scripts/create-runtime-secrets.sh
docker compose up -d --build dev
```

Открыть проект через **Dev Containers: Reopen in Container**. В контейнере:

```bash
mise install --locked
mise run frontend:install
mise run db:bootstrap
mise run db:migrate
mise run db:runtime
mise run db:seed
mise run dev
```

Открыть <http://localhost:5173> через VS Code Ports. Для ежедневного запуска
достаточно `mise run dev`; после изменения Go-кода перезапустить эту команду.
Для существующей БД со сменённым паролем нужна
[отдельная процедура](docs/guides/environment.md#локальные-секреты).
Не использовать `docker compose down -v`: это удалит данные.

Проверки внутри контейнера:

```bash
mise run test
mise run lint
mise run build
mise run test:browser
```

Последняя команда проверяет API, миграции и браузер на временной test DB.
[Создание администратора, импорт и обслуживание](docs/guides/development.md).

## CI и деплой

1. Push/merge в `main` запускает CI: тесты, lint, сборку и публикацию двух образов
   в GHCR с тегом SHA и digest. В `dev` Actions пока не запускается.
2. Миграции Aiven выполняются вручную через goose: сначала каталог, затем заявки.
   После успешных миграций обновляются API и frontend выбранного commit.
3. Render API: два Web Service из образов GHCR. Frontend: Static Site из `main`,
   Build Command `bash scripts/render-static-build.sh`, Publish Directory
   `frontend/dist`. Автодеплой пока выключен.
4. Секреты БД — только в соответствующем API на Render через Secret Files;
   в GitHub для текущего CI достаточно автоматического `GITHUB_TOKEN`.
   Frontend не получает паролей. Подключение Aiven должно проверять TLS и CA.

**Production ещё не готов:** runtime пока ограничен локальной БД; подключение
Aiven и доставка фотографий, исключённых из Git, требуют подготовки.
[Настройка секретов, Render и ручной релиз](docs/guides/deployment.md).

[Текущий план](PLANS.md) · [Правила работы](AGENTS.md) ·
[Подробности окружения](docs/guides/environment.md)
