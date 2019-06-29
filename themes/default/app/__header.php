<?php

class Header extends Theme {

	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$this->db = $main->getDB();

		$q = $this->db->query("SELECT * from asset_classes ORDER BY description ASC");
		while ($item = $this->db->fetch($q)) {
			$vars['class_menu_items'][] = $item;
		}

		$q = $this->db->query("SELECT * from asset_list ORDER BY description ASC");
		while ($item = $this->db->fetch($q)) {
			$vars['asset_menu_items'][] = $item;
		}

		$this->vars = $vars;
		$this->document = $twig->load('__header.html');
	}

}
