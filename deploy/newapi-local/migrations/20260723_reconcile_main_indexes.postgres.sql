\set ON_ERROR_STOP on

-- Run once while new-api is stopped. The transaction is idempotent and keeps
-- the index names/column order used by origin/main. deploy-dev-only tables keep
-- their explicit model index names.
BEGIN;

SELECT pg_advisory_xact_lock(hashtext('new-api:reconcile-main-indexes:20260723'));

-- main uses the partial unique index uk_prefill_name. Older installations can
-- also contain an unconditional UNIQUE(name) constraint/index under a generated
-- name. GORM sees that as a column-level unique constraint and tries to drop a
-- different generated constraint name.
DO $migration$
BEGIN
    IF to_regclass('public.prefill_groups') IS NOT NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS uk_prefill_name
            ON public.prefill_groups (name)
            WHERE deleted_at IS NULL;

        ALTER TABLE public.prefill_groups
            DROP CONSTRAINT IF EXISTS uni_prefill_groups_name;
        ALTER TABLE public.prefill_groups
            DROP CONSTRAINT IF EXISTS idx_prefill_groups_name;
        DROP INDEX IF EXISTS public.uni_prefill_groups_name;
        DROP INDEX IF EXISTS public.idx_prefill_groups_name;
    END IF;
END
$migration$;

-- Phone is deploy-dev-only, so retain its explicit model index name. Remove
-- aliases produced by earlier tags or PostgreSQL column constraints.
DO $migration$
BEGIN
    IF to_regclass('public.users') IS NOT NULL
       AND EXISTS (
           SELECT 1
           FROM pg_attribute
           WHERE attrelid = 'public.users'::regclass
             AND attname = 'phone'
             AND NOT attisdropped
       ) THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique
            ON public.users (phone);

        ALTER TABLE public.users DROP CONSTRAINT IF EXISTS uni_users_phone;
        ALTER TABLE public.users DROP CONSTRAINT IF EXISTS users_phone_key;
        ALTER TABLE public.users DROP CONSTRAINT IF EXISTS idx_users_phone;
        DROP INDEX IF EXISTS public.uni_users_phone;
        DROP INDEX IF EXISTS public.users_phone_key;
        DROP INDEX IF EXISTS public.idx_users_phone;
    END IF;
END
$migration$;

-- These deploy-dev fields use named unique indexes. Remove older column-level
-- constraints only after the canonical indexes have been created.
DO $migration$
BEGIN
    IF to_regclass('public.user_subscriptions') IS NOT NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_user_subscription_provider_invoice
            ON public.user_subscriptions (provider_invoice_unique_id);

        ALTER TABLE public.user_subscriptions
            DROP CONSTRAINT IF EXISTS uni_user_subscriptions_provider_invoice_unique_id;
        ALTER TABLE public.user_subscriptions
            DROP CONSTRAINT IF EXISTS user_subscriptions_provider_invoice_unique_id_key;
        DROP INDEX IF EXISTS public.uni_user_subscriptions_provider_invoice_unique_id;
        DROP INDEX IF EXISTS public.user_subscriptions_provider_invoice_unique_id_key;
    END IF;
END
$migration$;

DO $migration$
BEGIN
    IF to_regclass('public.billing_subscriptions') IS NOT NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_subscription_signup_reference
            ON public.billing_subscriptions (signup_reference_unique_id);

        ALTER TABLE public.billing_subscriptions
            DROP CONSTRAINT IF EXISTS uni_billing_subscriptions_signup_reference_unique_id;
        ALTER TABLE public.billing_subscriptions
            DROP CONSTRAINT IF EXISTS idx_billing_subscriptions_signup_reference_unique_id;
        DROP INDEX IF EXISTS public.uni_billing_subscriptions_signup_reference_unique_id;
        DROP INDEX IF EXISTS public.idx_billing_subscriptions_signup_reference_unique_id;
    END IF;
END
$migration$;

-- deploy-dev temporarily reversed this composite index. main defines
-- CreatedAt priority 1 and Id priority 2.
DO $migration$
BEGIN
    IF to_regclass('public.logs') IS NOT NULL THEN
        DROP INDEX IF EXISTS public.idx_created_at_id;
        CREATE INDEX idx_created_at_id
            ON public.logs (created_at, id);
    END IF;
END
$migration$;

COMMIT;

SELECT tablename, indexname, indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND indexname IN (
      'uk_prefill_name',
      'idx_users_phone_unique',
      'idx_user_subscription_provider_invoice',
      'idx_billing_subscription_signup_reference',
      'idx_created_at_id'
  )
ORDER BY tablename, indexname;
