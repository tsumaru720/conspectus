<?php

class EntityManager {
    private $main = null;
    private $db = null;
    private $lastAsset = null;
    private $lastClass = null;

    public function __construct(&$main) {
        $this->main = $main;
        $this->db = $main->getDB();
    }
/*
    private function checkInt($int) {
        if (!is_numeric($int) || $int < 0) {
            return false;
        } else {
            return true;
        }
    }
*/
    public function getAsset($assetID) {
        if ($this->lastAsset instanceof EntityObject) {
            if ($this->lastAsset->getID() == $assetID) { return $this->lastAsset; }
        }
/*
        if (!$this->checkInt($assetID)) {
            return false;
        }
*/
        $data = array(':item_id' => $assetID);

        $q = $this->db->query("SELECT
                                    asset_list.id as id,
                                    asset_list.description as description,
                                    asset_classes.description AS class,
                                    asset_classes.id AS class_id,
                                    asset_list.closed as closed
                                FROM
                                    asset_list
                                LEFT JOIN asset_classes ON asset_class = asset_classes.id
                                WHERE
                                    asset_list.id = :item_id;", $data);

        if ($item = $this->db->fetch($q)) {
            $this->lastAsset = new AssetEntity($item);
            return $this->lastAsset;
        } else {
            return false;
        }
    }

    public function getClass($classID) {
        if ($this->lastClass instanceof EntityObject) {
            if ($this->lastClass->getID() == $classID) { return $this->lastClass; }
        }
/*
        if (!$this->checkInt($classID)) {
            return false;
        }
*/
        $data = array(':item_id' => $classID);

        $q = $this->db->query("SELECT
                                    asset_classes.id AS id,
                                    asset_classes.description AS description
                                FROM
                                    asset_classes
                                WHERE
                                    asset_classes.id = :item_id;", $data);

        if ($item = $this->db->fetch($q)) {
            $this->lastClass = new ClassEntity($item);
            return $this->lastClass;
        } else {
            return false;
        }
    }
}


        