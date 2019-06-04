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
		$this->router->get('/', function() {
			$this->page->setVar('modifier', '>');
			$this->page->setVar('asset_id', '0');
			$this->page->display('asset_view');
		});

		$this->router->get('/asset/{assetID}', function($assetID) {
			$this->page->setVar('modifier', '=');
			$this->page->setVar('asset_id', $assetID);
			$this->page->display('asset_view');
		});

		$this->router->set404(function() {
			$this->page->setFrame(false, false);
			$this->page->display('http_404');
		});
	}

}
