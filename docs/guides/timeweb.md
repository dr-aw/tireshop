# Пилот статической витрины на Timeweb

App Platform: GitHub, React/Bun, корень проекта пустой, команда сборки
`bash scripts/timeweb-static-build.sh`, выходная директория `frontend/dist`
(относительно корня, без начального слеша).

Публичные переменные сборки:

- SSG_CATALOG_URL: полный HTTPS URL catalog API с /api/products/snapshot,
  тот же, что на Render.
- SITE_ORIGIN: HTTPS origin технического домена Timeweb для пилота.
- VITE_MEDIA_BASE_URL: существующий публичный base фотографий с Render.

Пароли БД, CA Aiven, Google/GitHub credentials приложению не нужны.
Runner устанавливает только frontend toolchain по mise.lock, требует live
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
