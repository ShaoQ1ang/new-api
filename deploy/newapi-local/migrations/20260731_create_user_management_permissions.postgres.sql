\set ON_ERROR_STOP on

-- Hotfix for deployments created from builds where
-- UserManagementPermission was omitted from the application AutoMigrate list.
-- The application should be stopped while this migration runs.
BEGIN;

SELECT pg_advisory_xact_lock(
    hashtext('new-api:create-user-management-permissions:20260731')
);

DO $migration$
BEGIN
    IF to_regclass('public.user_management_permissions') IS NULL THEN
        CREATE SEQUENCE IF NOT EXISTS
            public.user_management_permissions_id_seq AS bigint;

        CREATE TABLE public.user_management_permissions (
            id bigint NOT NULL
                DEFAULT nextval('public.user_management_permissions_id_seq'),
            user_id bigint NOT NULL,
            permission varchar(64) NOT NULL,
            granted_by bigint NOT NULL,
            created_time bigint NOT NULL,
            CONSTRAINT user_management_permissions_pkey PRIMARY KEY (id)
        );

        ALTER SEQUENCE public.user_management_permissions_id_seq
            OWNED BY public.user_management_permissions.id;
    END IF;
END
$migration$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_management_permission
    ON public.user_management_permissions (user_id, permission);

CREATE INDEX IF NOT EXISTS idx_user_management_permission_user
    ON public.user_management_permissions (user_id);

COMMIT;

SELECT
    to_regclass('public.user_management_permissions') AS table_name,
    indexname,
    indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND tablename = 'user_management_permissions'
ORDER BY indexname;
