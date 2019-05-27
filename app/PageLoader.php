<?php

class PageLoader {

	private $theme = 'default';
	private $main = null;
	private $twig = null;
	private $vars = array();
	
	private $displayHeader = true;
	private $displayMenu = true;
	private $displayFooter = true;

	public function __construct(&$main) {
		$this->main = $main;
	}

	public function setTheme($theme) {
		$this->$theme = $theme;
	}

	public function setVar($name, $value) {
		$this->vars[$name] = $value;
	}

	public function setFrame($header, $footer, $menu) {
		$this->displayHeader = $header;
		$this->displayFooter = $footer;
		$this->displayMenu = $menu;
	}

	private function resolveTheme($pageName) {
		if (file_exists(__DIR__ . '/../themes/'.$this->theme.'/'.$pageName)) {
			return $this->theme.'/'.$pageName;
		} else {
			return 'default/'.$pageName;
		}
	}

	public function load($pageName) {
		$loader = new \Twig\Loader\FilesystemLoader(__DIR__.'/../themes/default/html');

		// Loadin the main theme class/interface
		include __DIR__ . '/../themes/'.$this->resolveTheme('theme.php');

		// Load in the page we're trying to load
		if (file_exists(__DIR__ . '/../themes/'.$this->theme.'/html')) {
			$loader->prependPath(__DIR__ . '/../themes/'.$this->theme.'/html');
		}
		include __DIR__ . '/../themes/'.$this->resolveTheme('app/'.$pageName.'.php');

		$this->twig = new \Twig\Environment($loader);

		$doc = new Document($this->main, $this->twig, $this->vars);
		if (!$doc instanceof ThemeClass) {
			//TODO theme this - but by this point we already have "Document" declared...
			echo 'Template page does not use ThemeClass interface';
			die();
		}

		$doc->render();
	}

}

?>
