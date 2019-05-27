<?php

class Theme implements ThemeClass {

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
}
