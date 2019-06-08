<?php

class Header extends Theme {

	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$this->db = $main->getDB();

		if ($vars['nav_item'] == "asset") {
			$vars['view_string'] = "Assets";
			$q = $this->db->query("SELECT * from asset_list ORDER BY description ASC");
		} elseif ($vars['nav_item'] == "class") {
			$vars['view_string'] = "Classes";
			$q = $this->db->query("SELECT * from asset_classes ORDER BY description ASC");
		}

		while ($asset = $this->db->fetch($q)) {
			$vars['assets'][] = $asset;
		}

		$this->vars = $vars;
		$this->document = $twig->load('__header.html');
	}

}
