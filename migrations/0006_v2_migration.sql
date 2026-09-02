CREATE INDEX idx_asset_log_asset_epoch ON asset_log (asset_id, epoch);

CREATE INDEX idx_payments_asset_epoch ON payments (asset_id, epoch);

ALTER TABLE settings
  ADD COLUMN description VARCHAR(120) NOT NULL DEFAULT '',
  ADD COLUMN display TINYINT(1) NOT NULL DEFAULT 0;
