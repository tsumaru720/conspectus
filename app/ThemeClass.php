<?php

interface ThemeClass {

	public function __construct(&$main, &$twig, $vars);

	public function render();

}

