<?php

class Document extends Theme {

	public function __construct(&$main, &$twig, $vars) {

		$this->vars = $vars;
		$page = $main->getPage();

		$page->setListType($page->resolveListType($vars['nav_item']));

		header("Location: /");
		die();

	}

}
