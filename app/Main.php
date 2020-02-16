<?php

class Main {

    // Control variables for when we dont want to simply render the page

    // initOnly will load, parse and test our config and create a DB object
    // but wont do any rendering
    public static $initOnly = false;

    // dbUpgrade signals that we are intentionally calling this with a mismatched
    // db version - we're probably doing this to do some DB modifications, but still
    // need to parse the config and test we can actually connect.
    public static $dbUpgrade = false;

    // Variable to control output method. Initially this will be used for when we
    // need to bail and generate an error - mostly with config/db loading.
    // Variable allows us to change this to "cli" for example when initiating DB upgrade
    // from command line
    //
    // Current supported values are "cli" and <anything else>
    public static $outputMethod = 'html';

    // The current expected database schema version.
    public static $expectedSchema = 1;

    // variable initialization
    private $config = null;
    private $db = null;
    private $pageLoader = null;
    private $router = null;
    private $entityManager = null;

    public function __construct() {
        spl_autoload_register(array($this,'classLoader'));

        if (Main::$dbUpgrade) {
            // initOnly is implied
            Main::$initOnly = true;
        }

        $this->pageLoader = new PageLoader($this);
        $this->loadConfig();
        $this->initDB(Main::$expectedSchema);

        // If we get this far, initial loading _seems_ ok
        // Check if we should go further...
        if (!Main::$initOnly) {
            $this->entityManager = new EntityManager($this);
            $this->router = new Router($this->pageLoader);
            $this->router->run();
        }
    }

    private function classLoader($class) {
        require __DIR__. '/'.$class.'.php';
    }
    
    private function loadConfig() {
        // Placeholder for default config settings
        $config = array();

        $this->config = $config;
        if (file_exists(__DIR__ . '/../config.php')) {
            require_once(__DIR__. '/../config.php');
            $this->config = array_merge($this->config, $config);
            //TODO check for required config entries
        } else {
            //TODO handle this nicer
            $this->fatalErr('NO_CFG', 'Unable to load config file');
        }

        if (array_key_exists('THEME', $this->config)) {
            $this->pageLoader->setTheme($this->config['THEME']);
        }
    }

    private function initDB($expectedSchema) {
        $this->db = new MySQL($this->config['SQL_HOSTNAME'],
                            $this->config['SQL_PORT'],
                            $this->config['SQL_USERNAME'],
                            $this->config['SQL_PASSWORD'],
                            $this->config['SQL_DATABASE']
                        );
        $db = $this->db;

        // Only do further checks if we're not upgrading the database
        if (!Main::$dbUpgrade) {
            if ($e = $db->getError()) {
                //Some kind of connection/access error occured
                $this->fatalErr('MYSQL_'.$e['code'], $e['message']);
            }

            $data = array(':database' => $this->config['SQL_DATABASE']);
            $q = $db->query("SELECT
                                 COUNT(TABLE_NAME) AS count
                             FROM
                                 information_schema.tables
                             WHERE
                                 table_schema = :database AND TABLE_NAME = 'settings'
                             LIMIT 1", $data);
            $r = $db->fetch($q);
            if ($r['count'] > 0) {
                $q = $db->query("SELECT value from settings WHERE setting = 'db_version'");
                $r = $db->fetch($q);
                if ($r['value'] != $expectedSchema) {
                    if (!Main::$dbUpgrade) {
                        $this->fatalErr('DB_SCHEMA', 'Database schema upgrade is required');
                    }
                }
            } else {
                $this->fatalErr('DB_404', 'Database appears incomplete. Please ensure first setup has been performed');
            }
        }
    }

    public function fatalErr($errorCode, $errorStr) {
        if (Main::$outputMethod == 'cli') {
            // This likely doesnt need to be themed so can just output our default error
            echo $errorStr;
            echo "\n";
        } else {
            $this->pageLoader->setFrame(false, false);
            $this->pageLoader->setVar('error_code', $errorCode);
            $this->pageLoader->setVar('error_string', $errorStr);
            $this->pageLoader->display('loading_error');
        }
        die();
    }
    
    public function getDB() {
        return $this->db;
    }

    public function getPageLoader() {
        return $this->pageLoader;
    }

    public function getConfig() {
        return $this->config;
    }

    public function getEntityManager() {
        return $this->entityManager;
    }
}
