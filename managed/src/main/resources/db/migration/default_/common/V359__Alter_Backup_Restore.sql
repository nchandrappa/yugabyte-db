-- Copyright (c) YugaByte, Inc.

ALTER TABLE IF EXISTS backup ADD COLUMN IF NOT EXISTS hidden BOOLEAN DEFAULT false NOT NULL;
ALTER TABLE IF EXISTS restore ADD COLUMN IF NOT EXISTS hidden BOOLEAN  DEFAULT false NOT NULL;
