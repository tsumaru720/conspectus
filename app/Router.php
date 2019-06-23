<?php

class Router {

	private $router = null;
	private $page = null;

	public function __construct(&$page) {
		$this->page = $page;
		$this->router = new \Bramus\Router\Router();
		$this->setGlobals();
		$this->addRoutes();
	}
	
	public function run() {
		$this->router->run();
	}

	private function setGlobals() {
		$this->page->setVar('nav_item', 'overview');
		$this->page->setVar('type', 'asset');
	}

	private function addRoutes() {
		$this->router->get('/', function() {
			$this->page->setVar('left_menu', 'overview');
			$this->page->setVar('modifier', '>');
			$this->page->setVar('item_id', '0');
			$this->page->display('item_view');
		});

		$this->router->get('/view/{type}/{itemID}', function($type, $itemID) {
			$this->page->setVar('left_menu', $type.'/'.$itemID);
			$this->page->setVar('modifier', '=');
			$this->page->setVar('item_id', $itemID);
			$this->page->setVar('type', $type);
			$this->page->display('item_view');
		});

		$this->router->get('/breakdown', function() {
			$this->page->setVar('nav_item', 'breakdown');
			$this->page->display('breakdown');
		});

		$this->router->set404(function() {
			$this->page->setFrame(false, false);
			$this->page->display('http_404');
		});
	}

}
