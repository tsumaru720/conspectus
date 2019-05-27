<?php

class Document extends Theme {

	protected $pageTitle = 'Error';

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->vars = $vars;
		$this->document = $twig->load('loading_error.html');
	}

}

?>
