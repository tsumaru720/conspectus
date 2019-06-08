<?php

class Main {

	private $config = null;
	private $db = null;
	private $page = null;
	private $router = null;

	public function __construct() {
		spl_autoload_register(array($this,'classLoader'));

		session_start();
		$this->page = new PageLoader($this);
		$this->loadConfig();
		$this->initDB();

		// If we get this far, initial loading _seems_ ok
		$this->router = new Router($this->page);
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
			$this->page->setTheme($this->config['THEME']);
		}
		if (array_key_exists('VIEW', $this->config) && !array_key_exists('view', $_SESSION)) {
			// This should only set the default view if we dont already have one
			// User might have changed it in the UI
			$this->page->setView($this->config['VIEW']);
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
			$this->fatalErr('DB_403', 'Unable to connect to database server; Please check the credentials provided');
		}

	}

	public function fatalErr($errorCode, $errorStr) {
		$this->page->setFrame(false, false);
		$this->page->setVar('error_code', $errorCode);
		$this->page->setVar('error_string', $errorStr);
		$this->page->display('loading_error');
		die();
	}
	
	public function getDB() {
		return $this->db;
	}

	public function getPage() {
		return $this->page;
	}
}
