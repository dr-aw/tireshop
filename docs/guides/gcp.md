# API в Google Cloud Run: первый запуск

Цель: два Go API в проекте `tireshop-513806`, регион `asia-southeast1`,
frontend на Render, PostgreSQL 18.6 в Aiven Singapore. Миграции ручные.
Эти действия выполняет пользователь в своей доверенной Linux admin-среде
(например Cloud Shell), не в dev/agent container. Не передавать Google/Aiven
credentials агенту. Ни одна команда ниже ещё не выполнялась в облаке агентом.

## Что делает Terraform

`deploy/terraform/main.tf` описывает желаемые ресурсы. `init` скачивает provider,
`validate` проверяет структуру, `plan` показывает изменения, `apply` создаёт их.
Terraform не пересобирает приложение, не применяет SQL и не хранит значения
наших secrets. Версия CLI задаётся mise.toml/mise.lock, provider закреплён
в конфигурации и `.terraform.lock.hcl`.

Сначала `deploy_services=false`: создаются Artifact Registry, два service accounts,
три пустых Secret Manager secrets и права доступа. Второй этап запускает сервисы,
когда уже есть образы, схема/роли БД и секреты. На каждом этапе просмотреть plan.

State — учёт созданных ресурсов, его нужно сохранять между запусками. На старте
он локален в admin-среде; не добавлять в Git, не удалять и не передавать агенту.
Перед совместной эксплуатацией перенести в приватный GCS backend с ограничением
доступа, версионированием и locking. Значения секретов не вводить в tfvars:
`sensitive` само по себе не исключает их из state. Облачные расходы после apply
включают registry/secrets и работу Cloud Run; задать Billing alerts в консоли.

## 1. Создать инфраструктуру без API

В admin-среде открыть checkout проверенного commit и установить инструменты
по `mise install --locked`. В Cloud Shell gcloud уже есть; в другой среде
установить/авторизовать его отдельно, без скачивания service-account JSON key.
Для Terraform вне Cloud Shell обычно нужен `gcloud auth application-default login`.

```bash
gcloud config set project tireshop-513806
umask 077
mise exec -- terraform -chdir=deploy/terraform init
mise exec -- terraform -chdir=deploy/terraform validate
mise exec -- terraform -chdir=deploy/terraform plan -out=foundation.tfplan
mise exec -- terraform -chdir=deploy/terraform apply foundation.tfplan
```

Нужны привязанный billing account и права создавать ресурсы/API/IAM. Ошибку
permissions разбирать по конкретной операции, не выдавать runtime account Owner.
`prevent_destroy` защищает registry/secrets; Cloud Run также защищён от удаления.

## 2. Подготовить Aiven и Secret Manager

Адрес БД: `pg-2f7b3aad-snorchello-5783.g.aivencloud.com:22201`, база `defaultdb`.
Успешный DBeaver не подтверждает наличие таблиц и ограниченных ролей приложения.
До запуска сервисов требуется проверить доступные Aiven права и подготовить:

- NOLOGIN-роли `defaultdb_catalog`, `defaultdb_requests`, `defaultdb_migrator`;
- отдельные LOGIN `defaultdb_catalog_login` и `defaultdb_requests_login`,
  наследующие соответствующие runtime-права, без DDL/CREATEROLE/CREATEDB;
- схемы catalog/requests и ручные миграции goose под ролью мигратора,
  с `catalog.goose_db_version` и `requests.goose_db_version`;
- проверку GRANT, backup/restore и создание первого администратора.

Bootstrap и migration runner подготовлены; ручная процедура описана в конце
этого руководства. **Не перенаправлять scripts/local-db-env.sh в Aiven.**
Не запускать SQL-файлы с Up/Down целиком в DBeaver. До готовности схемы оставить
`deploy_services=false`. Нужен новый image после изменения runtime, старые packages
с жёсткой привязкой к локальному postgres не подойдут.

В консоли GCP Secret Manager добавить версии в созданные секреты:

| Secret | Содержимое |
| --- | --- |
| `tireshop-catalog` | Пароль `defaultdb_catalog_login` |
| `tireshop-requests` | Пароль `defaultdb_requests_login` |
| `tireshop-ca` | Содержимое CA PEM, скачанного из Aiven |

Terraform не читает эти значения. Номера версий указываются в `secret_versions`
(первые версии по умолчанию `1`). Каждый API получает только свой пароль и CA.
Пароли мигратора и администратора магазина этим сервисам не передаются.

Aiven остаётся публичным TLS endpoint. Cloud Run по умолчанию не имеет
фиксированного исходящего IP. Для строгого Aiven allowlist потребуется отдельное
решение со стабильным egress (VPC/NAT и его стоимость), или другой хостинг;
до реальных данных явно согласовать сетевой доступ. Не считать один регион
приватной сетью. Учитывать Aiven connection limit: два сервиса × два экземпляра ×
четыре соединения = 16, плюс перекрытие ревизий и административные соединения.

## 3. Скопировать выбранные образы в Artifact Registry

После успешного CI взять по одному GHCR reference с digest для каждого API.
Никакой новой сборки не требуется. Пример для каталога, повторить для заявок:

```bash
gcloud auth configure-docker asia-southeast1-docker.pkg.dev
# Заменить OWNER/REPO и DIGEST; SHA — тот же commit для обоих сервисов.
docker pull ghcr.io/OWNER/REPO/catalog@sha256:DIGEST
docker tag ghcr.io/OWNER/REPO/catalog@sha256:DIGEST \
  asia-southeast1-docker.pkg.dev/tireshop-513806/tireshop/catalog:SHA
docker push asia-southeast1-docker.pkg.dev/tireshop-513806/tireshop/catalog:SHA
```

Для private GHCR отдельно авторизовать Docker read-only package credential
через `docker login --password-stdin`; не передавать токен через argv, Git или
Terraform. Права этого credential не нужны Cloud Run после копирования.
Для запуска взять **destination digest** из Artifact Registry — он может
отличаться от исходного manifest digest. Образы — linux/amd64.

## 4. Запустить API после ручных миграций

Создать игнорируемый Git файл `deploy/terraform/production.tfvars`:

```hcl
deploy_services = true
images = {
  catalog  = "asia-southeast1-docker.pkg.dev/tireshop-513806/tireshop/catalog@sha256:REPLACE_WITH_DIGEST"
  requests = "asia-southeast1-docker.pkg.dev/tireshop-513806/tireshop/requests@sha256:REPLACE_WITH_DIGEST"
}
```

```bash
mise exec -- terraform -chdir=deploy/terraform plan -out=release.tfplan
mise exec -- terraform -chdir=deploy/terraform apply release.tfplan
mise exec -- terraform -chdir=deploy/terraform output service_urls
```

Cloud Run: request-based CPU, минимум 0, максимум 2 экземпляра в ревизии,
512 MiB памяти. При простое возможен холодный старт. Startup проверяет /readyz,
liveness /healthz не зависит от доступности БД. Неудачный startup из-за схемы,
CA или ролей требует исправления причины, не отключения проверки сертификатов.
Лимит экземпляров при rollout и число соединений контролировать по метрикам.

API публичны для вызовов Render proxy. IAM public invocation не даёт доступа
к административным операциям: приложение проверяет сессию, Origin и права.
Это также означает, что сторонний HTTP клиент может обращаться к публичному API;
Origin не является сетевой аутентификацией. Для защиты от злоупотреблений нужен
отдельный этап rate limiting и наблюдения перед реальными заявками.

## 5. Подключить Render и проверить

В Render добавить специфичные API rewrites раньше SPA: products и
admin/products в catalog, requests и остальные admin маршруты в requests,
с сохранением полного пути. Проверить POST/PATCH, Set-Cookie, вход/выход и Origin.
Frontend должен обращаться к `/api` на собственном origin, без DB secrets.
Медиа и готовый HTML для SEO остаются отдельными задачами плана.

Первый релиз проверить на синтетике: товар/фото, повтор заявки, админка,
списание, restart/cold start. Откат — предыдущие image digest; миграции БД
автоматически не откатываются. Новые SQL должны сохранять совместимость с
работающей старой ревизией. Коммиты/push выполняет пользователь.

Справка: [Cloud Run secrets](https://docs.cloud.google.com/run/docs/configuring/services/secrets),
[Terraform resource](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service),
[Aiven TLS](https://aiven.io/docs/platform/concepts/tls-ssl-certificates).

## Подготовка пустой Aiven: ручная процедура

Для PostgreSQL 18 подготовлен [bootstrap.sql](../../deploy/aiven/bootstrap.sql).
Это административный SQL без паролей, не миграция таблиц. Запускать **один раз**
в выделенной пустой `defaultdb` под её владельцем с CREATEROLE, без superuser.
CREATEDB означает право создавать новые БД, а не права владельца существующей.
В DBeaver сначала проверить:

```sql
SELECT current_user, pg_get_userbyid(datdba) AS database_owner,
       has_database_privilege(current_user, current_database(), 'CREATE') AS can_create_schema
FROM pg_database WHERE datname = current_database();
```

Затем открыть bootstrap.sql в редакторе, привязать именно к `defaultdb` и выполнить
**как скрипт целиком**. Он создаёт роли и схемы внутри транзакции, без таблиц и
паролей; при конфликте имён или ошибке всё откатывается. Если DBeaver оставил
транзакцию в aborted state, выполнить `ROLLBACK` и разобрать ошибку. Повторный
запуск после успеха намеренно отклоняется: bootstrap не является обновлением БД.
Он снимает CREATE/TEMP у PUBLIC в выделенной БД — для БД с другими приложениями
эта процедура без отдельного review не подходит.

Проверить результат (паролей запрос не возвращает):

```sql
SELECT rolname, rolcanlogin, rolcreaterole, rolcreatedb
FROM pg_roles
WHERE rolname IN ('defaultdb_catalog', 'defaultdb_requests', 'defaultdb_migrator',
                 'defaultdb_catalog_login', 'defaultdb_requests_login',
                 'defaultdb_migration_login')
ORDER BY rolname;

SELECT schema_name, schema_owner
FROM information_schema.schemata
WHERE schema_name IN ('catalog', 'requests');
```

Дальше в доверенной пользовательской admin-среде открыть psql как текущий
Aiven administrator, с `sslmode=verify-full` и CA. Пароль администратора вводить
в скрытый prompt (`-W`), не включать в URI. В интерактивном psql выполнить:

```text
\password defaultdb_catalog_login
\password defaultdb_requests_login
\password defaultdb_migration_login
```

Три разных случайных пароля сгенерировать в своём password manager. `\password`
не пишет пароль в историю и хеширует перед отправкой серверу. Runtime-пароли
впоследствии добавить в соответствующие GCP secrets; пароль мигратора оставить
только в пользовательской admin-среде, в отдельном файле с правами 600 вне Git.
Не использовать обычный ALTER ROLE ... PASSWORD с открытым паролем в DBeaver.

Скрипт [aiven-migrate.sh](../../scripts/aiven-migrate.sh) — только для ручного
запуска пользователем. Цель фиксирована: этот Aiven host, порт 22201, defaultdb,
LOGIN `defaultdb_migration_login`. CA и пароль берутся из файлов. Пример путей
ниже заменить своими; никаких production файлов в dev/agent не монтировать:

```bash
export PGSSLROOTCERT=/absolute/private/path/ca.pem
export MIGRATION_PASSWORD_FILE=/absolute/private/path/migration-password
bash scripts/aiven-migrate.sh status
# Версии текущего набора: catalog 4, requests 3. Сверить SQL выбранного commit.
bash scripts/aiven-migrate.sh up-to 4 3
bash scripts/aiven-migrate.sh status
```

Runner сначала мигрирует catalog, затем requests. При ошибке не продолжает.
Диагностика goose остаётся в приватном временном файле с правами 600; путь
выводится без содержимого. Не публиковать сырые логи/URI. Проверить версии
можно из DBeaver:

```sql
SELECT version_id, is_applied FROM catalog.goose_db_version ORDER BY id;
SELECT version_id, is_applied FROM requests.goose_db_version ORDER BY id;
```

Это история применений, не просто максимальный номер версии. Миграции создают
пустой каталог, заявки и таблицы авторизации. Данные, фото и первый администратор
готовятся отдельно; до этого API нельзя считать готовыми к реальным заказам.
Bootstrap и миграции проверены только на локальной PostgreSQL с синтетикой,
под non-superuser оператором и отдельным migration LOGIN. Aiven-специфичные
ограничения и реальное применение пользователь подтверждает отдельно.
