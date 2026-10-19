-- 222: mode "Yang Proper" per media (rilis resmi, strict grounding).
-- Isian tab Yang Proper di modal Pengelola Naskah disimpan supaya
-- regenerasi dan Gatekeeper memakai sumber fakta yang sama dengan Writer.
ALTER TABLE taut_campaign_media ADD COLUMN IF NOT EXISTS proper_fakta text;
ALTER TABLE taut_campaign_media ADD COLUMN IF NOT EXISTS proper_dateline text;
ALTER TABLE taut_campaign_media ADD COLUMN IF NOT EXISTS proper_narasumber text;
ALTER TABLE taut_campaign_media ADD COLUMN IF NOT EXISTS proper_kutipan text;
ALTER TABLE taut_campaign_media ADD COLUMN IF NOT EXISTS proper_fokus text;
ALTER TABLE taut_campaign_media ADD COLUMN IF NOT EXISTS proper_brief_id text;
