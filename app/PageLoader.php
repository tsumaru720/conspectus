<?php

class PageLoader {

	private $theme = 'default';
	private $main = null;
	private $doc = null;
	private $vars = array();
	
	public $displayHeader = true;
	public $displayMenu = true;
	public $displayFooter = true;

	public function __construct(&$main) {
		$this->main = $main;
	}

	public function setTheme($theme) {
		$this->$theme = $theme;
	}
	
	public function setVar($name, $value) {
		$this->vars[$name] = $value;
	}

	public function load($pageName) {
		if (file_exists(__DIR__ . '/../themes/'.$this->theme.'/app/'.$pageName.'.php')) {
			include __DIR__ . '/../themes/'.$this->theme.'/app/'.$pageName.'.php';
		} else {
			include __DIR__ . '/../themes/default/app/'.$pageName.'.php';
		}
		//TODO load twig here with fallback to "default" if no template file is found
		$this->doc = new Document($this->main, $this->vars);
	}

}

?>
