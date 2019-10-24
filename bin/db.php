<?php
require __DIR__ . '/../vendor/autoload.php';
require __DIR__ . '/../app/Main.php';

Main::$dbUpgrade = true;
Main::$outputMethod = 'cli';

$main = new Main();

$db = $main->getDB();

$q = $db->query("SELECT value from settings WHERE setting = 'db_version'");
$r = $db->fetch($q);

var_dump($r);