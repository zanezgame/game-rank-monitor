-- 游戏榜单监控平台初始化建表 SQL（SQLite）。
-- 榜单快照主键维度：platform × region × chart_type × period × date。

CREATE TABLE IF NOT EXISTS games (
    id          TEXT PRIMARY KEY,            -- platform:region:externalID
    platform    TEXT NOT NULL,
    region      TEXT NOT NULL,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    icon_url    TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    price_text  TEXT NOT NULL DEFAULT '',
    tags_json   TEXT NOT NULL DEFAULT '[]',
    first_seen  TEXT NOT NULL DEFAULT '',   -- YYYY-MM-DD
    created_at  TEXT NOT NULL DEFAULT ''    -- RFC3339
);
CREATE INDEX IF NOT EXISTS idx_games_platform ON games(platform, region);

CREATE TABLE IF NOT EXISTS rank_snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id     TEXT NOT NULL REFERENCES games(id),
    platform    TEXT NOT NULL,
    region      TEXT NOT NULL,
    chart_type  TEXT NOT NULL,               -- hot / new
    period      TEXT NOT NULL,               -- daily / weekly
    date        TEXT NOT NULL,               -- YYYY-MM-DD
    rank        INTEGER NOT NULL,
    rank_change INTEGER,                     -- NULL 表示无历史可比
    reason_code TEXT NOT NULL DEFAULT '',
    reason_text TEXT NOT NULL DEFAULT '',
    player_count INTEGER                     -- NULL 表示未采集
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_rank_snapshots
    ON rank_snapshots(platform, region, chart_type, period, date, game_id);
CREATE INDEX IF NOT EXISTS idx_rank_snapshots_game
    ON rank_snapshots(game_id, platform, region, chart_type, period, date);

CREATE TABLE IF NOT EXISTS player_samples (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id     TEXT NOT NULL REFERENCES games(id),
    sampled_at  TEXT NOT NULL,               -- RFC3339
    count       INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_player_samples_game
    ON player_samples(game_id, sampled_at);

CREATE TABLE IF NOT EXISTS discussions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id     TEXT NOT NULL REFERENCES games(id),
    source      TEXT NOT NULL,
    author      TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL,
    sentiment   TEXT NOT NULL DEFAULT 'neutral',  -- positive / negative / neutral
    posted_at   TEXT NOT NULL,                    -- RFC3339
    url         TEXT NOT NULL DEFAULT ''
);
-- 按 source+author+content 去重（content 哈希避免超长索引问题）
CREATE UNIQUE INDEX IF NOT EXISTS uq_discussions
    ON discussions(game_id, source, author, substr(content, 1, 200));
CREATE INDEX IF NOT EXISTS idx_discussions_game
    ON discussions(game_id, posted_at);
