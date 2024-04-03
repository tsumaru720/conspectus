<?php

class EntityManager {
    private $main = null;
    private $db = null;
    private $lastAssetObject = null;
    private $lastClassObject = null;
    private $lastPaymentObject = null;

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
        if ($this->lastAssetObject instanceof AssetEntity) {
            if ($this->lastAssetObject->getID() == $assetID) { return $this->lastAssetObject; }
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
            $this->lastAssetObject = new AssetEntity($item);
            return $this->lastAssetObject;
        } else {
            return false;
        }
    }

    public function getClass($classID) {
        if ($this->lastClassObject instanceof ClassEntity) {
            if ($this->lastClassObject->getID() == $classID) { return $this->lastClassObject; }
        }

        $data = array(':item_id' => $classID);

        $q = $this->db->query("SELECT
                                    asset_classes.id AS id,
                                    asset_classes.description AS description
                                FROM
                                    asset_classes
                                WHERE
                                    asset_classes.id = :item_id;", $data);

        if ($item = $this->db->fetch($q)) {
            $this->lastClassObject = new ClassEntity($item);
            return $this->lastClassObject;
        } else {
            return false;
        }
    }

    public function getPayment($paymentID) {
        if ($this->lastPaymentObject instanceof PaymentEntity) {
            if ($this->lastPaymentObject->getID() == $paymentID) { return $this->lastPaymentObject; }
        }

        $data = array(':payment_id' => $paymentID);

        $q = $this->db->query("SELECT
                                    payments.id,
                                    asset_list.id as asset_id,
                                    asset_list.description,
                                    epoch,
                                    amount
                                FROM
                                    payments
                                LEFT JOIN asset_list ON asset_id = asset_list.id
                                WHERE
                                    payments.id = :payment_id;", $data);

        if ($item = $this->db->fetch($q)) {
            $this->lastPaymentObject = new PaymentEntity($item);
            return $this->lastPaymentObject;
        } else {
            return false;
        }
    }
}
