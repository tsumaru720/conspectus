<?php

class Header extends Theme {

	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$this->db = $main->getDB();
		$this->page = $main->getPage();

		$type = $this->page->resolveListType($vars['nav_item']);

		if ($type == "asset") {
			$q = $this->db->query("SELECT * from asset_list ORDER BY description ASC");
		} elseif ($type == "class") {
			$q = $this->db->query("SELECT * from asset_classes ORDER BY description ASC");
		}

		while ($item = $this->db->fetch($q)) {
			$vars['items'][] = $item;
		}

		$this->vars = $vars;
		$this->document = $twig->load('__header.html');
	}

}
