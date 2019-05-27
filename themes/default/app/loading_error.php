<?php

class Document {
	public function __construct(&$main, $vars) {
		$loader = new \Twig\Loader\FilesystemLoader(__DIR__.'/../html');
		$twig = new \Twig\Environment($loader);

		$vars['page_title'] = 'Error';
		
		switch ($vars['error_code']) {
			case 'NO_CFG':
				$vars['string'] = 'Unable to load config file.';
				break;
			case 'DB_403':
				$vars['string'] = 'Unable to connect to database server; Please check the credentials provided';
				break;
		}
		
		echo $twig->render('loading_error.html', $vars);
	}
}

?>
