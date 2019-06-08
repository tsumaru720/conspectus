<?php

class Document extends Theme {

	public function __construct(&$main, &$twig, $vars) {

		$this->vars = $vars;
		$page = $main->getPage();

		$page->setView($page->resolveView($vars['nav_item']));

		header("Location: /");
		die();

	}

}
