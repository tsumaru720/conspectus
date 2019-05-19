<?php

class Main {

	private $theme = "default";

	public function __construct() {
		spl_autoload_register(array($this,'classLoader'));
	}

	private function classLoader($class) {
		require_once(__DIR__ . '/'.$class.'.php');
	}
	
	private function loadConfig() {
		// Some kind of config check
	}
	
	
}
?>



