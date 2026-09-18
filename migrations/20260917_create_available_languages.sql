CREATE TABLE IF NOT EXISTS available_languages (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_available_languages_sort_order
    ON available_languages (sort_order);

INSERT INTO available_languages (code, name, sort_order) VALUES
    ('cn', '简体中文', 10),
    ('cht', '繁体中文', 20),
    ('en', '英文', 30),
    ('jp', '日语', 40),
    ('pt', '葡萄牙语', 50),
    ('es', '西班牙语', 60),
    ('ru', '俄语', 70),
    ('de', '德语', 80),
    ('fr', '法语', 90),
    ('ko', '韩语', 100),
    ('th', '泰语', 110),
    ('vi', '越南语', 120),
    ('ind', '印尼语', 130),
    ('tr', '土耳其语', 140),
    ('bn', '孟加拉语', 150),
    ('pl', '波兰语', 160),
    ('it', '意大利语', 170)
ON CONFLICT (code) DO NOTHING;
