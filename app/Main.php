<?php

class Main {

	private $config = null;
	private $db = null;

	public function __construct() {
		spl_autoload_register(array($this,'classLoader'));

		$this->loadConfig();
		$this->initDB();
	}

	private function classLoader($class) {
		require __DIR__. '/'.$class.'.php';
	}
	
	private function loadConfig() {
		// Load defaults here.
		// If for any reason we cant load our config file, we'll need to know these to display an error.
		$config['THEME'] = 'default';

		$this->config = $config;
		if (file_exists(__DIR__ . '/../config.php')) {
			require_once(__DIR__. '/../config.php');
			$this->config = array_merge($this->config, $config);
			//TODO check for required config entries
		} else {
			//TODO handle this nicer
			$this->fatalErr('no config');
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
			$this->fatalErr('invalid db creds');
		}

	}

	private function fatalErr($string) {
		//TODO make this theme stuff
		//TODO some kind of error codes so theme can do wording
		var_dump($this->config);
		echo $string;
		die();
	}
	
	public function getDB() {
		return $this->db;
	}
	
}

?>
