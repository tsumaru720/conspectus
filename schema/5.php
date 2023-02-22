<?php

$q = $db->query(<<<'EOT'
ALTER TABLE `settings` CHANGE `value` `value` VARCHAR(2048) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL;

UPDATE `settings` SET `value` = '5' WHERE `settings`.`setting` = 'db_version';
EOT
);