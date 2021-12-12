<?php

$q = $db->query(<<<'EOT'
DROP TABLE IF EXISTS `payments`;

CREATE TABLE `payments` (
  `id` INT(11) NOT NULL AUTO_INCREMENT,
  `asset_id` INT(11) NOT NULL,
  `epoch` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `amount` DECIMAL(13,2) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=latin1;

UPDATE `settings` SET `value` = '4' WHERE `settings`.`setting` = 'db_version';
EOT
);