<?php

interface ThemeInterface {

	public function __construct(&$main, &$twig, $vars);

	public function render();

}

