<?php

class EntityManager {
    private $main = null;
    private $db = null;

    public function __construct(&$main) {
        $this->main = $main;
        $this->db = $main->getDB();
    }

    public function getAsset($assetID) {
        $data = array(':item_id' => $assetID);

        $q = $this->db->query("SELECT
                                    asset_list.id as id,
                                    asset_list.description as description,
                                    asset_classes.description AS class,
                                    asset_classes.id AS class_id
                                FROM
                                    asset_list
                                LEFT JOIN asset_classes ON asset_class = asset_classes.id
                                WHERE
                                    asset_list.id = :item_id;", $data);

        if ($item = $this->db->fetch($q)) {
            return new AssetEntity($item);
        } else {
            return false;
        }
    }

    public function getClass($classID) {
        $data = array(':item_id' => $classID);

        $q = $this->db->query("SELECT
                                    asset_classes.id AS id,
                                    asset_classes.description AS description
                                FROM
                                    asset_classes
                                WHERE
                                    asset_classes.id = :item_id;", $data);

        if ($item = $this->db->fetch($q)) {
            return new ClassEntity($item);
        } else {
            return false;
        }
    }



}


        