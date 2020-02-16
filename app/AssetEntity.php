<?php

class AssetEntity extends EntityObject {
    private $class = null;

    public function __construct(&$data) {
        $this->id = $data['id'];
        $this->description = $data['description'];
        $this->class = $data['class'];
    }

    public function getClass() {
        return $this->class;
    }
}