<?php

abstract class Theme implements ThemeInterface {

	protected $document = null;
	protected $vars = array();
	protected $pageTitle = null;
	protected $register = array('script' => array(), 'style' => array());

	public function __construct(&$main, &$twig, $vars) { }

	public function render() {
		$this->vars['register'] = $this->register;
		echo $this->document->render($this->vars);
	}
	
	public function getTitle() {
		return $this->pageTitle;
	}

	public function setRegister($type, $value) {
		if (is_array($value)) {
			$this->register[$type] = $value;
		} else {
			$this->register[$type][] = $value;
		}
	}

	public function getRegister($type) {
		return $this->register[$type];
	}

	public function dump($str) {
		echo "<!-- \n\n";
		var_dump($str);
		echo "\n\n-->";
	}
}
