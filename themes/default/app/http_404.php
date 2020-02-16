<?php

class Document extends Theme {

    protected $pageTitle = '404 Page not found';

    public function __construct(&$main, &$twig, $vars) {

        $vars['page_title'] = $this->pageTitle;

        $this->vars = $vars;

        header('HTTP/1.1 404 Not Found');
        $this->document = $twig->load('http_404.html');
    }

}
