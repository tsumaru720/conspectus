<?php

class Document extends Theme {

	protected $pageTitle = 'Dashboard';

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->vars = $vars;
		$this->document = $twig->load('dashboard.html');
	}

}
