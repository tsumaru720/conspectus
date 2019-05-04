<?php

require_once('config.php');
require_once('inc/mysql.php');

// Lets connect to MYSQL
$mysql = new MySQL($CONFIG['SQL_HOSTNAME'], $CONFIG['SQL_PORT'], $CONFIG['SQL_USERNAME'], $CONFIG['SQL_PASSWORD'], $CONFIG['SQL_DATABASE']);

echo "<p>List of assets</p>\n";

$q = $mysql->query("SELECT asset_list.description as asset, asset_classes.description as class FROM asset_list LEFT JOIN asset_classes ON asset_list.asset_class = asset_classes.id ORDER BY asset ASC");
while ($r = $mysql->fetch($q)) {
	echo $r['asset']."<br>\n";
}

?>


