CREATE TABLE discord_users (
    user_id      TEXT NOT NULL,
    guild_id     TEXT NOT NULL,
    username     TEXT NOT NULL,
    display_name TEXT NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, guild_id)
);
