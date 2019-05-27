<?php

class Document extends Theme {

	protected $pageTitle = 'Error';

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;
		
		switch ($vars['error_code']) {
			case 'NO_CFG':
				$vars['string'] = 'Unable to load config file';
				break;
			case 'DB_403':
				$vars['string'] = 'Unable to connect to database server; Please check the credentials provided';
				break;
		}

		$this->vars = $vars;
		$this->document = $twig->load('loading_error.html');
	}

}

?>
