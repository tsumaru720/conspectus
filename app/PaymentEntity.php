<?php

class PaymentEntity extends EntityObject {
    private $asset_id = null;
    private $epoch = null;
    private $amount = null;

    public function __construct(&$data) {
        $this->id = $data['id'];
        $this->description = $data['description'];
        $this->asset_id = $data['asset_id'];
        $this->epoch = $data['epoch'];
        $this->amount = $data['amount'];
    }

    public function getAssetID() {
        return $this->asset_id;
    }

    public function getEpoch() {
        return $this->epoch;
    }

    public function getAmount() {
        return $this->amount;
    }
}