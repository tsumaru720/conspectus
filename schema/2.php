<?php

$q = $db->query(<<<'EOT'
ALTER TABLE `asset_list` ADD `closed` BOOLEAN NOT NULL DEFAULT FALSE AFTER `description`;

UPDATE `settings` SET `value` = '2' WHERE `settings`.`setting` = 'db_version';
EOT
);

