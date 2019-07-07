<?php

class Main {

	private $dbSchema = 1;

	private $config = null;
	private $db = null;
	private $pageLoader = null;
	private $router = null;

	public function __construct() {
		spl_autoload_register(array($this,'classLoader'));

		$this->pageLoader = new PageLoader($this);
		$this->loadConfig();
		$this->initDB($this->dbSchema);

		// If we get this far, initial loading _seems_ ok
		$this->router = new Router($this->pageLoader);
		$this->router->run();

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

	private function initDB($expectedVersion) {
		$this->db = new MySQL($this->config['SQL_HOSTNAME'],
							$this->config['SQL_PORT'],
							$this->config['SQL_USERNAME'],
							$this->config['SQL_PASSWORD'],
							$this->config['SQL_DATABASE']
						);
		$db = $this->db;
		if (!is_object($db->getHandle())) {
			$this->fatalErr('DB_403', 'Unable to connect to database server; Please check the credentials provided');
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
			if ($r['value'] != $expectedVersion) {
				$this->fatalErr('DB_SCHEMA', 'Database schema upgrade is required');
			}
		} else {
			$this->fatalErr('DB_404', 'Database appears incomplete. Please ensure first setup has been performed');
		}
	}

	public function fatalErr($errorCode, $errorStr) {
		$this->pageLoader->setFrame(false, false);
		$this->pageLoader->setVar('error_code', $errorCode);
		$this->pageLoader->setVar('error_string', $errorStr);
		$this->pageLoader->display('loading_error');
		die();
	}
	
	public function getDB() {
		return $this->db;
	}

	public function getPageLoader() {
		return $this->pageLoader;
	}
}
