-- Creates a dedicated non-superuser role for the bot and hands it the bot's
-- database. Run once as postgres, connected to the bot database:
--   psql "postgres://postgres:...@host:port/reels_bot" \
--        -v bot_password="'<strong password>'" -f scripts/create_db_user.sql
-- Then switch DATABASE_URL to user reels_bot.

CREATE ROLE reels_bot LOGIN PASSWORD :bot_password
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;

REVOKE ALL ON DATABASE reels_bot FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE reels_bot TO reels_bot;

-- The bot applies migrations on startup, so it owns its schema objects.
ALTER SCHEMA public OWNER TO reels_bot;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

DO $$
DECLARE
    t record;
BEGIN
    -- Changing table ownership also moves the sequences they own.
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('ALTER TABLE public.%I OWNER TO reels_bot', t.tablename);
    END LOOP;
END
$$;
