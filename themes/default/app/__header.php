<?php

class Header extends Theme {

    private $db = null;

    public function __construct(&$main, &$twig, $vars) {

        $this->db = $main->getDB();

        $q = $this->db->query("SELECT * from asset_classes ORDER BY description ASC");
        while ($item = $this->db->fetch($q)) {
            $vars['class_menu_items'][] = $item;
        }

        // TODO: Integrate this with EntintyManager?
        $q = $this->db->query("SELECT
                                asset_list.id as id,
                                asset_list.description as description,
                                asset_classes.description AS class,
                                asset_classes.id AS class_id
                            FROM
                                asset_list
                            LEFT JOIN asset_classes ON asset_class = asset_classes.id ORDER BY description ASC");
        while ($item = $this->db->fetch($q)) {
            $vars['asset_menu_items'][] = $item;
        }

        $this->vars = $vars;
        $this->document = $twig->load('__header.html');
    }

}
