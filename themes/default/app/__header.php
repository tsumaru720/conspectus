<?php

class Header extends Theme {

	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$this->db = $main->getDB();
		$q = $this->db->query("SELECT * from asset_list ORDER BY description ASC");
		while ($asset = $this->db->fetch($q)) {
			$vars['assets'][] = $asset;
		}

		$this->vars = $vars;
		$this->document = $twig->load('__header.html');
	}

}
