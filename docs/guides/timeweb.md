# Пилот статической витрины на Timeweb

Подтверждено вторым логом Timeweb: весь репозиторий доступен в /app,
зависимости ставятся из frontend, после cd .. runner запускается из корня.

| Настройка | Значение |
| --- | --- |
| Путь проекта | frontend |
| Команда сборки | cd .. && bash scripts/timeweb-static-build.sh |
| Директория сборки | dist |
| Системные зависимости | curl git unzip xz-utils |

Пакеты указать в расширенном поле «Зависимости»: платформа установит их
через apt до build-команды. В базовом Bun-образе из лога есть ca-certificates,
но нет curl. curl нужен для установки mise, unzip/xz-utils — для архивов
инструментов, git — для операций mise/инструментов. Корневой package.json
при этой конфигурации не нужен. После настройки пакетов требуется новый deploy.
Реальное завершение сборки на Timeweb пока не подтверждено.

Публичные переменные сборки:

- SSG_CATALOG_URL: полный HTTPS URL catalog API с /api/products/snapshot,
  тот же, что на Render.
- SITE_ORIGIN: HTTPS origin технического домена Timeweb для пилота.
- VITE_MEDIA_BASE_URL: `https://storage.googleapis.com/tireshop-513806-product-media`
  без /media в конце. Необязательная настройка кода; без неё фото идут через
  /media на домене сайта и потребуют отдельного proxy. Прямые фото по-прежнему
  зависят от доступности Google Storage для покупателя.

Пароли БД, CA Aiven, Google/GitHub credentials приложению не нужны.
Runner устанавливает только frontend toolchain по mise.lock, экспортирует
PATH выбранных bun/node/tsc/vite и вызывает задачи с --skip-tools. Без этого
флага mise run повторно устанавливает backend/infra инструменты из общего
mise.toml (в третьем логе это привело к падению на Terraform). Runner требует live
snapshot и запрещает локальный файл. Без переменных сборка останавливается,
синтетический fixture в этом пути не публикуется.

Для пилота выключить SPA Fallback, проверить /, /about/, /catalog/tires/,
/products/{существующий UUID}, неизвестный путь и /publication.json.
Разрешение extensionless product URL требуется проверить на платформе.
Настоящий запуск установки и сборки внутри Timeweb ещё не проверен.

Статический пилот не завершает миграцию магазина. Клиент обращается к /api
на своём origin; поддержка внешнего proxy для frontend App Platform не
подтверждена. Нужен ответ поддержки или отдельный Nginx/Caddy/Docker-путь,
проверка API/cookies/origins, обеих частей backend и реальной покупки.
Глобальный SPA fallback не заменяет API proxy.

Документация: [деплой](https://timeweb.cloud/docs/apps/deploying-frontend-apps),
[веб-сервер](https://timeweb.cloud/docs/apps/reverse-proxy).
