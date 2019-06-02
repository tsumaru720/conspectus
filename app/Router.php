<?php

class Router {

	private $router = null;
	private $page = null;

	public function __construct(&$page) {
		$this->page = $page;
		$this->router = new \Bramus\Router\Router();
		$this->addRoutes();
	}
	
	public function run() {
		$this->router->run();
	}

	private function addRoutes() {
		$this->router->get('/', function() { $this->page->display('dashboard'); });

		$this->router->set404(function() {
			$this->page->setFrame(false, false);
			$this->page->display('http_404');
		});
	}

}
