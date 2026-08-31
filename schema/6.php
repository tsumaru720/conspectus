<?php
$q = $db->query(<<<'EOT'
CREATE INDEX `idx_asset_log_asset_epoch` ON `asset_log` (`asset_id`, `epoch`);

CREATE INDEX `idx_payments_asset_epoch` ON `payments` (`asset_id`, `epoch`);

ALTER TABLE `settings`
  ADD `description` VARCHAR(120) NOT NULL DEFAULT '' AFTER `value`,
  ADD `display` BOOLEAN NOT NULL DEFAULT FALSE AFTER `description`;

UPDATE `settings` SET `value` = '6' WHERE `settings`.`setting` = 'db_version';
EOT
);
