CREATE TABLE `asset_classes` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `description` varchar(20) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=latin1;

INSERT INTO `asset_classes` VALUES (1,'Cash'),(2,'Shares');

CREATE TABLE `asset_list` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `asset_class` int(11) NOT NULL,
  `description` varchar(40) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=latin1;

INSERT INTO `asset_list` VALUES (1,1,'Bank Savings'),(2,2,'Apple Inc.');

CREATE TABLE `asset_log` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `asset_id` int(11) NOT NULL,
  `epoch` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `deposit_value` decimal(13,2) NOT NULL,
  `asset_value` decimal(13,2) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

CREATE TABLE `settings` (
  `setting` varchar(20) NOT NULL,
  `value` varchar(20) NOT NULL,
  PRIMARY KEY (`setting`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

INSERT INTO `settings` VALUES ('db_version','1');
