<?php

class Main {

	private $config = null;
	private $db = null;
	private $page = null;

	public function __construct() {
		spl_autoload_register(array($this,'classLoader'));

		$this->page = new PageLoader($this);
		$this->loadConfig();
		$this->initDB();
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
			$this->fatalErr('NO_CFG');
		}

		if (array_key_exists('THEME', $this->config)) {
			$this->page->setTheme($this->config['THEME']);
		}
	}

	private function initDB() {
		$this->db = new MySQL($this->config['SQL_HOSTNAME'],
							$this->config['SQL_PORT'],
							$this->config['SQL_USERNAME'],
							$this->config['SQL_PASSWORD'],
							$this->config['SQL_DATABASE']
						);
		$db = $this->db;
		if (!is_object($db->getHandle())) {
			$this->fatalErr('DB_403');
		}

	}

	private function fatalErr($error) {
		$this->page->displayHeader = false;
		$this->page->displayMenu = false;
		$this->page->displayFooter = false;
		$this->page->setVar('error_code', $error);
		$this->page->load('loading_error');
		die();
	}
	
	public function getDB() {
		return $this->db;
	}
	
}

?>
