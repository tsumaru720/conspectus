<?php

abstract class Theme implements ThemeInterface {

	protected $document = null;
	protected $vars = array();
	protected $pageTitle = null;

	public function __construct(&$main, &$twig, $vars) { }

	public function render() {
		echo $this->document->render($this->vars);
	}
	
	public function getTitle() {
		return $this->pageTitle;
	}

	public function dump($str) {
		echo "<!-- \n\n";
		var_dump($str);
		echo "\n\n-->";
	}
}
