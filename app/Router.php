<?php

class Router {

    private $router = null;
    private $page = null;
    private $entityManager = null;

    public function __construct(&$main) {
        $this->page = $main->getPageLoader();
        $this->entityManager = $main->getEntityManager();

        $this->router = new \Bramus\Router\Router();
        $this->setGlobals();
        $this->addRoutes();
        Theme::customRoutes($this->router, $this->page);
    }
    
    private function checkExists($type, $itemID) {
        if ($type == "asset") {
            return $this->entityManager->getAsset($itemID);
        } elseif ($type == "class") {
            return $this->entityManager->getClass($itemID);
        } else {
            return false;
        }
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

        $this->router->get('/view/{type}/(\d+)', function($type, $itemID) {
            if (!$this->checkExists($type, $itemID)) {
                $this->page->setFrame(false, false);
                $this->page->display('http_404');
            } else {
                $this->page->setVar('left_menu', $type.'/'.$itemID);
                $this->page->setVar('type', $type);
                $this->page->setVar('item_id', $itemID);
                $this->page->display('item_view');
            }
        });

        $this->router->get('/breakdown', function() {
            $this->page->setVar('nav_item', 'breakdown');
            $this->page->display('breakdown');
        });

        $this->router->get('/breakdown/{type}/(\d+)', function($type, $itemID) {
            if (!$this->checkExists($type, $itemID)) {
                $this->page->setFrame(false, false);
                $this->page->display('http_404');
            } else {
                $this->page->setVar('left_menu', $type.'/'.$itemID);
                $this->page->setVar('nav_item', 'breakdown');
                $this->page->setVar('type', $type);
                $this->page->setVar('item_id', $itemID);
                $this->page->display('breakdown');
            }
        });

        $this->router->get('/analytics', function() {
            $this->page->setVar('nav_item', 'analytics');
            $this->page->display('analytics');
        });

        $this->router->get('/analytics/{type}/(\d+)', function($type, $itemID) {
            if (!$this->checkExists($type, $itemID)) {
                $this->page->setFrame(false, false);
                $this->page->display('http_404');
            } else {
                $this->page->setVar('left_menu', $type.'/'.$itemID);
                $this->page->setVar('nav_item', 'analytics');
                $this->page->setVar('type', $type);
                $this->page->setVar('item_id', $itemID);
                $this->page->display('analytics');
            }
        });

        $this->router->get('/projections', function() {
            $this->page->setVar('nav_item', 'projections');
            $this->page->display('projections');
        });

        $this->router->get('/projections/{type}/(\d+)', function($type, $itemID) {
            if (!$this->checkExists($type, $itemID)) {
                $this->page->setFrame(false, false);
                $this->page->display('http_404');
            } else {
                $this->page->setVar('left_menu', $type.'/'.$itemID);
                $this->page->setVar('nav_item', 'projections');
                $this->page->setVar('type', $type);
                $this->page->setVar('item_id', $itemID);
                $this->page->display('projections');
            }
        });

        $this->router->get('/ledger', function() {
            $this->page->setVar('nav_item', 'ledger');
            $this->page->display('ledger');
        });

        $this->router->get('/ledger/{type}/(\d+)', function($type, $itemID) {
            if (!$this->checkExists($type, $itemID)) {
                $this->page->setFrame(false, false);
                $this->page->display('http_404');
            } else {
                $this->page->setVar('left_menu', $type.'/'.$itemID);
                $this->page->setVar('nav_item', 'ledger');
                $this->page->setVar('type', $type);
                $this->page->setVar('item_id', $itemID);
                $this->page->display('ledger');
            }
        });

        $this->router->match('GET|POST', '/asset/new', function() {
            $this->page->setVar('action', 'new');
            $this->page->display('asset_manager');
        });

        $this->router->match('GET|POST', '/asset/edit/(\d+)', function($itemID) {
            if (!$this->checkExists('asset', $itemID)) {
                $this->page->setFrame(false, false);
                $this->page->display('http_404');
            } else {
                $this->page->setVar('left_menu', 'asset/'.$itemID);
                $this->page->setVar('nav_item', 'view');
                $this->page->setVar('item_id', $itemID);
                $this->page->setVar('action', 'edit');
                $this->page->display('asset_manager');
            }
        });

        $this->router->match('GET|POST', '/class/new', function() {
            $this->page->setVar('action', 'new');
            $this->page->display('class_manager');
        });

        $this->router->match('GET|POST', '/class/edit/(\d+)', function($itemID) {
            if (!$this->checkExists('class', $itemID)) {
                $this->page->setFrame(false, false);
                $this->page->display('http_404');
            } else {
                $this->page->setVar('left_menu', 'class/'.$itemID);
                $this->page->setVar('nav_item', 'view');
                $this->page->setVar('item_id', $itemID);
                $this->page->setVar('action', 'edit');
                $this->page->display('class_manager');
            }
        });

        $this->router->set404(function() {
            $this->page->setFrame(false, false);
            $this->page->display('http_404');
        });
    }

}
