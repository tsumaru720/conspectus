<?php

class PageLoader {

	private $theme = 'default';
	private $main = null;
	private $twig = null;
	private $vars = array();
	
	private $displayHeader = true;
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

	public function setFrame($header, $footer) {
		$this->displayHeader = $header;
		$this->displayFooter = $footer;
	}

	private function resolveTheme($location) {
		if (file_exists(__DIR__ . '/../themes/'.$this->theme.'/'.$location)) {
			return __DIR__ . '/../themes/'.$this->theme.'/'.$location;
		} else {
			return __DIR__ . '/../themes/default/'.$location;
		}
	}

	private function checkInterface($var) {
		if (!$var instanceof ThemeInterface) {
			//TODO theme this - but by this point we already have "Document" declared...
			echo 'Template page does not use ThemeInterface interface';
			die();
		}
	}

	public function load($pageName) {
		$loader = new \Twig\Loader\FilesystemLoader(__DIR__.'/../themes/default/html');
		$loader->prependPath($this->resolveTheme('html'));
		$this->twig = new \Twig\Environment($loader);

		// Loadin the main theme class/interface
		include $this->resolveTheme('theme.php');
		include $this->resolveTheme('app/'.$pageName.'.php');

		$doc = new Document($this->main, $this->twig, $this->vars);
		$this->checkInterface($doc);

		$this->setVar('page_title', $doc->getTitle());

		if ($this->displayHeader) {
			include $this->resolveTheme('app/__header.php');
			$header = new Header($this->main, $this->twig, $this->vars);
			$this->checkInterface($header);
			$header->render();
		}

		$doc->render();

		if ($this->displayFooter) {
			include $this->resolveTheme('app/__footer.php');
			$footer = new Footer($this->main, $this->twig, $this->vars);
			$this->checkInterface($footer);
			$footer->render();
		}
	}

}
