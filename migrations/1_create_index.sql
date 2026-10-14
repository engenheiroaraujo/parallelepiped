CREATE TABLE notes (
    path        TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    folder      TEXT NOT NULL,
    -- Lower-case title, used to resolve [[Title]] links.
    title_key   TEXT NOT NULL,
    -- Lower-case path without extension, used to resolve [[folder/Title]].
    path_key    TEXT NOT NULL,
    modified_at INTEGER NOT NULL
);
CREATE INDEX notes_title_key ON notes (title_key);
CREATE INDEX notes_path_key ON notes (path_key);

CREATE TABLE links (
    source_path TEXT NOT NULL REFERENCES notes (path) ON DELETE CASCADE,
    target_key  TEXT NOT NULL,
    alias       TEXT NOT NULL,
    embed       INTEGER NOT NULL
);
CREATE INDEX links_source ON links (source_path);
CREATE INDEX links_target ON links (target_key);

CREATE TABLE tags (
    path TEXT NOT NULL REFERENCES notes (path) ON DELETE CASCADE,
    tag  TEXT NOT NULL,
    PRIMARY KEY (path, tag)
);
CREATE INDEX tags_tag ON tags (tag);

CREATE VIRTUAL TABLE notes_fts USING fts5 (
    path UNINDEXED,
    title,
    content,
    tokenize = 'unicode61 remove_diacritics 2'
);
