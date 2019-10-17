<?php

class Router {

	private $router = null;
	private $page = null;

	public function __construct(&$page) {
		$this->page = $page;
		$this->router = new \Bramus\Router\Router();
		$this->setGlobals();
		$this->addRoutes();
		Theme::customRoutes($this->router, $this->page);
	}
	
	public function run() {
		$this->router->run();
	}

	private function setGlobals() {
		$this->page->setVar('left_menu', 'all');
		$this->page->setVar('nav_item', 'view');
		$this->page->setVar('type', 'asset');
	}

	private function addRoutes() {
		$this->router->get('/', function() {
			$this->page->display('item_view');
		});

		$this->router->get('/view', function() {
			$this->page->display('item_view');
		});

		$this->router->get('/view/{type}/{itemID}', function($type, $itemID) {
			$this->page->setVar('left_menu', $type.'/'.$itemID);
			$this->page->setVar('type', $type);
			$this->page->setVar('item_id', $itemID);
			$this->page->display('item_view');
		});

		$this->router->get('/breakdown', function() {
			$this->page->setVar('nav_item', 'breakdown');
			$this->page->display('breakdown');
		});

		$this->router->get('/breakdown/{type}/{itemID}', function($type, $itemID) {
			$this->page->setVar('left_menu', $type.'/'.$itemID);
			$this->page->setVar('nav_item', 'breakdown');
			$this->page->setVar('type', $type);
			$this->page->setVar('item_id', $itemID);
			$this->page->display('breakdown');
		});

		$this->router->get('/analytics', function() {
			$this->page->setVar('nav_item', 'analytics');
			$this->page->display('analytics');
		});

		$this->router->get('/analytics/{type}/{itemID}', function($type, $itemID) {
			$this->page->setVar('left_menu', $type.'/'.$itemID);
			$this->page->setVar('nav_item', 'analytics');
			$this->page->setVar('type', $type);
			$this->page->setVar('item_id', $itemID);
			$this->page->display('analytics');
		});

		$this->router->get('/projections', function() {
			$this->page->setVar('nav_item', 'projections');
			$this->page->display('projections');
		});

		$this->router->get('/projections/{type}/{itemID}', function($type, $itemID) {
			$this->page->setVar('left_menu', $type.'/'.$itemID);
			$this->page->setVar('nav_item', 'projections');
			$this->page->setVar('type', $type);
			$this->page->setVar('item_id', $itemID);
			$this->page->display('projections');
		});

		$this->router->match('GET|POST', '/asset/new', function() {
			$this->page->setVar('action', 'new');
			$this->page->display('asset_manager');
		});

		$this->router->set404(function() {
			$this->page->setFrame(false, false);
			$this->page->display('http_404');
		});
	}

}
