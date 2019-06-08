<?php

class Document extends Theme {

	public function __construct(&$main, &$twig, $vars) {

		$this->vars = $vars;
		$page = $main->getPage();

		if ($vars['nav_item'] == 'asset') {
			$page->setView('asset');
		} elseif ($vars['nav_item'] == 'class') {
			$page->setView('class');
		} else {
			$page->setView('asset');
		}
		
		header("Location: /");
		die();
		//header("Location: http://www.redirect.to.url.com/")
		//header('HTTP/1.1 404 Not Found');
	}

}
