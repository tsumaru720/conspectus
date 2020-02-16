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
                                    asset_classes.description AS class
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

    }



}


        