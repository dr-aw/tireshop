-- Run ONCE as the database owner/admin in DBeaver or psql, not as an API login.
-- No passwords. No application tables. Do not modify after applying.
BEGIN;
DO $bootstrap$
DECLARE
  db text := current_database();
  operator_name text := current_user;
  suffix text;
BEGIN
  IF db = 'postgres' OR db = 'template0' OR db = 'template1' THEN
    RAISE EXCEPTION 'Choose the application database';
  END IF;
  IF length(db) > 35 THEN RAISE EXCEPTION 'Database name too long for role names'; END IF;
  IF NOT has_database_privilege(current_user, db, 'CREATE') THEN
    RAISE EXCEPTION 'Operator needs CREATE on this database, not just CREATEDB';
  END IF;
  IF NOT pg_has_role(current_user, (SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = db), 'USAGE') THEN
    RAISE EXCEPTION 'Bootstrap needs the database owner or inherited owner rights to restrict PUBLIC privileges';
  END IF;
  IF EXISTS (SELECT FROM pg_namespace WHERE nspname IN ('catalog','requests')) THEN
    RAISE EXCEPTION 'Bootstrap requires absent catalog/requests schemas; inspect existing state';
  END IF;
  FOREACH suffix IN ARRAY ARRAY['_catalog','_requests','_migrator','_catalog_login','_requests_login','_migration_login'] LOOP
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = db || suffix) THEN
      RAISE EXCEPTION 'Application role already exists; inspect existing state';
    END IF;
  END LOOP;
  FOREACH suffix IN ARRAY ARRAY['_catalog','_requests','_migrator'] LOOP
    EXECUTE format('CREATE ROLE %I NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE', db || suffix);
  END LOOP;
  FOREACH suffix IN ARRAY ARRAY['_catalog_login','_requests_login','_migration_login'] LOOP
    -- No password yet: operator sets it interactively with psql \password.
    EXECUTE format('CREATE ROLE %I LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE', db || suffix);
  END LOOP;
  -- Since PostgreSQL 16, CREATEROLE alone does not give automatic SET permission.
  EXECUTE format('GRANT %I TO %I WITH SET TRUE, INHERIT FALSE', db || '_migrator', operator_name);
  EXECUTE format('CREATE SCHEMA catalog AUTHORIZATION %I', db || '_migrator');
  EXECUTE format('CREATE SCHEMA requests AUTHORIZATION %I', db || '_migrator');
  EXECUTE format('GRANT %I TO %I WITH ADMIN FALSE, SET TRUE, INHERIT FALSE', db || '_migrator', db || '_migration_login');
  FOREACH suffix IN ARRAY ARRAY['_catalog','_requests'] LOOP
    EXECUTE format('GRANT %I TO %I WITH ADMIN FALSE, SET FALSE, INHERIT TRUE', db || suffix, db || suffix || '_login');
  END LOOP;
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO %I, %I, %I', db,
    db || '_catalog_login', db || '_requests_login', db || '_migration_login');
  -- This is a dedicated application DB: runtime logins must not create objects
  -- via the PUBLIC role. Existing other applications need a separate review.
  EXECUTE format('REVOKE CREATE, TEMPORARY ON DATABASE %I FROM PUBLIC', db);
END
$bootstrap$;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
COMMIT;
