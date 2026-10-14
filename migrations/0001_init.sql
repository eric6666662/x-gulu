-- 阶段 1 首版 schema（02-后端设计 §2）。
-- 迁移文件一旦发布即不可修改，只能追加新文件；本文件为 v0 首次发布。

CREATE TABLE services (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, preset TEXT,
  base_url TEXT NOT NULL, api_key TEXT,           -- 明文存本地 0600 文件；仅本机、无登录
  models_json TEXT NOT NULL DEFAULT '[]',         -- [{"id","name"}]
  is_default INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

CREATE TABLE roles (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, system_prompt TEXT NOT NULL,
  params_json TEXT NOT NULL DEFAULT '{}',         -- {"temperature":0.3,...}
  is_builtin INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

CREATE TABLE conversations (
  id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '',
  service_id TEXT REFERENCES services(id) ON DELETE SET NULL,
  model_id TEXT, role_id TEXT REFERENCES roles(id) ON DELETE SET NULL,
  last_seq INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);

CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  seq INTEGER NOT NULL, role TEXT NOT NULL CHECK(role IN ('user','assistant')),
  content TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK(status IN ('complete','streaming','interrupted','error')),
  error_code TEXT, pinned INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  UNIQUE(conversation_id, seq)
);

CREATE INDEX idx_messages_conv ON messages(conversation_id, seq);
CREATE INDEX idx_conversations_updated ON conversations(updated_at DESC);

CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
-- 键：global_prompt, context_limit
INSERT INTO settings(key, value) VALUES ('global_prompt', ''), ('context_limit', '20');
