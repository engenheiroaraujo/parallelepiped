-- Keep the link text as written, so a missing note can be shown and created
-- with its original casing instead of the lower-case resolution key.
ALTER TABLE links ADD COLUMN target TEXT NOT NULL DEFAULT '';
