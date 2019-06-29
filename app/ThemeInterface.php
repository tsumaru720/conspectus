<?php

interface ThemeInterface {

	public function __construct(&$main, &$twig, $vars);

	public function render();

	public function getTitle();

	public function setRegister($type, $value);

	public function getRegister($type);

	public static function customRoutes(&$router, &$page);

}

