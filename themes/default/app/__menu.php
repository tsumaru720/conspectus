<?php

class Menu extends Theme {

	public function __construct(&$main, &$twig, $vars) {

		$this->vars = $vars;
		$this->document = $twig->load('__menu.html');
	}

}

?>
