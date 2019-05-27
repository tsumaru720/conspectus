<?php

class Header extends Theme {

	public function __construct(&$main, &$twig, $vars) {

		$this->vars = $vars;
		$this->document = $twig->load('__header.html');
	}

}

?>
