<?php
require __DIR__ . '/../vendor/autoload.php';
require __DIR__ . '/../app/Main.php';

Main::$dbUpgrade = true;
Main::$outputMethod = 'cli';

//Set starting schema to 1 until we determine otherwise
$currentSchema = 1;

//---------

$path = __DIR__ . '/../schema';

$main = new Main();
$db = $main->getDB();

//Some kind of connection/access error occured
if ($e = $db->getError()) {
    // If the chosen database doesnt exist
    // PDO wont give us a handle, so we need to bail :(
    $main->fatalErr('MYSQL_'.$e['code'], $e['message']);
}

$q = $db->query("SHOW TABLES");
if ($q->rowCount() == 0) {
    //Empty database?
    echo "Database is empty\n";
    schemaUpdate($currentSchema);
}

$q = $db->query("SHOW TABLES LIKE 'settings'");
if ($q->rowCount() == 0) {
    // No settings table, broken install?
    echo "Settings table not found\n";
    schemaUpdate($currentSchema);
}

$currentSchema = getVersion();

while ($currentSchema < Main::$expectedSchema) {
    schemaUpdate($currentSchema + 1);
    $appliedSchema = getVersion();
    if ($appliedSchema == $currentSchema) {
        //Applied schema should be 1 version higher
        // than when we last checked. If not then
        // something went wrong and we should bail.
        echo "Something went wrong.\n";
        echo "Database version: ".getVersion()."\n";
    }
    $currentSchema = $appliedSchema;
}

echo "Nothing more to do.\n";

//---------

function schemaUpdate($newSchema) {
    global $path, $db;
    $success = false;

    echo "Apply Schema: ".$newSchema;
    echo "\n";

    $schema = $path."/".$newSchema.".php";
    if (file_exists($schema)) {
        $h = $db->getHandle();
        $h->beginTransaction();
        require $schema;
        $q->closeCursor();
        $h->commit();
        // Great, no fatal errors
        $success = true;
    } else {
        echo "Schema ".$newSchema." is missing\n";
    }

    if ($success) {
        bumpVersion($newSchema);
        echo "Complete\n";
    } else {
        //Bail
        die();
    }
}

function getVersion() {
    global $db;
    $q = $db->query("SELECT value AS version from settings WHERE setting = 'db_version'");
    $r = $db->fetch($q);
    return $r['version'];
}

function bumpVersion($newVersion) {
    global $db;
    $q = $db->query("UPDATE `settings` SET `value` = '".$newVersion."' WHERE `settings`.`setting` = 'db_version'");
}
